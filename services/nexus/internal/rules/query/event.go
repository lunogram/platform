package query

import (
	"fmt"
	"strings"

	"github.com/lunogram/platform/services/nexus/internal/rules"
)

// buildEventRule builds SQL for event rules with optional frequency
func (qb *QueryBuilder) buildEventRule(rule rules.Rule) (string, error) {
	if rule.Frequency != nil {
		return qb.buildFrequencyRule(rule)
	}

	eventConditions := []string{}

	// Event name condition
	if rule.Value != nil {
		eventConditions = append(eventConditions, fmt.Sprintf("e.name = %s", qb.arg(rule.Value)))
	}

	// Child conditions (event attributes)
	if rule.HasChildren() {
		for _, child := range rule.Children {
			column, err := qb.buildColumnPath("e", child.Path, child.Type)
			if err != nil {
				return "", err
			}

			condition, err := qb.buildComparison(column, child.Operator, child.Value, child.Type)
			if err != nil {
				return "", err
			}
			eventConditions = append(eventConditions, condition)
		}
	}

	// Generate a unique alias for this event join
	alias := qb.nextJoinAlias()

	// Build the JOIN clause with subquery that joins user_events with events
	joinClause := fmt.Sprintf(
		"JOIN (SELECT DISTINCT ue.user_id FROM user_events ue JOIN events e ON e.id = ue.event_id WHERE %s) %s ON %s.user_id = u.id",
		strings.Join(eventConditions, " AND "),
		alias,
		alias,
	)

	qb.joins = append(qb.joins, joinClause)

	// Return empty condition since the filtering is in the JOIN
	return "", nil
}

// buildFrequencyRule builds SQL for frequency-based event rules
func (qb *QueryBuilder) buildFrequencyRule(rule rules.Rule) (string, error) {
	freq := rule.Frequency

	// Build time period condition
	if freq.Period.Type != rules.PeriodTypeRolling {
		return "", fmt.Errorf("only rolling periods are currently supported")
	}

	interval := fmt.Sprintf("%d %s", freq.Period.Value, freq.Period.Unit.SQL())

	// Start with base event conditions
	eventConditions := []string{fmt.Sprintf("ue.created_at >= NOW() - %s::interval", qb.arg(interval))}

	// Event name condition
	if rule.Value != nil {
		eventConditions = append(eventConditions, fmt.Sprintf("e.name = %s", qb.arg(rule.Value)))
	}

	// Child conditions (event attributes)
	if rule.HasChildren() {
		for _, child := range rule.Children {
			column, err := qb.buildColumnPath("e", child.Path, child.Type)
			if err != nil {
				return "", err
			}

			condition, err := qb.buildComparison(column, child.Operator, child.Value, child.Type)
			if err != nil {
				return "", err
			}
			eventConditions = append(eventConditions, condition)
		}
	}

	// Use JOINs if enabled, otherwise use subquery in WHERE
	// Generate a unique alias for this frequency join
	alias := qb.nextJoinAlias()

	// Build HAVING clause based on frequency operator
	var havingClause string
	switch freq.Operator {
	case rules.OperatorGreaterThan:
		havingClause = fmt.Sprintf("COUNT(*) > %s", qb.arg(freq.Count))
	case rules.OperatorGreaterEqual:
		havingClause = fmt.Sprintf("COUNT(*) >= %s", qb.arg(freq.Count))
	case rules.OperatorLessThan:
		havingClause = fmt.Sprintf("COUNT(*) < %s", qb.arg(freq.Count))
	case rules.OperatorLessEqual:
		havingClause = fmt.Sprintf("COUNT(*) <= %s", qb.arg(freq.Count))
	case rules.OperatorEquals:
		havingClause = fmt.Sprintf("COUNT(*) = %s", qb.arg(freq.Count))
	case rules.OperatorNotEquals:
		havingClause = fmt.Sprintf("COUNT(*) != %s", qb.arg(freq.Count))
	default:
		return "", fmt.Errorf("unsupported frequency operator: %s", freq.Operator)
	}

	// Build the JOIN clause with subquery using GROUP BY and HAVING
	joinClause := fmt.Sprintf(
		"JOIN (SELECT ue.user_id FROM user_events ue JOIN events e ON e.id = ue.event_id WHERE %s GROUP BY ue.user_id HAVING %s) %s ON %s.user_id = u.id",
		strings.Join(eventConditions, " AND "),
		havingClause,
		alias,
		alias,
	)

	qb.joins = append(qb.joins, joinClause)

	// Return empty condition since the filtering is in the JOIN
	return "", nil
}
