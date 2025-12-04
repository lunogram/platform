package query

import (
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/rules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testProjectID = uuid.New()

func TestQueryBuilderBuild(t *testing.T) {
	type test struct {
		name     string
		ruleSet  *rules.RuleSet
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}

	tests := map[string]test{
		"simple user rule": {
			name: "simple user rule",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".email",
					Operator: rules.OperatorEquals,
					Value:    "user@example.com",
				},
			},
			wantSQL:  "SELECT u.* FROM users u WHERE u.project_id = $2 AND u.email = $1",
			wantArgs: []any{"user@example.com", testProjectID},
			wantErr:  false,
		},
		"complex AND rule": {
			name: "complex AND rule",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".email",
							Operator: rules.OperatorEquals,
							Value:    "user@example.com",
						},
						{
							Type:     rules.RuleTypeNumber,
							Group:    rules.RuleGroupUser,
							Path:     ".age",
							Operator: rules.OperatorGreaterThan,
							Value:    18,
						},
					},
				},
			},
			wantSQL:  "SELECT u.* FROM users u WHERE u.project_id = $3 AND (u.email = $1 AND u.age > $2)",
			wantArgs: []any{"user@example.com", 18, testProjectID},
			wantErr:  false,
		},
		"event rule": {
			name: "event rule",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "purchase",
				},
			},
			wantSQL:  "SELECT u.* FROM users u WHERE u.project_id = $3 AND EXISTS (SELECT 1 FROM user_events e WHERE e.user_id = u.id AND e.project_id = $2 AND e.name = $1)",
			wantArgs: []any{"purchase", testProjectID, testProjectID},
			wantErr:  false,
		},
		"frequency rule": {
			name: "frequency rule",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "login",
					Frequency: &rules.Frequency{
						Count:    3,
						Operator: rules.OperatorGreaterThan,
						Period: rules.Period{
							Type:  rules.PeriodTypeRolling,
							Unit:  rules.PeriodUnitDay,
							Value: 7,
						},
					},
				},
			},
			wantSQL:  "SELECT u.* FROM users u WHERE u.project_id = $5 AND (SELECT COUNT(*) FROM user_events e WHERE e.user_id = u.id AND e.project_id = $2 AND e.created_at >= NOW() - $1::interval AND e.name = $3) > $4",
			wantArgs: []any{"7 days", testProjectID, "login", 3, testProjectID},
			wantErr:  false,
		},
		"empty wrapper returns all users": {
			name: "empty wrapper returns all users",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{},
				},
			},
			wantSQL:  "SELECT u.* FROM users u WHERE u.project_id = $1",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			qb := NewQueryBuilder(testProjectID)
			result, err := qb.Build(tc.ruleSet)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantSQL, result.SQL)
			assert.Equal(t, tc.wantArgs, result.Args)
		})
	}
}

func TestQueryBuilderBuildCondition(t *testing.T) {
	type test struct {
		name     string
		ruleSet  *rules.RuleSet
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}

	tests := map[string]test{
		"simple equals": {
			name: "simple equals",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".email",
					Operator: rules.OperatorEquals,
					Value:    "user@example.com",
				},
			},
			wantSQL:  "u.project_id = $2 AND u.email = $1",
			wantArgs: []any{"user@example.com", testProjectID},
			wantErr:  false,
		},
		"not equals": {
			name: "not equals",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".status",
					Operator: rules.OperatorNotEquals,
					Value:    "inactive",
				},
			},
			wantSQL:  "u.project_id = $2 AND u.status != $1",
			wantArgs: []any{"inactive", testProjectID},
			wantErr:  false,
		},
		"contains": {
			name: "contains",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".name",
					Operator: rules.OperatorContains,
					Value:    "john",
				},
			},
			wantSQL:  "u.project_id = $2 AND u.name ILIKE $1",
			wantArgs: []any{"%john%", testProjectID},
			wantErr:  false,
		},
		"starts with": {
			name: "starts with",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".email",
					Operator: rules.OperatorStartsWith,
					Value:    "admin",
				},
			},
			wantSQL:  "u.project_id = $2 AND u.email ILIKE $1",
			wantArgs: []any{"admin%", testProjectID},
			wantErr:  false,
		},
		"ends with": {
			name: "ends with",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".email",
					Operator: rules.OperatorEndsWith,
					Value:    "@example.com",
				},
			},
			wantSQL:  "u.project_id = $2 AND u.email ILIKE $1",
			wantArgs: []any{"%@example.com", testProjectID},
			wantErr:  false,
		},
		"greater than": {
			name: "greater than",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeNumber,
					Group:    rules.RuleGroupUser,
					Path:     ".age",
					Operator: rules.OperatorGreaterThan,
					Value:    18,
				},
			},
			wantSQL:  "u.project_id = $2 AND u.age > $1",
			wantArgs: []any{18, testProjectID},
			wantErr:  false,
		},
		"in operator": {
			name: "in operator",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".country",
					Operator: rules.OperatorAny,
					Value:    []string{"US", "UK", "CA"},
				},
			},
			wantSQL:  "u.project_id = $2 AND u.country = ANY($1)",
			wantArgs: []any{[]string{"US", "UK", "CA"}, testProjectID},
			wantErr:  false,
		},
		"not in operator": {
			name: "not in operator",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".status",
					Operator: rules.OperatorNone,
					Value:    []string{"banned", "deleted"},
				},
			},
			wantSQL:  "u.project_id = $2 AND u.status != ALL($1)",
			wantArgs: []any{[]string{"banned", "deleted"}, testProjectID},
			wantErr:  false,
		},
		"is set operator": {
			name: "is set operator",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".phone",
					Operator: rules.OperatorIsSet,
				},
			},
			wantSQL:  "u.project_id = $1 AND u.phone IS NOT NULL",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
		"is not set operator": {
			name: "is not set operator",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".deleted_at",
					Operator: rules.OperatorIsNotSet,
				},
			},
			wantSQL:  "u.project_id = $1 AND u.deleted_at IS NULL",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
		"empty operator for string": {
			name: "empty operator for string",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".name",
					Operator: rules.OperatorEmpty,
				},
			},
			wantSQL:  "u.project_id = $1 AND (u.name IS NULL OR u.name = '')",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
		"empty operator for array": {
			name: "empty operator for array",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeArray,
					Group:    rules.RuleGroupUser,
					Path:     ".tags",
					Operator: rules.OperatorEmpty,
				},
			},
			wantSQL:  "u.project_id = $1 AND (u.tags IS NULL OR array_length(u.tags, 1) IS NULL)",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
		"not contain operator": {
			name: "not contain operator",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".email",
					Operator: rules.OperatorNotContain,
					Value:    "spam",
				},
			},
			wantSQL:  "u.project_id = $2 AND u.email NOT ILIKE $1",
			wantArgs: []any{"%spam%", testProjectID},
			wantErr:  false,
		},
		"not start with operator": {
			name: "not start with operator",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".email",
					Operator: rules.OperatorNotStartWith,
					Value:    "test",
				},
			},
			wantSQL:  "u.project_id = $2 AND u.email NOT ILIKE $1",
			wantArgs: []any{"test%", testProjectID},
			wantErr:  false,
		},
		"is same day operator": {
			name: "is same day operator",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeDate,
					Group:    rules.RuleGroupUser,
					Path:     ".created_at",
					Operator: rules.OperatorIsSameDay,
					Value:    "2024-01-15",
				},
			},
			wantSQL:  "u.project_id = $2 AND DATE(u.created_at) = DATE($1)",
			wantArgs: []any{"2024-01-15", testProjectID},
			wantErr:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			qb := NewQueryBuilder(testProjectID)
			result, err := qb.BuildCondition(tc.ruleSet)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantSQL, result.SQL)
			assert.Equal(t, tc.wantArgs, result.Args)
		})
	}
}

func TestQueryBuilderBuildWrapper(t *testing.T) {
	type test struct {
		name     string
		ruleSet  *rules.RuleSet
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}

	tests := map[string]test{
		"and with two conditions": {
			name: "and with two conditions",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".email",
							Operator: rules.OperatorEquals,
							Value:    "user@example.com",
						},
						{
							Type:     rules.RuleTypeNumber,
							Group:    rules.RuleGroupUser,
							Path:     ".age",
							Operator: rules.OperatorGreaterThan,
							Value:    18,
						},
					},
				},
			},
			wantSQL:  "u.project_id = $3 AND (u.email = $1 AND u.age > $2)",
			wantArgs: []any{"user@example.com", 18, testProjectID},
			wantErr:  false,
		},
		"or with two conditions": {
			name: "or with two conditions",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorOr,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".status",
							Operator: rules.OperatorEquals,
							Value:    "active",
						},
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".status",
							Operator: rules.OperatorEquals,
							Value:    "pending",
						},
					},
				},
			},
			wantSQL:  "u.project_id = $3 AND (u.status = $1 OR u.status = $2)",
			wantArgs: []any{"active", "pending", testProjectID},
			wantErr:  false,
		},
		"nested and/or": {
			name: "nested and/or",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".country",
							Operator: rules.OperatorEquals,
							Value:    "US",
						},
						{
							Type:     rules.RuleTypeWrapper,
							Group:    rules.RuleGroupParent,
							Operator: rules.OperatorOr,
							Children: []rules.Rule{
								{
									Type:     rules.RuleTypeNumber,
									Group:    rules.RuleGroupUser,
									Path:     ".age",
									Operator: rules.OperatorGreaterThan,
									Value:    18,
								},
								{
									Type:     rules.RuleTypeBoolean,
									Group:    rules.RuleGroupUser,
									Path:     ".verified",
									Operator: rules.OperatorEquals,
									Value:    true,
								},
							},
						},
					},
				},
			},
			wantSQL:  "u.project_id = $4 AND (u.country = $1 AND (u.age > $2 OR u.verified = $3))",
			wantArgs: []any{"US", 18, true, testProjectID},
			wantErr:  false,
		},
		"single child unwraps": {
			name: "single child unwraps",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".email",
							Operator: rules.OperatorEquals,
							Value:    "user@example.com",
						},
					},
				},
			},
			wantSQL:  "u.project_id = $2 AND u.email = $1",
			wantArgs: []any{"user@example.com", testProjectID},
			wantErr:  false,
		},
		"empty wrapper": {
			name: "empty wrapper",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{},
				},
			},
			wantSQL:  "u.project_id = $1",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			qb := NewQueryBuilder(testProjectID)
			result, err := qb.BuildCondition(tc.ruleSet)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantSQL, result.SQL)
			assert.Equal(t, tc.wantArgs, result.Args)
		})
	}
}

func TestQueryBuilderBuildEventRule(t *testing.T) {
	type test struct {
		name     string
		ruleSet  *rules.RuleSet
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}

	tests := map[string]test{
		"simple event exists": {
			name: "simple event exists",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "purchase",
				},
			},
			wantSQL:  "u.project_id = $3 AND EXISTS (SELECT 1 FROM user_events e WHERE e.user_id = u.id AND e.project_id = $2 AND e.name = $1)",
			wantArgs: []any{"purchase", testProjectID, testProjectID},
			wantErr:  false,
		},
		"event with attribute": {
			name: "event with attribute",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "purchase",
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeNumber,
							Group:    rules.RuleGroupEvent,
							Path:     ".amount",
							Operator: rules.OperatorGreaterThan,
							Value:    100,
						},
					},
				},
			},
			wantSQL:  "u.project_id = $4 AND EXISTS (SELECT 1 FROM user_events e WHERE e.user_id = u.id AND e.project_id = $3 AND e.name = $1 AND e.amount > $2)",
			wantArgs: []any{"purchase", 100, testProjectID, testProjectID},
			wantErr:  false,
		},
		"event with nested jsonb attribute": {
			name: "event with nested jsonb attribute",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "purchase",
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupEvent,
							Path:     ".data.product.category",
							Operator: rules.OperatorEquals,
							Value:    "electronics",
						},
					},
				},
			},
			wantSQL:  "u.project_id = $4 AND EXISTS (SELECT 1 FROM user_events e WHERE e.user_id = u.id AND e.project_id = $3 AND e.name = $1 AND (e.data->'product'->>'category')::text = $2)",
			wantArgs: []any{"purchase", "electronics", testProjectID, testProjectID},
			wantErr:  false,
		},
		"event with deeply nested jsonb path": {
			name: "event with deeply nested jsonb path",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "checkout",
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupEvent,
							Path:     ".data.cart.items.sku",
							Operator: rules.OperatorEquals,
							Value:    "PROD-12345",
						},
					},
				},
			},
			wantSQL:  "u.project_id = $4 AND EXISTS (SELECT 1 FROM user_events e WHERE e.user_id = u.id AND e.project_id = $3 AND e.name = $1 AND (e.data->'cart'->'items'->>'sku')::text = $2)",
			wantArgs: []any{"checkout", "PROD-12345", testProjectID, testProjectID},
			wantErr:  false,
		},
		"event with numeric jsonb attribute": {
			name: "event with numeric jsonb attribute",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "purchase",
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeNumber,
							Group:    rules.RuleGroupEvent,
							Path:     ".data.order.total",
							Operator: rules.OperatorGreaterThan,
							Value:    500,
						},
					},
				},
			},
			wantSQL:  "u.project_id = $4 AND EXISTS (SELECT 1 FROM user_events e WHERE e.user_id = u.id AND e.project_id = $3 AND e.name = $1 AND (e.data->'order'->>'total')::numeric > $2)",
			wantArgs: []any{"purchase", 500, testProjectID, testProjectID},
			wantErr:  false,
		},
		"event with bracket notation jsonb path": {
			name: "event with bracket notation jsonb path",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "form_submit",
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupEvent,
							Path:     `.data["form fields"]["email address"]`,
							Operator: rules.OperatorEndsWith,
							Value:    "@company.com",
						},
					},
				},
			},
			wantSQL:  "u.project_id = $4 AND EXISTS (SELECT 1 FROM user_events e WHERE e.user_id = u.id AND e.project_id = $3 AND e.name = $1 AND (e.data->'form fields'->>'email address')::text ILIKE $2)",
			wantArgs: []any{"form_submit", "%@company.com", testProjectID, testProjectID},
			wantErr:  false,
		},
		"event with multiple nested jsonb conditions": {
			name: "event with multiple nested jsonb conditions",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "purchase",
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupEvent,
							Path:     ".data.product.category",
							Operator: rules.OperatorEquals,
							Value:    "electronics",
						},
						{
							Type:     rules.RuleTypeNumber,
							Group:    rules.RuleGroupEvent,
							Path:     ".data.product.price",
							Operator: rules.OperatorGreaterEqual,
							Value:    1000,
						},
					},
				},
			},
			wantSQL:  "u.project_id = $5 AND EXISTS (SELECT 1 FROM user_events e WHERE e.user_id = u.id AND e.project_id = $4 AND e.name = $1 AND (e.data->'product'->>'category')::text = $2 AND (e.data->'product'->>'price')::numeric >= $3)",
			wantArgs: []any{"purchase", "electronics", 1000, testProjectID, testProjectID},
			wantErr:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			qb := NewQueryBuilder(testProjectID)
			result, err := qb.BuildCondition(tc.ruleSet)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantSQL, result.SQL)
			assert.Equal(t, tc.wantArgs, result.Args)
		})
	}
}

func TestQueryBuilderBuildFrequencyRule(t *testing.T) {
	type test struct {
		name     string
		ruleSet  *rules.RuleSet
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}

	tests := map[string]test{
		"event count in last 7 days": {
			name: "event count in last 7 days",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "login",
					Frequency: &rules.Frequency{
						Count:    3,
						Operator: rules.OperatorGreaterThan,
						Period: rules.Period{
							Type:  rules.PeriodTypeRolling,
							Unit:  rules.PeriodUnitDay,
							Value: 7,
						},
					},
				},
			},
			wantSQL:  "u.project_id = $5 AND (SELECT COUNT(*) FROM user_events e WHERE e.user_id = u.id AND e.project_id = $2 AND e.created_at >= NOW() - $1::interval AND e.name = $3) > $4",
			wantArgs: []any{"7 days", testProjectID, "login", 3, testProjectID},
			wantErr:  false,
		},
		"event count with attribute in last 30 days": {
			name: "event count with attribute in last 30 days",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "purchase",
					Frequency: &rules.Frequency{
						Count:    5,
						Operator: rules.OperatorGreaterEqual,
						Period: rules.Period{
							Type:  rules.PeriodTypeRolling,
							Unit:  rules.PeriodUnitDay,
							Value: 30,
						},
					},
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupEvent,
							Path:     ".product_category",
							Operator: rules.OperatorEquals,
							Value:    "electronics",
						},
					},
				},
			},
			wantSQL:  "u.project_id = $6 AND (SELECT COUNT(*) FROM user_events e WHERE e.user_id = u.id AND e.project_id = $2 AND e.created_at >= NOW() - $1::interval AND e.name = $3 AND e.product_category = $4) >= $5",
			wantArgs: []any{"30 days", testProjectID, "purchase", "electronics", 5, testProjectID},
			wantErr:  false,
		},
		"event count equals zero in last hour": {
			name: "event count equals zero in last hour",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "error",
					Frequency: &rules.Frequency{
						Count:    0,
						Operator: rules.OperatorEquals,
						Period: rules.Period{
							Type:  rules.PeriodTypeRolling,
							Unit:  rules.PeriodUnitHour,
							Value: 1,
						},
					},
				},
			},
			wantSQL:  "u.project_id = $5 AND (SELECT COUNT(*) FROM user_events e WHERE e.user_id = u.id AND e.project_id = $2 AND e.created_at >= NOW() - $1::interval AND e.name = $3) = $4",
			wantArgs: []any{"1 hours", testProjectID, "error", 0, testProjectID},
			wantErr:  false,
		},
		"event count with nested jsonb attribute": {
			name: "event count with nested jsonb attribute",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "purchase",
					Frequency: &rules.Frequency{
						Count:    2,
						Operator: rules.OperatorGreaterEqual,
						Period: rules.Period{
							Type:  rules.PeriodTypeRolling,
							Unit:  rules.PeriodUnitWeek,
							Value: 1,
						},
					},
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupEvent,
							Path:     ".data.payment.method",
							Operator: rules.OperatorEquals,
							Value:    "credit_card",
						},
					},
				},
			},
			wantSQL:  "u.project_id = $6 AND (SELECT COUNT(*) FROM user_events e WHERE e.user_id = u.id AND e.project_id = $2 AND e.created_at >= NOW() - $1::interval AND e.name = $3 AND (e.data->'payment'->>'method')::text = $4) >= $5",
			wantArgs: []any{"1 weeks", testProjectID, "purchase", "credit_card", 2, testProjectID},
			wantErr:  false,
		},
		"event count with deeply nested jsonb": {
			name: "event count with deeply nested jsonb",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "transaction",
					Frequency: &rules.Frequency{
						Count:    5,
						Operator: rules.OperatorGreaterThan,
						Period: rules.Period{
							Type:  rules.PeriodTypeRolling,
							Unit:  rules.PeriodUnitMonth,
							Value: 1,
						},
					},
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupEvent,
							Path:     ".data.transaction.status.code",
							Operator: rules.OperatorEquals,
							Value:    "success",
						},
					},
				},
			},
			wantSQL:  "u.project_id = $6 AND (SELECT COUNT(*) FROM user_events e WHERE e.user_id = u.id AND e.project_id = $2 AND e.created_at >= NOW() - $1::interval AND e.name = $3 AND (e.data->'transaction'->'status'->>'code')::text = $4) > $5",
			wantArgs: []any{"1 months", testProjectID, "transaction", "success", 5, testProjectID},
			wantErr:  false,
		},
		"event count with numeric nested jsonb": {
			name: "event count with numeric nested jsonb",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "api_call",
					Frequency: &rules.Frequency{
						Count:    100,
						Operator: rules.OperatorGreaterEqual,
						Period: rules.Period{
							Type:  rules.PeriodTypeRolling,
							Unit:  rules.PeriodUnitDay,
							Value: 1,
						},
					},
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeNumber,
							Group:    rules.RuleGroupEvent,
							Path:     ".data.response.status_code",
							Operator: rules.OperatorEquals,
							Value:    200,
						},
					},
				},
			},
			wantSQL:  "u.project_id = $6 AND (SELECT COUNT(*) FROM user_events e WHERE e.user_id = u.id AND e.project_id = $2 AND e.created_at >= NOW() - $1::interval AND e.name = $3 AND (e.data->'response'->>'status_code')::numeric = $4) >= $5",
			wantArgs: []any{"1 days", testProjectID, "api_call", 200, 100, testProjectID},
			wantErr:  false,
		},
		"event count with multiple nested jsonb conditions": {
			name: "event count with multiple nested jsonb conditions",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "checkout",
					Frequency: &rules.Frequency{
						Count:    3,
						Operator: rules.OperatorGreaterEqual,
						Period: rules.Period{
							Type:  rules.PeriodTypeRolling,
							Unit:  rules.PeriodUnitWeek,
							Value: 2,
						},
					},
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupEvent,
							Path:     ".data.cart.currency",
							Operator: rules.OperatorEquals,
							Value:    "USD",
						},
						{
							Type:     rules.RuleTypeNumber,
							Group:    rules.RuleGroupEvent,
							Path:     ".data.cart.total",
							Operator: rules.OperatorGreaterThan,
							Value:    250,
						},
					},
				},
			},
			wantSQL:  "u.project_id = $7 AND (SELECT COUNT(*) FROM user_events e WHERE e.user_id = u.id AND e.project_id = $2 AND e.created_at >= NOW() - $1::interval AND e.name = $3 AND (e.data->'cart'->>'currency')::text = $4 AND (e.data->'cart'->>'total')::numeric > $5) >= $6",
			wantArgs: []any{"2 weeks", testProjectID, "checkout", "USD", 250, 3, testProjectID},
			wantErr:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			qb := NewQueryBuilder(testProjectID)
			result, err := qb.BuildCondition(tc.ruleSet)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantSQL, result.SQL)
			assert.Equal(t, tc.wantArgs, result.Args)
		})
	}
}

func TestQueryBuilderComplexRules(t *testing.T) {
	type test struct {
		name     string
		ruleSet  *rules.RuleSet
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}

	tests := map[string]test{
		"complex multi-level rule": {
			name: "complex multi-level rule",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".country",
							Operator: rules.OperatorAny,
							Value:    []string{"US", "CA", "UK"},
						},
						{
							Type:     rules.RuleTypeWrapper,
							Group:    rules.RuleGroupParent,
							Operator: rules.OperatorOr,
							Children: []rules.Rule{
								{
									Type:     rules.RuleTypeNumber,
									Group:    rules.RuleGroupUser,
									Path:     ".age",
									Operator: rules.OperatorGreaterEqual,
									Value:    18,
								},
								{
									Type:     rules.RuleTypeBoolean,
									Group:    rules.RuleGroupUser,
									Path:     ".parent_consent",
									Operator: rules.OperatorEquals,
									Value:    true,
								},
							},
						},
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupEvent,
							Path:     "",
							Operator: rules.OperatorEquals,
							Value:    "purchase",
							Frequency: &rules.Frequency{
								Count:    1,
								Operator: rules.OperatorGreaterEqual,
								Period: rules.Period{
									Type:  rules.PeriodTypeRolling,
									Unit:  rules.PeriodUnitDay,
									Value: 30,
								},
							},
						},
					},
				},
			},
			wantSQL:  "u.project_id = $8 AND (u.country = ANY($1) AND (u.age >= $2 OR u.parent_consent = $3) AND (SELECT COUNT(*) FROM user_events e WHERE e.user_id = u.id AND e.project_id = $5 AND e.created_at >= NOW() - $4::interval AND e.name = $6) >= $7)",
			wantArgs: []any{[]string{"US", "CA", "UK"}, 18, true, "30 days", testProjectID, "purchase", 1, testProjectID},
			wantErr:  false,
		},
		"user and event combined": {
			name: "user and event combined",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".email",
							Operator: rules.OperatorEndsWith,
							Value:    "@company.com",
						},
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupEvent,
							Path:     "",
							Operator: rules.OperatorEquals,
							Value:    "login",
							Children: []rules.Rule{
								{
									Type:     rules.RuleTypeString,
									Group:    rules.RuleGroupEvent,
									Path:     ".device",
									Operator: rules.OperatorEquals,
									Value:    "mobile",
								},
							},
						},
					},
				},
			},
			wantSQL:  "u.project_id = $5 AND (u.email ILIKE $1 AND EXISTS (SELECT 1 FROM user_events e WHERE e.user_id = u.id AND e.project_id = $4 AND e.name = $2 AND e.device = $3))",
			wantArgs: []any{"%@company.com", "login", "mobile", testProjectID, testProjectID},
			wantErr:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			qb := NewQueryBuilder(testProjectID)
			result, err := qb.BuildCondition(tc.ruleSet)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantSQL, result.SQL)
			assert.Equal(t, tc.wantArgs, result.Args)
		})
	}
}

func TestQueryBuilderNestedJSONBPaths(t *testing.T) {
	type test struct {
		name     string
		ruleSet  *rules.RuleSet
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}

	tests := map[string]test{
		"single nested jsonb path": {
			name: "single nested jsonb path",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data.subscription",
					Operator: rules.OperatorEquals,
					Value:    "premium",
				},
			},
			wantSQL:  "u.project_id = $2 AND (u.data->>'subscription')::text = $1",
			wantArgs: []any{"premium", testProjectID},
			wantErr:  false,
		},
		"bracket notation with spaces": {
			name: "bracket notation with spaces",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data['purchase agreement']",
					Operator: rules.OperatorEquals,
					Value:    "signed",
				},
			},
			wantSQL:  "u.project_id = $2 AND (u.data->>'purchase agreement')::text = $1",
			wantArgs: []any{"signed", testProjectID},
			wantErr:  false,
		},
		"mixed bracket and dot notation": {
			name: "mixed bracket and dot notation",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data['purchase agreement'].value",
					Operator: rules.OperatorEquals,
					Value:    "active",
				},
			},
			wantSQL:  "u.project_id = $2 AND (u.data->'purchase agreement'->>'value')::text = $1",
			wantArgs: []any{"active", testProjectID},
			wantErr:  false,
		},
		"bracket notation with double quotes": {
			name: "bracket notation with double quotes",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     `.data["user info"].name`,
					Operator: rules.OperatorEquals,
					Value:    "John",
				},
			},
			wantSQL:  "u.project_id = $2 AND (u.data->'user info'->>'name')::text = $1",
			wantArgs: []any{"John", testProjectID},
			wantErr:  false,
		},
		"complex path with special characters": {
			name: "complex path with special characters",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data['customer-id'].verified",
					Operator: rules.OperatorEquals,
					Value:    true,
				},
			},
			wantSQL:  "u.project_id = $2 AND (u.data->'customer-id'->>'verified')::text = $1",
			wantArgs: []any{true, testProjectID},
			wantErr:  false,
		},
		"deeply nested with brackets": {
			name: "deeply nested with brackets",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeNumber,
					Group:    rules.RuleGroupUser,
					Path:     ".data['billing info']['payment method'].priority",
					Operator: rules.OperatorGreaterThan,
					Value:    5,
				},
			},
			wantSQL:  "u.project_id = $2 AND (u.data->'billing info'->'payment method'->>'priority')::numeric > $1",
			wantArgs: []any{5, testProjectID},
			wantErr:  false,
		},
		"double nested jsonb path": {
			name: "double nested jsonb path",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data.subscription.tier",
					Operator: rules.OperatorEquals,
					Value:    "gold",
				},
			},
			wantSQL:  "u.project_id = $2 AND (u.data->'subscription'->>'tier')::text = $1",
			wantArgs: []any{"gold", testProjectID},
			wantErr:  false,
		},
		"triple nested jsonb path": {
			name: "triple nested jsonb path",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data.preferences.notifications.email",
					Operator: rules.OperatorEquals,
					Value:    true,
				},
			},
			wantSQL:  "u.project_id = $2 AND (u.data->'preferences'->'notifications'->>'email')::text = $1",
			wantArgs: []any{true, testProjectID},
			wantErr:  false,
		},
		"nested jsonb with text extraction": {
			name: "nested jsonb with text extraction",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data.company.name",
					Operator: rules.OperatorContains,
					Value:    "Tech",
				},
			},
			wantSQL:  "u.project_id = $2 AND (u.data->'company'->>'name')::text ILIKE $1",
			wantArgs: []any{"%Tech%", testProjectID},
			wantErr:  false,
		},
		"nested jsonb number comparison": {
			name: "nested jsonb number comparison",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeNumber,
					Group:    rules.RuleGroupUser,
					Path:     ".data.metrics.score",
					Operator: rules.OperatorGreaterThan,
					Value:    100,
				},
			},
			wantSQL:  "u.project_id = $2 AND (u.data->'metrics'->>'score')::numeric > $1",
			wantArgs: []any{100, testProjectID},
			wantErr:  false,
		},
		"nested jsonb is set": {
			name: "nested jsonb is set",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data.settings.theme",
					Operator: rules.OperatorIsSet,
				},
			},
			wantSQL:  "u.project_id = $1 AND (u.data->'settings'->>'theme')::text IS NOT NULL",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
		"nested jsonb is not set": {
			name: "nested jsonb is not set",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data.billing.card",
					Operator: rules.OperatorIsNotSet,
				},
			},
			wantSQL:  "u.project_id = $1 AND (u.data->'billing'->>'card')::text IS NULL",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
		"nested jsonb in complex wrapper": {
			name: "nested jsonb in complex wrapper",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".data.subscription.plan",
							Operator: rules.OperatorEquals,
							Value:    "enterprise",
						},
						{
							Type:     rules.RuleTypeNumber,
							Group:    rules.RuleGroupUser,
							Path:     ".data.subscription.seats",
							Operator: rules.OperatorGreaterEqual,
							Value:    10,
						},
					},
				},
			},
			wantSQL:  "u.project_id = $3 AND ((u.data->'subscription'->>'plan')::text = $1 AND (u.data->'subscription'->>'seats')::numeric >= $2)",
			wantArgs: []any{"enterprise", 10, testProjectID},
			wantErr:  false,
		},
		"mixed regular and nested jsonb fields": {
			name: "mixed regular and nested jsonb fields",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".email",
							Operator: rules.OperatorEndsWith,
							Value:    "@enterprise.com",
						},
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".data.company.verified",
							Operator: rules.OperatorEquals,
							Value:    true,
						},
					},
				},
			},
			wantSQL:  "u.project_id = $3 AND (u.email ILIKE $1 AND (u.data->'company'->>'verified')::text = $2)",
			wantArgs: []any{"%@enterprise.com", true, testProjectID},
			wantErr:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			qb := NewQueryBuilder(testProjectID)
			result, err := qb.BuildCondition(tc.ruleSet)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantSQL, result.SQL)
			assert.Equal(t, tc.wantArgs, result.Args)
		})
	}
}

func TestQueryBuilderErrorCases(t *testing.T) {
	type test struct {
		name    string
		ruleSet *rules.RuleSet
		wantErr bool
	}

	tests := map[string]test{
		"unsupported rule group": {
			name: "unsupported rule group",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroup("invalid"),
					Path:     ".field",
					Operator: rules.OperatorEquals,
					Value:    "value",
				},
			},
			wantErr: true,
		},
		"unsupported period type": {
			name: "unsupported period type",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "event",
					Frequency: &rules.Frequency{
						Count:    1,
						Operator: rules.OperatorGreaterThan,
						Period: rules.Period{
							Type:  rules.PeriodTypeAbsolute,
							Unit:  rules.PeriodUnitDay,
							Value: 1,
						},
					},
				},
			},
			wantErr: true,
		},
		"bracket notation with single quote in key": {
			name: "bracket notation with single quote in key",
			ruleSet: &rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     `.data["user's preference"].theme`,
					Operator: rules.OperatorEquals,
					Value:    "dark",
				},
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			qb := NewQueryBuilder(testProjectID)
			_, err := qb.Build(tc.ruleSet)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOperatorSQL(t *testing.T) {
	type test struct {
		op   rules.Operator
		want string
	}

	tests := map[string]test{
		"and operator": {
			op:   rules.OperatorAnd,
			want: "AND",
		},
		"or operator": {
			op:   rules.OperatorOr,
			want: "OR",
		},
		"comparison operator defaults to AND": {
			op:   rules.OperatorEquals,
			want: "AND",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.op.SQL())
		})
	}
}

func TestPeriodUnitPostgresInterval(t *testing.T) {
	type test struct {
		unit rules.PeriodUnit
		want string
	}

	tests := map[string]test{
		"minutes": {
			unit: rules.PeriodUnitMinute,
			want: "minutes",
		},
		"hours": {
			unit: rules.PeriodUnitHour,
			want: "hours",
		},
		"days": {
			unit: rules.PeriodUnitDay,
			want: "days",
		},
		"weeks": {
			unit: rules.PeriodUnitWeek,
			want: "weeks",
		},
		"months": {
			unit: rules.PeriodUnitMonth,
			want: "months",
		},
		"years": {
			unit: rules.PeriodUnitYear,
			want: "years",
		},
		"invalid defaults to days": {
			unit: rules.PeriodUnit("invalid"),
			want: "days",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.unit.SQL())
		})
	}
}

func TestRuleIsWrapper(t *testing.T) {
	type test struct {
		rule rules.Rule
		want bool
	}

	tests := map[string]test{
		"wrapper type": {
			rule: rules.Rule{Type: rules.RuleTypeWrapper},
			want: true,
		},
		"string type": {
			rule: rules.Rule{Type: rules.RuleTypeString},
			want: false,
		},
		"number type": {
			rule: rules.Rule{Type: rules.RuleTypeNumber},
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.rule.IsWrapper())
		})
	}
}

func TestRuleHasChildren(t *testing.T) {
	type test struct {
		rule rules.Rule
		want bool
	}

	tests := map[string]test{
		"with children": {
			rule: rules.Rule{
				Children: []rules.Rule{
					{Type: rules.RuleTypeString},
				},
			},
			want: true,
		},
		"without children": {
			rule: rules.Rule{
				Children: []rules.Rule{},
			},
			want: false,
		},
		"nil children": {
			rule: rules.Rule{},
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.rule.HasChildren())
		})
	}
}

func TestRuleIsRoot(t *testing.T) {
	parentUUID := uuid.New()

	type test struct {
		rule rules.Rule
		want bool
	}

	tests := map[string]test{
		"is root": {
			rule: rules.Rule{ParentUUID: nil},
			want: true,
		},
		"has parent": {
			rule: rules.Rule{ParentUUID: &parentUUID},
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.rule.IsRoot())
		})
	}
}
