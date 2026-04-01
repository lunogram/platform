package store

import "fmt"

// QueryArgs is a builder for PostgreSQL positional parameters ($1, $2, ...).
// It tracks the argument index automatically so callers never need to manually
// manage placeholder numbering.
//
// Usage:
//
//	qa := NewQueryArgs()
//	qa.Add(projectID)                             // $1
//	cond := qa.Clause("source = %s AND id = %s",
//	    qa.Add(src), qa.Add(id))                  // "source = $2 AND id = $3"
//	query := fmt.Sprintf("SELECT * FROM t WHERE project_id = $1 AND (%s)", cond)
//	db.SelectContext(ctx, &dst, query, qa.Args()...)
type QueryArgs struct {
	args []any
}

// NewQueryArgs creates a new QueryArgs builder with no arguments.
func NewQueryArgs() *QueryArgs {
	return &QueryArgs{}
}

// Add appends a value to the argument list and returns its PostgreSQL
// positional placeholder (e.g. "$1", "$2").
func (q *QueryArgs) Add(value any) string {
	q.args = append(q.args, value)
	return fmt.Sprintf("$%d", len(q.args))
}

// Clause builds a SQL fragment by formatting placeholders into the template.
// The placeholders should be the return values of Add calls.
//
// Example:
//
//	qa.Clause("(source = %s AND external_id = %s)", qa.Add(src), qa.Add(eid))
//	// => "(source = $2 AND external_id = $3)"
func (q *QueryArgs) Clause(format string, placeholders ...any) string {
	return fmt.Sprintf(format, placeholders...)
}

// Args returns the accumulated argument slice, ready to be spread into a
// database query call.
func (q *QueryArgs) Args() []any {
	return q.args
}

// Len returns the number of arguments added so far.
func (q *QueryArgs) Len() int {
	return len(q.args)
}
