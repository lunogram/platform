package rules

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type WrapperRule struct{}

func (r *WrapperRule) Check(params RuleCheckParams) bool {
	if isEventWrapper(params.Rule) {
		return r.checkEventWrapper(params)
	}
	return r.checkWrapper(params)
}

func (r *WrapperRule) checkWrapper(params RuleCheckParams) bool {
	if params.Rule.Children == nil {
		return true
	}

	predicate := func(child *RuleTree) bool {
		checker := params.Registry.Get(child.Type)
		if checker == nil {
			return false
		}
		return checker.Check(RuleCheckParams{
			Registry: params.Registry,
			Input:    params.Input,
			Rule:     child,
			Value:    params.Value,
		})
	}

	if params.Rule.Operator == OpOr {
		for _, child := range params.Rule.Children {
			if predicate(child) {
				return true
			}
		}
		return false
	}

	if params.Rule.Operator == OpAnd {
		for _, child := range params.Rule.Children {
			if !predicate(child) {
				return false
			}
		}
		return true
	}

	return false
}

func (r *WrapperRule) checkEventWrapper(params RuleCheckParams) bool {
	var eventName string
	if len(params.Rule.Value) > 0 {
		json.Unmarshal(params.Rule.Value, &eventName)
	}

	if eventName == "" {
		return false
	}

	operator := params.Rule.Operator
	if operator == "" {
		operator = OpGreaterThanEq
	}

	count := 1
	var period *EventRulePeriod
	if params.Rule.Frequency != nil {
		operator = params.Rule.Frequency.Operator
		count = params.Rule.Frequency.Count
		period = &params.Rule.Frequency.Period
	}

	checkCount := 0
	for _, event := range params.Input.Events {
		name, _ := event["name"].(string)
		if name != eventName {
			continue
		}

		if period != nil {
			startDate, endDate := dateFromPeriod(*period)
			eventTime, ok := event["created_at"].(string)
			if ok {
				createdAt, err := parseTime(eventTime)
				if err == nil {
					if createdAt.Before(startDate) {
						continue
					}
					if endDate != nil && createdAt.After(*endDate) {
						continue
					}
				}
			}
		}

		if r.checkWrapper(RuleCheckParams{
			Registry: params.Registry,
			Input:    params.Input,
			Rule:     params.Rule,
			Value:    convertToMap(event),
		}) {
			checkCount++

			if numComp(float64(checkCount), operator, float64(count)) &&
				operator != OpLessThan && operator != OpLessThanEq {
				return true
			}
		}
	}

	return numComp(float64(checkCount), operator, float64(count))
}

func (r *WrapperRule) Query(params RuleQueryParams) string {
	if isEventWrapper(params.Rule) {
		return r.eventWrapperQuery(params)
	}

	operator := params.Rule.Operator
	if operator != OpAnd && operator != OpOr {
		return ""
	}

	baseQuery := "SELECT id FROM users"
	children := params.Rule.Children
	if children == nil || len(children) == 0 {
		return fmt.Sprintf("%s WHERE project_id = '%s'", baseQuery, params.ProjectID)
	}

	var userRules []*RuleTree
	var eventRules []*RuleTree

	for _, child := range children {
		if child.Group == RuleGroupUser {
			userRules = append(userRules, child)
		} else if child.Group == RuleGroupEvent {
			eventRules = append(eventRules, child)
		}
	}

	parentOperator := "INTERSECT"
	if params.Rule.Operator == OpOr {
		parentOperator = "UNION DISTINCT"
	}

	var queries []string

	if len(userRules) > 0 {
		conditions := r.buildUserConditions(userRules, params.Registry, params.ProjectID, operator)
		if conditions != "" {
			userQuery := fmt.Sprintf("%s WHERE %s AND project_id = '%s'",
				baseQuery,
				conditions,
				params.ProjectID,
			)
			queries = append(queries, userQuery)
		}
	}

	if len(eventRules) > 0 {
		var eventQueries []string
		for _, child := range eventRules {
			checker := params.Registry.Get(child.Type)
			if checker != nil {
				eventQueries = append(eventQueries, checker.Query(RuleQueryParams{
					Registry:  params.Registry,
					ProjectID: params.ProjectID,
					Rule:      child,
				}))
			}
		}
		if len(eventQueries) > 0 {
			queries = append(queries, strings.Join(eventQueries, fmt.Sprintf(" %s ", parentOperator)))
		}
	}

	if len(queries) == 0 {
		return fmt.Sprintf("%s WHERE project_id = '%s'", baseQuery, params.ProjectID)
	}

	return strings.Join(queries, fmt.Sprintf(" %s ", parentOperator))
}

func (r *WrapperRule) buildUserConditions(rules []*RuleTree, registry Registry, projectID uuid.UUID, operator Operator) string {
	var conditions []string

	for _, rule := range rules {
		var condition string

		// Handle nested wrappers
		if rule.Type == RuleTypeWrapper {
			// Recursively build conditions for nested wrappers
			nestedConditions := r.buildUserConditions(rule.Children, registry, projectID, rule.Operator)
			if nestedConditions != "" {
				// Wrap in parentheses for proper precedence
				condition = fmt.Sprintf("(%s)", nestedConditions)
			}
		} else {
			// For non-wrapper rules, get the WHERE clause condition
			checker := registry.Get(rule.Type)
			if checker != nil {
				condition = checker.Query(RuleQueryParams{
					Registry:  registry,
					ProjectID: projectID,
					Rule:      rule,
				})
			}
		}

		if condition != "" {
			conditions = append(conditions, condition)
		}
	}

	if len(conditions) == 0 {
		return ""
	}

	return strings.Join(conditions, fmt.Sprintf(" %s ", operator))
}

func (r *WrapperRule) eventWrapperQuery(params RuleQueryParams) string {
	var eventName string
	if len(params.Rule.Value) > 0 {
		json.Unmarshal(params.Rule.Value, &eventName)
	}

	children := params.Rule.Children
	operator := params.Rule.Operator

	if operator != OpAnd && operator != OpOr {
		return ""
	}

	var filters []string
	for _, child := range children {
		checker := params.Registry.Get(child.Type)
		if checker != nil {
			filters = append(filters, checker.Query(RuleQueryParams{
				Registry:  params.Registry,
				ProjectID: params.ProjectID,
				Rule:      child,
			}))
		}
	}

	where := []string{
		fmt.Sprintf("project_id = '%s'", params.ProjectID),
		whereQuery("name", OpEqual, eventName),
	}

	if len(filters) > 0 {
		where = append(where, fmt.Sprintf("(%s)", strings.Join(filters, fmt.Sprintf(" %s ", operator))))
	}

	if params.Rule.Frequency != nil && params.Rule.Frequency.Period.Type != "" {
		periodQuery := r.periodQuery(params.Rule.Frequency.Period)
		if periodQuery != "" {
			where = append(where, periodQuery)
		}
	}

	having := r.frequencyQuery(params.Rule.Frequency)

	return fmt.Sprintf(`
		SELECT user_id AS id 
		FROM user_events 
		WHERE %s
		GROUP BY project_id, user_id
		HAVING %s`, strings.Join(where, " AND "), having)
}

func (r *WrapperRule) periodQuery(period EventRulePeriod) string {
	if period.Type == PeriodTypeRolling && period.Unit != nil && period.Value != nil {
		return fmt.Sprintf("created_at >= now() - INTERVAL '%d %s'", *period.Value, *period.Unit)
	}

	if period.Type == PeriodTypeFixed {
		if period.StartDate != nil && period.EndDate == nil {
			return fmt.Sprintf("created_at >= '%s'", *period.StartDate)
		}
		if period.StartDate != nil && period.EndDate != nil {
			return fmt.Sprintf("(created_at >= '%s' AND created_at <= '%s')", *period.StartDate, *period.EndDate)
		}
	}

	return ""
}

func (r *WrapperRule) frequencyQuery(frequency *EventRuleFrequency) string {
	count := 1
	operator := OpGreaterThanEq

	if frequency != nil {
		count = frequency.Count
		operator = frequency.Operator
	}

	return whereQuery("count(*)", operator, count)
}

func convertToMap(event TemplateEvent) map[string]interface{} {
	return map[string]interface{}(event)
}
