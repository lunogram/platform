package query

import (
	"fmt"
	"strings"

	"github.com/lunogram/platform/internal/rules"
)

// buildEventRule builds SQL for event rules with optional frequency
func (qb *QueryBuilder) buildEventRule(rule *rules.Rule) (string, error) {
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
		for i := range rule.Children {
			child := &rule.Children[i]
			condition, err := qb.buildConditionFromPath("ue", child.Path, child)
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
func (qb *QueryBuilder) buildFrequencyRule(rule *rules.Rule) (string, error) {
	freq := rule.Frequency

	// Build time period condition
	var timeCondition string
	switch freq.Period.Type {
	case rules.PeriodTypeRolling:
		interval := fmt.Sprintf("%d %s", freq.Period.Value, freq.Period.Unit.SQL())
		timeCondition = fmt.Sprintf("ue.created_at >= NOW() - %s::interval", qb.arg(interval))
	case rules.PeriodTypeSinceEntered:
		if qb.sinceTimestamp == nil {
			return "", fmt.Errorf("since_entered period type requires a since timestamp")
		}
		timeCondition = fmt.Sprintf("ue.created_at >= %s", qb.arg(*qb.sinceTimestamp))
	default:
		return "", fmt.Errorf("unsupported period type: %s", freq.Period.Type)
	}

	// Start with base event conditions
	eventConditions := []string{timeCondition}

	// Event name condition
	if rule.Value != nil {
		eventConditions = append(eventConditions, fmt.Sprintf("e.name = %s", qb.arg(rule.Value)))
	}

	// Child conditions (event attributes)
	if rule.HasChildren() {
		for i := range rule.Children {
			child := &rule.Children[i]
			condition, err := qb.buildConditionFromPath("ue", child.Path, child)
			if err != nil {
				return "", err
			}
			eventConditions = append(eventConditions, condition)
		}
	}

	// Build HAVING clause based on frequency operator
	havingClause, err := qb.buildHavingClause(freq)
	if err != nil {
		return "", err
	}

	// Generate a unique alias for this frequency join
	alias := qb.nextJoinAlias()

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
