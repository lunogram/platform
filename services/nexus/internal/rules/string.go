package rules

import (
	"fmt"
	"strings"
)

type StringRule struct{}

func (r *StringRule) Check(params RuleCheckParams) bool {
	values, err := queryValue(params.Value, params.Rule, func(item interface{}) (string, error) {
		switch v := item.(type) {
		case string:
			return v, nil
		case bool:
			return fmt.Sprint(v), nil
		case int, int64, float64:
			return fmt.Sprint(v), nil
		default:
			return "", fmt.Errorf("cannot convert to string")
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

	if params.Rule.Operator == OpEmpty {
		for _, v := range values {
			if v != "" {
				return false
			}
		}
		return true
	}

	ruleValue, err := compile(params.Rule, func(item interface{}) (string, error) {
		return fmt.Sprint(item), nil
	})

	if err != nil {
		return false
	}

	for _, v := range values {
		match := false
		switch params.Rule.Operator {
		case OpEqual:
			match = v == ruleValue
		case OpNotEqual:
			match = v != ruleValue
		case OpStartsWith:
			match = strings.HasPrefix(v, ruleValue)
		case OpNotStartWith:
			match = !strings.HasPrefix(v, ruleValue)
		case OpEndsWith:
			match = strings.HasSuffix(v, ruleValue)
		case OpContains:
			match = strings.Contains(v, ruleValue)
		case OpNotContain:
			match = !strings.Contains(v, ruleValue)
		}
		if match {
			return true
		}
	}

	return false
}

func (r *StringRule) Query(params RuleQueryParams) string {
	path := queryPath(params.Rule)

	if params.Rule.Operator == OpIsSet {
		return whereQueryNullable(path, false)
	}

	if params.Rule.Operator == OpIsNotSet {
		return whereQueryNullable(path, true)
	}

	if params.Rule.Operator == OpEmpty {
		return whereQuery(path, OpEqual, "")
	}

	ruleValue, err := compile(params.Rule, func(item interface{}) (string, error) {
		return fmt.Sprint(item), nil
	})

	if err != nil {
		return ""
	}

	return whereQuery(path, params.Rule.Operator, ruleValue)
}
