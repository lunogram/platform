package render

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderString(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		template string
		data     map[string]any
		expected string
		wantErr  bool
	}{
		"no variables - passthrough": {
			template: "hello world",
			data:     map[string]any{},
			expected: "hello world",
		},
		"simple variable": {
			template: "Hello {{ user.name }}",
			data: map[string]any{
				"user": map[string]any{"name": "Alice"},
			},
			expected: "Hello Alice",
		},
		"multiple variables": {
			template: "{{ user.first_name }} {{ user.last_name }}",
			data: map[string]any{
				"user": map[string]any{
					"first_name": "John",
					"last_name":  "Doe",
				},
			},
			expected: "John Doe",
		},
		"nested journey data": {
			template: "{{ journey.entrance.product_id }}",
			data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"product_id": "prod-123",
					},
				},
			},
			expected: "prod-123",
		},
		"missing variable renders empty": {
			template: "Hello {{ user.missing }}",
			data: map[string]any{
				"user": map[string]any{"name": "Alice"},
			},
			expected: "Hello ",
		},
		"empty data map": {
			template: "{{ user.name }}",
			data:     map[string]any{},
			expected: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := RenderString(tc.template, tc.data)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestRenderTime(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		template *string
		data     map[string]any
		expected *time.Time
		wantErr  bool
	}{
		"nil pointer returns nil": {
			template: nil,
			data:     map[string]any{},
			expected: nil,
		},
		"empty string returns nil": {
			template: ptr(""),
			data:     map[string]any{},
			expected: nil,
		},
		"static RFC3339 timestamp": {
			template: ptr("2025-06-01T00:00:00Z"),
			data:     map[string]any{},
			expected: timePtr(time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)),
		},
		"liquid template resolving to timestamp": {
			template: ptr("{{ journey.entrance.scheduled_at }}"),
			data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"scheduled_at": "2025-12-25T10:30:00Z",
					},
				},
			},
			expected: timePtr(time.Date(2025, 12, 25, 10, 30, 0, 0, time.UTC)),
		},
		"invalid RFC3339 format": {
			template: ptr("not-a-date"),
			data:     map[string]any{},
			wantErr:  true,
		},
		"liquid template rendering error": {
			template: ptr("{% invalid %}"),
			data:     map[string]any{},
			wantErr:  true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := RenderTime(tc.template, tc.data)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.True(t, tc.expected.Equal(*result), "expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func ptr(s string) *string           { return &s }
func timePtr(t time.Time) *time.Time { return &t }

func TestRenderJSON(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input    string
		data     map[string]any
		expected string
		wantErr  bool
	}{
		"nil input": {
			input:    "",
			data:     map[string]any{},
			expected: "",
		},
		"flat object with variables": {
			input: `{"email":"{{ user.email }}","name":"{{ user.name }}"}`,
			data: map[string]any{
				"user": map[string]any{
					"email": "alice@example.com",
					"name":  "Alice",
				},
			},
			expected: `{"email":"alice@example.com","name":"Alice"}`,
		},
		"nested object with variables": {
			input: `{"contact":{"email":"{{ user.email }}"},"ref":"{{ journey.entrance.id }}"}`,
			data: map[string]any{
				"user": map[string]any{
					"email": "bob@example.com",
				},
				"journey": map[string]any{
					"entrance": map[string]any{
						"id": "ref-456",
					},
				},
			},
			expected: `{"contact":{"email":"bob@example.com"},"ref":"ref-456"}`,
		},
		"array with variables": {
			input: `{"tags":["{{ user.role }}","static"]}`,
			data: map[string]any{
				"user": map[string]any{"role": "admin"},
			},
			expected: `{"tags":["admin","static"]}`,
		},
		"non-string values preserved": {
			input: `{"count":42,"active":true,"name":"{{ user.name }}"}`,
			data: map[string]any{
				"user": map[string]any{"name": "Test"},
			},
			expected: `{"active":true,"count":42,"name":"Test"}`,
		},
		"no variables - passthrough": {
			input:    `{"key":"value","num":123}`,
			data:     map[string]any{},
			expected: `{"key":"value","num":123}`,
		},
		"mixed static and variable strings": {
			input: `{"greeting":"Hello {{ user.name }}, welcome!"}`,
			data: map[string]any{
				"user": map[string]any{"name": "Charlie"},
			},
			expected: `{"greeting":"Hello Charlie, welcome!"}`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var input json.RawMessage
			if tc.input != "" {
				input = json.RawMessage(tc.input)
			}

			result, err := RenderJSON(input, tc.data)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			if tc.input == "" {
				assert.Nil(t, result)
				return
			}

			require.NoError(t, err)

			// Compare as parsed JSON to avoid key ordering issues
			var expectedMap, resultMap any
			require.NoError(t, json.Unmarshal([]byte(tc.expected), &expectedMap))
			require.NoError(t, json.Unmarshal(result, &resultMap))
			assert.Equal(t, expectedMap, resultMap)
		})
	}
}

func TestRenderRuleSet(t *testing.T) {
	t.Parallel()

	data := map[string]any{
		"user": map[string]any{
			"email": "alice@example.com",
			"name":  "Alice",
			"plan":  "pro",
		},
		"journey": map[string]any{
			"entrance": map[string]any{
				"product_id": "prod-123",
			},
		},
	}

	t.Run("leaf string value with variable is resolved", func(t *testing.T) {
		rs := rules.RuleSet{
			Rule: rules.Rule{
				UUID:     uuid.New(),
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupUser,
				Operator: rules.OperatorAnd,
				Children: []rules.Rule{
					{
						UUID:     uuid.New(),
						Type:     rules.RuleTypeString,
						Group:    rules.RuleGroupUser,
						Path:     "email",
						Operator: rules.OperatorEquals,
						Value:    "{{ user.email }}",
					},
				},
			},
		}

		result, err := RenderRuleSet(rs, data)
		require.NoError(t, err)
		assert.Equal(t, "alice@example.com", result.Children[0].Value)
	})

	t.Run("wrapper value is not resolved", func(t *testing.T) {
		rs := rules.RuleSet{
			Rule: rules.Rule{
				UUID:     uuid.New(),
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupEvent,
				Operator: rules.OperatorAnd,
				Value:    "page_viewed",
				Children: []rules.Rule{},
			},
		}

		result, err := RenderRuleSet(rs, data)
		require.NoError(t, err)
		assert.Equal(t, "page_viewed", result.Value)
	})

	t.Run("non-string value is not modified", func(t *testing.T) {
		rs := rules.RuleSet{
			Rule: rules.Rule{
				UUID:     uuid.New(),
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupUser,
				Operator: rules.OperatorAnd,
				Children: []rules.Rule{
					{
						UUID:     uuid.New(),
						Type:     rules.RuleTypeNumber,
						Group:    rules.RuleGroupUser,
						Path:     "age",
						Operator: rules.OperatorGreaterThan,
						Value:    float64(25),
					},
				},
			},
		}

		result, err := RenderRuleSet(rs, data)
		require.NoError(t, err)
		assert.Equal(t, float64(25), result.Children[0].Value)
	})

	t.Run("string value without template is unchanged", func(t *testing.T) {
		rs := rules.RuleSet{
			Rule: rules.Rule{
				UUID:     uuid.New(),
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupUser,
				Operator: rules.OperatorAnd,
				Children: []rules.Rule{
					{
						UUID:     uuid.New(),
						Type:     rules.RuleTypeString,
						Group:    rules.RuleGroupUser,
						Path:     "plan",
						Operator: rules.OperatorEquals,
						Value:    "enterprise",
					},
				},
			},
		}

		result, err := RenderRuleSet(rs, data)
		require.NoError(t, err)
		assert.Equal(t, "enterprise", result.Children[0].Value)
	})

	t.Run("deeply nested children are resolved", func(t *testing.T) {
		rs := rules.RuleSet{
			Rule: rules.Rule{
				UUID:     uuid.New(),
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupUser,
				Operator: rules.OperatorAnd,
				Children: []rules.Rule{
					{
						UUID:     uuid.New(),
						Type:     rules.RuleTypeWrapper,
						Group:    rules.RuleGroupUser,
						Operator: rules.OperatorOr,
						Children: []rules.Rule{
							{
								UUID:     uuid.New(),
								Type:     rules.RuleTypeString,
								Group:    rules.RuleGroupUser,
								Path:     "email",
								Operator: rules.OperatorEquals,
								Value:    "{{ user.email }}",
							},
							{
								UUID:     uuid.New(),
								Type:     rules.RuleTypeString,
								Group:    rules.RuleGroupUser,
								Path:     "plan",
								Operator: rules.OperatorEquals,
								Value:    "{{ user.plan }}",
							},
						},
					},
				},
			},
		}

		result, err := RenderRuleSet(rs, data)
		require.NoError(t, err)
		innerChildren := result.Children[0].Children
		assert.Equal(t, "alice@example.com", innerChildren[0].Value)
		assert.Equal(t, "pro", innerChildren[1].Value)
	})

	t.Run("journey data variable in rule value", func(t *testing.T) {
		rs := rules.RuleSet{
			Rule: rules.Rule{
				UUID:     uuid.New(),
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupUser,
				Operator: rules.OperatorAnd,
				Children: []rules.Rule{
					{
						UUID:     uuid.New(),
						Type:     rules.RuleTypeString,
						Group:    rules.RuleGroupEvent,
						Path:     "product_id",
						Operator: rules.OperatorEquals,
						Value:    "{{ journey.entrance.product_id }}",
					},
				},
			},
		}

		result, err := RenderRuleSet(rs, data)
		require.NoError(t, err)
		assert.Equal(t, "prod-123", result.Children[0].Value)
	})

	t.Run("original rule set is not mutated", func(t *testing.T) {
		original := rules.RuleSet{
			Rule: rules.Rule{
				UUID:     uuid.New(),
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupUser,
				Operator: rules.OperatorAnd,
				Children: []rules.Rule{
					{
						UUID:     uuid.New(),
						Type:     rules.RuleTypeString,
						Group:    rules.RuleGroupUser,
						Path:     "name",
						Operator: rules.OperatorEquals,
						Value:    "{{ user.name }}",
					},
				},
			},
		}

		result, err := RenderRuleSet(original, data)
		require.NoError(t, err)

		// Result should be resolved
		assert.Equal(t, "Alice", result.Children[0].Value)
		// Original should still have the template
		assert.Equal(t, "{{ user.name }}", original.Children[0].Value)
	})

	t.Run("user match member conditions are resolved", func(t *testing.T) {
		rs := rules.RuleSet{
			Rule: rules.Rule{
				UUID:     uuid.New(),
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupOrganizationEvent,
				Operator: rules.OperatorAnd,
				Value:    "org_event",
				UserMatch: &rules.UserMatch{
					Type: rules.UserMatchConditions,
					MemberConditions: &rules.Rule{
						UUID:     uuid.New(),
						Type:     rules.RuleTypeWrapper,
						Group:    rules.RuleGroupOrganizationUser,
						Operator: rules.OperatorAnd,
						Children: []rules.Rule{
							{
								UUID:     uuid.New(),
								Type:     rules.RuleTypeString,
								Group:    rules.RuleGroupOrganizationUser,
								Path:     "email",
								Operator: rules.OperatorEquals,
								Value:    "{{ user.email }}",
							},
						},
					},
				},
			},
		}

		result, err := RenderRuleSet(rs, data)
		require.NoError(t, err)

		require.NotNil(t, result.UserMatch)
		require.NotNil(t, result.UserMatch.MemberConditions)
		assert.Equal(t, "alice@example.com", result.UserMatch.MemberConditions.Children[0].Value)
		// Wrapper value should be unchanged
		assert.Equal(t, "org_event", result.Value)
	})

	t.Run("nil data renders empty", func(t *testing.T) {
		rs := rules.RuleSet{
			Rule: rules.Rule{
				UUID:     uuid.New(),
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupUser,
				Operator: rules.OperatorAnd,
				Children: []rules.Rule{
					{
						UUID:     uuid.New(),
						Type:     rules.RuleTypeString,
						Group:    rules.RuleGroupUser,
						Path:     "name",
						Operator: rules.OperatorEquals,
						Value:    "{{ user.name }}",
					},
				},
			},
		}

		result, err := RenderRuleSet(rs, map[string]any{})
		require.NoError(t, err)
		assert.Equal(t, "", result.Children[0].Value)
	})
}
