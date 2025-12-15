package query

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/rules"
)

// QueryBuilder builds PostgreSQL queries from rule definitions
type QueryBuilder struct {
	projectID uuid.UUID
	args      []any
}

// QueryResult contains the generated SQL and arguments
type QueryResult struct {
	SQL  string
	Args []any
}

// NewQueryBuilder creates a new QueryBuilder
func NewQueryBuilder(projectID uuid.UUID) *QueryBuilder {
	return &QueryBuilder{
		projectID: projectID,
		args:      []any{},
	}
}

// Build generates a complete SELECT query from a RuleSet
func (qb *QueryBuilder) Build(ruleSet *rules.RuleSet) (QueryResult, error) {
	condition, err := qb.buildRule(&ruleSet.Rule)
	if err != nil {
		return QueryResult{}, err
	}

	conditions := []string{fmt.Sprintf("u.project_id = %s", qb.arg(qb.projectID))}
	if condition != "" {
		conditions = append(conditions, condition)
	}

	result := QueryResult{
		SQL:  fmt.Sprintf("SELECT u.* FROM users u WHERE %s", strings.Join(conditions, " AND ")),
		Args: qb.args,
	}

	return result, nil
}

// arg adds a value to the args list and returns the placeholder index
func (qb *QueryBuilder) arg(value any) string {
	qb.args = append(qb.args, value)
	return fmt.Sprintf("$%d", len(qb.args))
}

// BuildCondition generates only the WHERE clause from a RuleSet
func (qb *QueryBuilder) BuildCondition(ruleSet *rules.RuleSet) (QueryResult, error) {
	condition, err := qb.buildRule(&ruleSet.Rule)
	if err != nil {
		return QueryResult{}, err
	}

	conditions := []string{fmt.Sprintf("u.project_id = %s", qb.arg(qb.projectID))}
	if condition != "" {
		conditions = append(conditions, condition)
	}

	return QueryResult{
		SQL:  strings.Join(conditions, " AND "),
		Args: qb.args,
	}, nil
}
