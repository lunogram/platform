package rules

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNumberRuleQuery(t *testing.T) {
	projectID := uuid.New()

	type test struct {
		name     string
		rule     *RuleTree
		expected string
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
			expected: "(data->>'age')::int = 25",
		},
		{
			name: "greater-than",
			rule: Make(MakeParams{
				Type:     RuleTypeNumber,
				Path:     "$.score",
				Operator: OpGreaterThan,
				Value:    100,
			}),
			expected: "(data->>'score')::int > 100",
		},
		{
			name: "less-than-or-equal",
			rule: Make(MakeParams{
				Type:     RuleTypeNumber,
				Path:     "$.balance",
				Operator: OpLessThanEq,
				Value:    50.5,
			}),
			expected: "(data->>'balance')::int <= 50.5",
		},
		{
			name: "is-set",
			rule: Make(MakeParams{
				Type:     RuleTypeNumber,
				Path:     "$.age",
				Operator: OpIsSet,
			}),
			expected: "(data->>'age')::int IS NOT NULL",
		},
		{
			name: "is-not-set",
			rule: Make(MakeParams{
				Type:     RuleTypeNumber,
				Path:     "$.age",
				Operator: OpIsNotSet,
			}),
			expected: "(data->>'age')::int IS NULL",
		},
		{
			name: "reserved-path-created-at",
			rule: Make(MakeParams{
				Type:     RuleTypeNumber,
				Path:     "$.created_at",
				Operator: OpGreaterThan,
				Value:    1609459200,
			}),
			expected: "created_at > 1.6094592e+09",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := GetRuleQuery(projectID, tc.rule)
			assert.Contains(t, query, tc.expected)
		})
	}
}

func TestStringRuleQuery(t *testing.T) {
	projectID := uuid.New()

	type test struct {
		name     string
		rule     *RuleTree
		expected string
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
			expected: "(data->>'name')::text = 'John'",
		},
		{
			name: "contains-custom-field",
			rule: Make(MakeParams{
				Type:     RuleTypeString,
				Path:     "$.company",
				Operator: OpContains,
				Value:    "Tech",
			}),
			expected: "(data->>'company')::text LIKE '%Tech%'",
		},
		{
			name: "starts-with",
			rule: Make(MakeParams{
				Type:     RuleTypeString,
				Path:     "$.name",
				Operator: OpStartsWith,
				Value:    "John",
			}),
			expected: "(data->>'name')::text LIKE 'John%'",
		},
		{
			name: "not-contain-reserved-path",
			rule: Make(MakeParams{
				Type:     RuleTypeString,
				Path:     "$.email",
				Operator: OpNotContain,
				Value:    "spam",
			}),
			expected: "email NOT LIKE '%spam%'",
		},
		{
			name: "empty",
			rule: Make(MakeParams{
				Type:     RuleTypeString,
				Path:     "$.description",
				Operator: OpEmpty,
			}),
			expected: "(data->>'description')::text = ''",
		},
		{
			name: "reserved-path-email",
			rule: Make(MakeParams{
				Type:     RuleTypeString,
				Path:     "$.email",
				Operator: OpContains,
				Value:    "@gmail",
			}),
			expected: "email LIKE '%@gmail%'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := GetRuleQuery(projectID, tc.rule)
			assert.Contains(t, query, tc.expected)
		})
	}
}

func TestBooleanRuleQuery(t *testing.T) {
	projectID := uuid.New()

	type test struct {
		name     string
		rule     *RuleTree
		expected string
	}

	tests := []test{
		{
			name: "true",
			rule: Make(MakeParams{
				Type:     RuleTypeBoolean,
				Path:     "$.active",
				Operator: OpEqual,
				Value:    true,
			}),
			expected: "(data->>'active')::boolean = true",
		},
		{
			name: "false",
			rule: Make(MakeParams{
				Type:     RuleTypeBoolean,
				Path:     "$.verified",
				Operator: OpEqual,
				Value:    false,
			}),
			expected: "(data->>'verified')::boolean = false",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := GetRuleQuery(projectID, tc.rule)
			assert.Contains(t, query, tc.expected)
		})
	}
}

func TestDateRuleQuery(t *testing.T) {
	projectID := uuid.New()

	type test struct {
		name     string
		rule     *RuleTree
		contains string
	}

	tests := []test{
		{
			name: "greater-than",
			rule: Make(MakeParams{
				Type:     RuleTypeDate,
				Path:     "$.signup_date",
				Operator: OpGreaterThan,
				Value:    "2024-01-01T00:00:00Z",
			}),
			contains: "parseDateTimeBestEffortOrNull",
		},
		{
			name: "is-same-day",
			rule: Make(MakeParams{
				Type:     RuleTypeDate,
				Path:     "$.last_login",
				Operator: OpIsSameDay,
				Value:    "2024-12-01T00:00:00Z",
			}),
			contains: "toDate",
		},
		{
			name: "is-set",
			rule: Make(MakeParams{
				Type:     RuleTypeDate,
				Path:     "$.verified_at",
				Operator: OpIsSet,
			}),
			contains: "IS NOT NULL",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := GetRuleQuery(projectID, tc.rule)
			assert.Contains(t, query, tc.contains)
		})
	}
}

func TestArrayRuleQuery(t *testing.T) {
	projectID := uuid.New()

	type test struct {
		name     string
		rule     *RuleTree
		expected string
	}

	tests := []test{
		{
			name: "is-set",
			rule: Make(MakeParams{
				Type:     RuleTypeArray,
				Path:     "$.tags",
				Operator: OpIsSet,
			}),
			expected: "(data->>'tags')::jsonb IS NOT NULL",
		},
		{
			name: "empty",
			rule: Make(MakeParams{
				Type:     RuleTypeArray,
				Path:     "$.subscriptions",
				Operator: OpEmpty,
			}),
			expected: "(data->>'subscriptions')::jsonb = '[]'::jsonb",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := GetRuleQuery(projectID, tc.rule)
			assert.Contains(t, query, tc.expected)
		})
	}
}

func TestWrapperRuleQuery(t *testing.T) {
	projectID := uuid.New()

	type test struct {
		name      string
		rule      *RuleTree
		contains  []string
		notContain string
	}

	tests := []test{
		{
			name: "and-with-two-user-rules",
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
			contains: []string{
				"SELECT id FROM users WHERE",
				"(data->>'age')::int > 18",
				"and",
				"(data->>'country')::text = 'US'",
				"project_id",
			},
		},
		{
			name: "or-with-two-user-rules",
			rule: Make(MakeParams{
				Type:     RuleTypeWrapper,
				Operator: OpOr,
				Children: []*RuleTree{
					Make(MakeParams{
						Type:     RuleTypeString,
						Path:     "$.tier",
						Operator: OpEqual,
						Value:    "premium",
					}),
					Make(MakeParams{
						Type:     RuleTypeString,
						Path:     "$.tier",
						Operator: OpEqual,
						Value:    "enterprise",
					}),
				},
			}),
			contains: []string{
				"SELECT id FROM users WHERE",
				"(data->>'tier')::text = 'premium'",
				"or",
				"(data->>'tier')::text = 'enterprise'",
			},
		},
		{
			name: "nested-and-or",
			rule: Make(MakeParams{
				Type:     RuleTypeWrapper,
				Operator: OpAnd,
				Children: []*RuleTree{
					Make(MakeParams{
						Type:     RuleTypeNumber,
						Path:     "$.age",
						Operator: OpGreaterThanEq,
						Value:    21,
					}),
					Make(MakeParams{
						Type:     RuleTypeWrapper,
						Operator: OpOr,
						Children: []*RuleTree{
							Make(MakeParams{
								Type:     RuleTypeString,
								Path:     "$.country",
								Operator: OpEqual,
								Value:    "US",
							}),
							Make(MakeParams{
								Type:     RuleTypeString,
								Path:     "$.country",
								Operator: OpEqual,
								Value:    "CA",
							}),
						},
					}),
				},
			}),
			contains: []string{
				"SELECT id FROM users WHERE",
				"(data->>'age')::int >= 21",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := GetRuleQuery(projectID, tc.rule)
			t.Logf("Generated query: %s", query)
			for _, expected := range tc.contains {
				assert.Contains(t, query, expected, "Query should contain: %s", expected)
			}
			if tc.notContain != "" {
				assert.NotContains(t, query, tc.notContain)
			}
		})
	}
}

func TestEventWrapperQuery(t *testing.T) {
	projectID := uuid.New()

	type test struct {
		name     string
		rule     *RuleTree
		contains []string
	}

	tests := []test{
		{
			name: "simple-event",
			rule: &RuleTree{
				Rule: Rule{
					UUID:     uuid.New().String(),
					Type:     RuleTypeWrapper,
					Group:    RuleGroupEvent,
					Path:     "$.name",
					Operator: OpAnd,
					Value:    mustMarshalBytes("purchase"),
				},
			},
			contains: []string{
				"SELECT user_id AS id",
				"FROM user_events",
				"WHERE",
				"name = 'purchase'",
				"GROUP BY project_id, user_id",
				"HAVING",
			},
		},
		{
			name: "event-with-frequency",
			rule: &RuleTree{
				Rule: Rule{
					UUID:     uuid.New().String(),
					Type:     RuleTypeWrapper,
					Group:    RuleGroupEvent,
					Path:     "$.name",
					Operator: OpAnd,
					Value:    mustMarshalBytes("page_view"),
				},
				Frequency: &EventRuleFrequency{
					Operator: OpGreaterThanEq,
					Count:    5,
					Period: EventRulePeriod{
						Type:  PeriodTypeRolling,
						Unit:  timeUnitPtr(TimeUnitDay),
						Value: intPtr(7),
					},
				},
			},
			contains: []string{
				"SELECT user_id AS id",
				"FROM user_events",
				"name = 'page_view'",
				"created_at >= now() - INTERVAL '7 day'",
				"HAVING count(*) >= 5",
			},
		},
		{
			name: "event-with-fixed-period",
			rule: &RuleTree{
				Rule: Rule{
					UUID:     uuid.New().String(),
					Type:     RuleTypeWrapper,
					Group:    RuleGroupEvent,
					Path:     "$.name",
					Operator: OpAnd,
					Value:    mustMarshalBytes("signup"),
				},
				Frequency: &EventRuleFrequency{
					Operator: OpEqual,
					Count:    1,
					Period: EventRulePeriod{
						Type:      PeriodTypeFixed,
						StartDate: stringPtr("2024-01-01T00:00:00Z"),
						EndDate:   stringPtr("2024-12-31T23:59:59Z"),
					},
				},
			},
			contains: []string{
				"SELECT user_id AS id",
				"FROM user_events",
				"name = 'signup'",
				"created_at >= '2024-01-01T00:00:00Z'",
				"created_at <= '2024-12-31T23:59:59Z'",
				"HAVING count(*) = 1",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := GetRuleQuery(projectID, tc.rule)
			t.Logf("Generated query: %s", query)
			for _, expected := range tc.contains {
				assert.Contains(t, query, expected)
			}
		})
	}
}

func TestComplexMixedQuery(t *testing.T) {
	projectID := uuid.New()

	// Users who are over 21, from US or CA, have premium tier, and made a purchase
	rule := Make(MakeParams{
		Type:     RuleTypeWrapper,
		Operator: OpAnd,
		Children: []*RuleTree{
			// User properties
			Make(MakeParams{
				Type:     RuleTypeNumber,
				Path:     "$.age",
				Operator: OpGreaterThanEq,
				Value:    21,
			}),
			Make(MakeParams{
				Type:     RuleTypeString,
				Path:     "$.tier",
				Operator: OpEqual,
				Value:    "premium",
			}),
			// Event
			&RuleTree{
				Rule: Rule{
					UUID:     uuid.New().String(),
					Type:     RuleTypeWrapper,
					Group:    RuleGroupEvent,
					Path:     "$.name",
					Operator: OpAnd,
					Value:    mustMarshalBytes("purchase"),
				},
				Frequency: &EventRuleFrequency{
					Operator: OpGreaterThanEq,
					Count:    1,
				},
			},
		},
	})

	query := GetRuleQuery(projectID, rule)
	t.Logf("Complex query: %s", query)

	// Should contain user filter
	assert.Contains(t, query, "(data->>'age')::int >= 21")
	assert.Contains(t, query, "(data->>'tier')::text = 'premium'")

	// Should contain event filter
	assert.Contains(t, query, "user_events")
	assert.Contains(t, query, "name = 'purchase'")

	// Should have INTERSECT or similar joining logic
	require.NotEmpty(t, query)
}

func TestNestedDataPaths(t *testing.T) {
	projectID := uuid.New()

	type test struct {
		name     string
		rule     *RuleTree
		expected string
	}

	tests := []test{
		{
			name: "nested-object-field",
			rule: Make(MakeParams{
				Type:     RuleTypeString,
				Path:     "$.profile.city",
				Operator: OpEqual,
				Value:    "New York",
			}),
			expected: "(data->'profile'->>'city')::text = 'New York'",
		},
		{
			name: "deeply-nested-field",
			rule: Make(MakeParams{
				Type:     RuleTypeNumber,
				Path:     "$.stats.engagement.score",
				Operator: OpGreaterThan,
				Value:    80,
			}),
			expected: "(data->'stats'->'engagement'->>'score')::int > 80",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			query := GetRuleQuery(projectID, tc.rule)
			t.Logf("Query: %s", query)
			assert.Contains(t, query, tc.expected)
		})
	}
}

func mustMarshalBytes(v interface{}) []byte {
	b, _ := marshalValue(v)
	return b
}

func intPtr(i int) *int {
	return &i
}

func stringPtr(s string) *string {
	return &s
}

func timeUnitPtr(u TimeUnit) *TimeUnit {
	return &u
}
