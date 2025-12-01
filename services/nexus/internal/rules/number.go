package rules

import (
	"fmt"
	"strconv"
)

type NumberRule struct{}

func (r *NumberRule) Check(params RuleCheckParams) bool {
	values, err := queryValue(params.Value, params.Rule, func(item interface{}) (float64, error) {
		switch v := item.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		case int64:
			return float64(v), nil
		case string:
			return strconv.ParseFloat(v, 64)
		default:
			return 0, fmt.Errorf("cannot convert to number")
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

	ruleValue, err := compile(params.Rule, func(item interface{}) (float64, error) {
		switch v := item.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		case string:
			return strconv.ParseFloat(v, 64)
		default:
			return 0, fmt.Errorf("cannot convert to number")
		}
	})

	if err != nil {
		return false
	}

	for _, v := range values {
		if numComp(v, params.Rule.Operator, ruleValue) {
			return true
		}
	}

	return false
}

func (r *NumberRule) Query(params RuleQueryParams) string {
	path := queryPath(params.Rule)

	if params.Rule.Operator == OpIsSet {
		return whereQueryNullable(path, false)
	}

	if params.Rule.Operator == OpIsNotSet {
		return whereQueryNullable(path, true)
	}

	ruleValue, err := compile(params.Rule, func(item interface{}) (float64, error) {
		switch v := item.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		case string:
			return strconv.ParseFloat(v, 64)
		default:
			return 0, fmt.Errorf("cannot convert to number")
		}
	})

	if err != nil {
		return ""
	}

	return whereQuery(path, params.Rule.Operator, ruleValue)
}

func numComp(left float64, operator Operator, right float64) bool {
	switch operator {
	case OpEqual:
		return left == right
	case OpNotEqual:
		return left != right
	case OpLessThan:
		return left < right
	case OpGreaterThan:
		return left > right
	case OpLessThanEq:
		return left <= right
	case OpGreaterThanEq:
		return left >= right
	default:
		return false
	}
}
