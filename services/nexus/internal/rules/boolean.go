package rules

import "fmt"

type BooleanRule struct{}

func (r *BooleanRule) Check(params RuleCheckParams) bool {
	values, err := queryValue(params.Value, params.Rule, func(item interface{}) (bool, error) {
		switch v := item.(type) {
		case bool:
			return v, nil
		case string:
			return v == "true", nil
		case int, int64:
			return v == 1, nil
		case float64:
			return v == 1.0, nil
		default:
			return false, fmt.Errorf("cannot convert to boolean")
		}
	})

	if err != nil {
		return false
	}

	hasTrue := false
	for _, v := range values {
		if v {
			hasTrue = true
			break
		}
	}

	if params.Rule.Operator == OpNotEqual {
		return !hasTrue
	}

	return hasTrue
}

func (r *BooleanRule) Query(params RuleQueryParams) string {
	path := queryPath(params.Rule)

	castValue := func(item interface{}) bool {
		switch v := item.(type) {
		case bool:
			return v
		case string:
			return v == "true"
		case int, int64:
			return v == 1
		case float64:
			return v == 1.0
		default:
			return false
		}
	}

	var value interface{}
	if len(params.Rule.Value) > 0 {
		compile(params.Rule, func(item interface{}) (interface{}, error) {
			value = castValue(item)
			return value, nil
		})
	}

	return whereQuery(path, OpEqual, value)
}
