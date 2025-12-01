package rules

import (
	"fmt"
	"time"
)

type DateRule struct{}

func (r *DateRule) Check(params RuleCheckParams) bool {
	values, err := queryValue(params.Value, params.Rule, func(item interface{}) (time.Time, error) {
		switch v := item.(type) {
		case string:
			return time.Parse(time.RFC3339, v)
		case int64:
			return time.Unix(v, 0), nil
		case float64:
			return time.Unix(int64(v), 0), nil
		case time.Time:
			return v, nil
		default:
			return time.Time{}, fmt.Errorf("cannot convert to date")
		}
	})

	if err != nil {
		return false
	}

	if params.Rule.Operator == OpIsSet {
		return len(values) > 0
	}

	if params.Rule.Operator == OpIsNotSet {
		return len(values) == 0
	}

	ruleValue, err := compile(params.Rule, func(item interface{}) (time.Time, error) {
		switch v := item.(type) {
		case string:
			return time.Parse(time.RFC3339, v)
		case int64:
			return time.Unix(v, 0), nil
		case float64:
			return time.Unix(int64(v), 0), nil
		default:
			return time.Time{}, fmt.Errorf("cannot convert to date")
		}
	})

	if err != nil {
		return false
	}

	for _, d := range values {
		match := false
		switch params.Rule.Operator {
		case OpEqual:
			match = d.Equal(ruleValue)
		case OpNotEqual:
			match = !d.Equal(ruleValue)
		case OpLessThan:
			match = d.Before(ruleValue)
		case OpLessThanEq:
			match = d.Before(ruleValue) || d.Equal(ruleValue)
		case OpGreaterThan:
			match = d.After(ruleValue)
		case OpGreaterThanEq:
			match = d.Equal(ruleValue) || d.After(ruleValue)
		case OpIsSameDay:
			match = isSameDay(d, ruleValue)
		}
		if match {
			return true
		}
	}

	return false
}

func (r *DateRule) Query(params RuleQueryParams) string {
	path := queryPath(params.Rule)

	if params.Rule.Operator == OpIsSet {
		return whereQueryNullable(path, false)
	}

	if params.Rule.Operator == OpIsNotSet {
		return whereQueryNullable(path, true)
	}

	ruleValue, err := compile(params.Rule, func(item interface{}) (time.Time, error) {
		switch v := item.(type) {
		case string:
			return time.Parse(time.RFC3339, v)
		case int64:
			return time.Unix(v, 0), nil
		case float64:
			return time.Unix(int64(v), 0), nil
		default:
			return time.Time{}, fmt.Errorf("cannot convert to date")
		}
	})

	if err != nil {
		return ""
	}

	if params.Rule.Operator == OpIsSameDay {
		dateStr := ruleValue.Format("2006-01-02")
		return fmt.Sprintf("toDate(%s) = toDate('%s')", path, dateStr)
	}

	timestamp := ruleValue.UnixMilli()
	return fmt.Sprintf("%s %s parseDateTimeBestEffortOrNull('%d')", path, params.Rule.Operator, timestamp)
}

func isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}
