package rules

import "fmt"

type ArrayRule struct{}

func (r *ArrayRule) Check(params RuleCheckParams) bool {
	values, err := queryValue(params.Value, params.Rule, func(item interface{}) (interface{}, error) {
		return item, nil
	})

	if err != nil {
		return false
	}

	if params.Rule.Operator == OpIsSet {
		for _, v := range values {
			if _, ok := v.([]interface{}); ok {
				return true
			}
		}
		return false
	}

	if params.Rule.Operator == OpIsNotSet {
		for _, v := range values {
			if _, ok := v.([]interface{}); ok {
				return false
			}
		}
		return true
	}

	if params.Rule.Operator == OpEmpty {
		for _, v := range values {
			if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
				return false
			}
		}
		return true
	}

	return false
}

func (r *ArrayRule) Query(params RuleQueryParams) string {
	path := queryPath(params.Rule)

	if params.Rule.Operator == OpIsSet {
		return whereQueryNullable(path, false)
	}

	if params.Rule.Operator == OpIsNotSet {
		return whereQueryNullable(path, true)
	}

	if params.Rule.Operator == OpEmpty {
		return fmt.Sprintf("%s = '[]'::jsonb", path)
	}

	if params.Rule.Operator == OpContains {
		return fmt.Sprintf("jsonb_exists(%s, %s)", path, formattedQueryValue(params.Rule.Value))
	}

	return ""
}
