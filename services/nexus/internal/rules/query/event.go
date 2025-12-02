package query

import (
	"fmt"
	"strings"

	"github.com/lunogram/platform/services/nexus/internal/rules"
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
		for _, child := range rule.Children {
			column := qb.buildColumnPath("e", child.Path, child.Type)
			condition, err := qb.buildComparison(column, child.Operator, child.Value, child.Type)
			if err != nil {
				return "", err
			}
			eventConditions = append(eventConditions, condition)
		}
	}

	// Add project_id filter for events
	eventConditions = append([]string{fmt.Sprintf("e.project_id = %s", qb.arg(qb.projectID))}, eventConditions...)

	existsClause := fmt.Sprintf(
		"EXISTS (SELECT 1 FROM user_events e WHERE e.user_id = u.id AND %s)",
		strings.Join(eventConditions, " AND "),
	)

	return existsClause, nil
}

// buildFrequencyRule builds SQL for frequency-based event rules
func (qb *QueryBuilder) buildFrequencyRule(rule *rules.Rule) (string, error) {
	freq := rule.Frequency

	// Build time period condition
	if freq.Period.Type != rules.PeriodTypeRolling {
		return "", fmt.Errorf("only rolling periods are currently supported")
	}

	interval := fmt.Sprintf("%d %s", freq.Period.Value, freq.Period.Unit.SQL())
	timePeriod := fmt.Sprintf("e.created_at >= NOW() - %s::interval", qb.arg(interval))

	// Build event conditions with project_id filter
	eventConditions := []string{fmt.Sprintf("e.project_id = %s", qb.arg(qb.projectID)), timePeriod}

	// Event name condition
	if rule.Value != nil {
		eventConditions = append(eventConditions, fmt.Sprintf("e.name = %s", qb.arg(rule.Value)))
	}

	// Child conditions (event attributes)
	if rule.HasChildren() {
		for _, child := range rule.Children {
			column := qb.buildColumnPath("e", child.Path, child.Type)
			condition, err := qb.buildComparison(column, child.Operator, child.Value, child.Type)
			if err != nil {
				return "", err
			}
			eventConditions = append(eventConditions, condition)
		}
	}

	whereClause := strings.Join(eventConditions, " AND ")

	// Build frequency comparison
	countComparison, err := qb.buildComparison(
		fmt.Sprintf("(SELECT COUNT(*) FROM user_events e WHERE e.user_id = u.id AND %s)", whereClause),
		freq.Operator,
		freq.Count,
		rules.RuleTypeNumber,
	)
	if err != nil {
		return "", err
	}

	return countComparison, nil
}
