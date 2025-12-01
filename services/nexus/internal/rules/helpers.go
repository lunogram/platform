package rules

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/oliveagle/jsonpath"
)

var reservedPaths = map[RuleGroup][]string{
	RuleGroupUser: {
		"external_id",
		"email",
		"phone",
		"timezone",
		"locale",
		"created_at",
		"has_push_device",
	},
	RuleGroupEvent: {
		"name",
		"created_at",
	},
	RuleGroupParent: {},
}

func queryValue[T any](value map[string]interface{}, rule *RuleTree, cast func(interface{}) (T, error)) ([]T, error) {
	path := rule.Path
	if path == "" {
		return nil, nil
	}

	if !strings.HasPrefix(path, "$.") && !strings.HasPrefix(path, "$[") {
		path = "$." + path
	}

	result, err := jsonpath.JsonPathLookup(value, path)
	if err != nil {
		return nil, nil
	}

	var results []T
	switch v := result.(type) {
	case []interface{}:
		for _, item := range v {
			if casted, err := cast(item); err == nil {
				results = append(results, casted)
			}
		}
	default:
		if casted, err := cast(result); err == nil {
			results = append(results, casted)
		}
	}

	return results, nil
}

func formattedQueryValue(value interface{}) string {
	if s, ok := value.(string); ok {
		return fmt.Sprintf("'%s'", s)
	}
	return fmt.Sprint(value)
}

func castRuleType(rule *RuleTree) string {
	switch rule.Type {
	case RuleTypeString:
		return "text"
	case RuleTypeNumber:
		return "int"
	case RuleTypeBoolean:
		return "boolean"
	case RuleTypeDate:
		return "timestamp"
	case RuleTypeArray:
		return "jsonb"
	case RuleTypeWrapper:
		return "wrapper"
	default:
		return "text"
	}
}

func queryPath(rule *RuleTree) string {
	column := strings.TrimPrefix(rule.Path, "$.")
	
	if contains(reservedPaths[rule.Group], column) {
		return column
	}

	parts := strings.Split(column, ".")
	quotedParts := make([]string, len(parts))
	for i, p := range parts {
		quotedParts[i] = fmt.Sprintf("'%s'", p)
	}
	
	pathParts := []string{"data"}
	pathParts = append(pathParts, quotedParts...)

	var path string
	if len(pathParts) > 1 {
		path = strings.Join(pathParts[:len(pathParts)-1], "->") + "->>" + pathParts[len(pathParts)-1]
	} else {
		path = pathParts[0]
	}

	return fmt.Sprintf("(%s)::%s", path, castRuleType(rule))
}

func whereQuery(path string, operator Operator, value interface{}) string {
	if arr, ok := value.([]interface{}); ok {
		parts := make([]string, len(arr))
		for i, v := range arr {
			parts[i] = formattedQueryValue(v)
		}
		joined := strings.Join(parts, ",")

		if operator == OpAny {
			return fmt.Sprintf("%s IN (%s)", path, joined)
		} else if operator == OpNone {
			return fmt.Sprintf("%s NOT IN (%s)", path, joined)
		}
	}

	switch operator {
	case OpContains:
		return fmt.Sprintf("%s LIKE '%%%v%%'", path, value)
	case OpNotContain:
		return fmt.Sprintf("%s NOT LIKE '%%%v%%'", path, value)
	case OpStartsWith:
		return fmt.Sprintf("%s LIKE '%v%%'", path, value)
	case OpNotStartWith:
		return fmt.Sprintf("%s NOT LIKE '%v%%'", path, value)
	default:
		return fmt.Sprintf("%s %s %s", path, operator, formattedQueryValue(value))
	}
}

func whereQueryNullable(path string, isNull bool) string {
	if isNull {
		return fmt.Sprintf("%s IS NULL", path)
	}
	return fmt.Sprintf("%s IS NOT NULL", path)
}

func compile[T any](rule *RuleTree, cast func(interface{}) (T, error)) (T, error) {
	var zero T
	var value interface{}
	
	if len(rule.Value) > 0 {
		if err := json.Unmarshal(rule.Value, &value); err != nil {
			return zero, err
		}
	}

	return cast(value)
}

func isEventWrapper(rule *RuleTree) bool {
	return rule.Group == RuleGroupEvent &&
		(rule.Path == "$.name" || rule.Path == "name")
}

func dateFromPeriod(period EventRulePeriod) (startDate time.Time, endDate *time.Time) {
	if period.Type == PeriodTypeFixed {
		if period.StartDate != nil {
			startDate, _ = time.Parse(time.RFC3339, *period.StartDate)
		}
		if period.EndDate != nil {
			t, _ := time.Parse(time.RFC3339, *period.EndDate)
			endDate = &t
		}
		return
	}

	if period.Unit == nil || period.Value == nil {
		return time.Now(), nil
	}

	intervals := map[TimeUnit]time.Duration{
		TimeUnitMinute: time.Minute,
		TimeUnitHour:   time.Hour,
		TimeUnitDay:    24 * time.Hour,
		TimeUnitWeek:   7 * 24 * time.Hour,
		TimeUnitMonth:  30 * 24 * time.Hour,
		TimeUnitYear:   365 * 24 * time.Hour,
	}

	offset := time.Duration(*period.Value) * intervals[*period.Unit]
	startDate = time.Now().Add(-offset)
	return
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func marshalValue(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func parseTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}
