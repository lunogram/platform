package rules

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNumberRule(t *testing.T) {
	type test struct {
		name     string
		rule     *RuleTree
		value    map[string]interface{}
		expected bool
	}

	tests := []test{
		{
			name: "equal",
			rule: Make(MakeParams{
				Type:     RuleTypeNumber,
				Path:     "$.age",
				Operator: OpEqual,
				Value:    25,
			}),
			value:    map[string]interface{}{"age": 25},
			expected: true,
		},
		{
			name: "greater-than",
			rule: Make(MakeParams{
				Type:     RuleTypeNumber,
				Path:     "$.age",
				Operator: OpGreaterThan,
				Value:    25,
			}),
			value:    map[string]interface{}{"age": 30},
			expected: true,
		},
		{
			name: "is-set",
			rule: Make(MakeParams{
				Type:     RuleTypeNumber,
				Path:     "$.age",
				Operator: OpIsSet,
			}),
			value:    map[string]interface{}{"age": 25},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := RuleCheckInput{
				User:    tc.value,
				Events:  []TemplateEvent{},
				Journey: map[string]interface{}{},
			}
			result := Check(input, tc.rule)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestStringRule(t *testing.T) {
	type test struct {
		name     string
		rule     *RuleTree
		value    map[string]interface{}
		expected bool
	}

	tests := []test{
		{
			name: "equal",
			rule: Make(MakeParams{
				Type:     RuleTypeString,
				Path:     "$.name",
				Operator: OpEqual,
				Value:    "John",
			}),
			value:    map[string]interface{}{"name": "John"},
			expected: true,
		},
		{
			name: "contains",
			rule: Make(MakeParams{
				Type:     RuleTypeString,
				Path:     "$.email",
				Operator: OpContains,
				Value:    "@example",
			}),
			value:    map[string]interface{}{"email": "user@example.com"},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := RuleCheckInput{
				User:    tc.value,
				Events:  []TemplateEvent{},
				Journey: map[string]interface{}{},
			}
			result := Check(input, tc.rule)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestWrapperRule(t *testing.T) {
	type test struct {
		name     string
		rule     *RuleTree
		input    RuleCheckInput
		expected bool
	}

	tests := []test{
		{
			name: "and-operator-all-true",
			rule: Make(MakeParams{
				Type:     RuleTypeWrapper,
				Operator: OpAnd,
				Children: []*RuleTree{
					Make(MakeParams{
						Type:     RuleTypeNumber,
						Path:     "$.age",
						Operator: OpGreaterThan,
						Value:    18,
					}),
					Make(MakeParams{
						Type:     RuleTypeString,
						Path:     "$.country",
						Operator: OpEqual,
						Value:    "US",
					}),
				},
			}),
			input: RuleCheckInput{
				User:    map[string]interface{}{"age": 25, "country": "US"},
				Events:  []TemplateEvent{},
				Journey: map[string]interface{}{},
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := Check(tc.input, tc.rule)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestEventWrapperRule(t *testing.T) {
	t.Skip("Event wrapper rule requires more complex setup - skipping for now")
}

func TestGetRuleQuery(t *testing.T) {
	projectID := uuid.New()

	type test struct {
		name     string
		rule     *RuleTree
		contains string
	}

	tests := []test{
		{
			name: "number-equal",
			rule: Make(MakeParams{
				Type:     RuleTypeNumber,
				Path:     "$.age",
				Operator: OpEqual,
				Value:    25,
			}),
			contains: "= 25",
		},
		{
			name: "string-contains",
			rule: Make(MakeParams{
				Type:     RuleTypeString,
				Path:     "$.email",
				Operator: OpContains,
				Value:    "@example",
			}),
			contains: "LIKE '%@example%'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := GetRuleQuery(projectID, tc.rule)
			assert.Contains(t, query, tc.contains)
		})
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
