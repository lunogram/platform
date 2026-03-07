package query

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rules"
)

// QueryBuilder builds PostgreSQL queries from rule definitions
type QueryBuilder struct {
	projectID      uuid.UUID
	userID         *uuid.UUID // Optional: filter for specific user
	sinceTimestamp *time.Time // Optional: timestamp for "since_entered" period type
	args           []any
	joins          []string
	joinCounter    int
}

// QueryResult contains the generated SQL and arguments
type QueryResult struct {
	SQL  string
	Args []any
}

// NewQueryBuilder creates a new QueryBuilder
func NewQueryBuilder(projectID uuid.UUID, userID *uuid.UUID) *QueryBuilder {
	return &QueryBuilder{
		projectID:   projectID,
		userID:      userID,
		args:        []any{},
		joins:       []string{},
		joinCounter: 0,
	}
}

// WithSinceTimestamp sets the timestamp used for "since_entered" period type
func (qb *QueryBuilder) WithSinceTimestamp(t time.Time) *QueryBuilder {
	qb.sinceTimestamp = &t
	return qb
}

// arg adds a value to the args list and returns the placeholder index
func (qb *QueryBuilder) arg(value any) string {
	qb.args = append(qb.args, value)
	return fmt.Sprintf("$%d", len(qb.args))
}

// nextJoinAlias generates the next join table alias
func (qb *QueryBuilder) nextJoinAlias() string {
	qb.joinCounter++
	return fmt.Sprintf("e%d", qb.joinCounter)
}

// Query generates a complete SELECT query with JOINs from a RuleSet
func (qb *QueryBuilder) Query(ruleSet rules.RuleSet) (QueryResult, error) {
	// Build the rule which may populate joins
	condition, err := qb.buildRule(&ruleSet.Rule)
	if err != nil {
		return QueryResult{}, err
	}

	// Start with base query
	sql := "SELECT u.id FROM users u"

	// Add all JOIN clauses
	if len(qb.joins) > 0 {
		sql += " " + strings.Join(qb.joins, " ")
	}

	// Build WHERE clause
	conditions := []string{fmt.Sprintf("u.project_id = %s", qb.arg(qb.projectID))}
	if qb.userID != nil {
		conditions = append(conditions, fmt.Sprintf("u.id = %s", qb.arg(*qb.userID)))
	}
	if condition != "" {
		conditions = append(conditions, condition)
	}

	sql += " WHERE " + strings.Join(conditions, " AND ")

	return QueryResult{
		SQL:  sql,
		Args: qb.args,
	}, nil
}
