package query

import (
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testProjectID = uuid.New()

func TestQueryBuilderBasicQueries(t *testing.T) {
	type test struct {
		name     string
		ruleSet  rules.RuleSet
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}

	tests := map[string]test{
		"simple equals": {
			name: "simple equals",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".email",
					Operator: rules.OperatorEquals,
					Value:    "user@example.com",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND u.email = $1",
			wantArgs: []any{"user@example.com", testProjectID},
			wantErr:  false,
		},
		"not equals": {
			name: "not equals",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".status",
					Operator: rules.OperatorNotEquals,
					Value:    "inactive",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND u.status != $1",
			wantArgs: []any{"inactive", testProjectID},
			wantErr:  false,
		},
		"contains": {
			name: "contains",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".name",
					Operator: rules.OperatorContains,
					Value:    "john",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND u.name ILIKE $1",
			wantArgs: []any{"%john%", testProjectID},
			wantErr:  false,
		},
		"starts with": {
			name: "starts with",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".email",
					Operator: rules.OperatorStartsWith,
					Value:    "admin",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND u.email ILIKE $1",
			wantArgs: []any{"admin%", testProjectID},
			wantErr:  false,
		},
		"ends with": {
			name: "ends with",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".email",
					Operator: rules.OperatorEndsWith,
					Value:    "@example.com",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND u.email ILIKE $1",
			wantArgs: []any{"%@example.com", testProjectID},
			wantErr:  false,
		},
		"greater than": {
			name: "greater than",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeNumber,
					Group:    rules.RuleGroupUser,
					Path:     ".age",
					Operator: rules.OperatorGreaterThan,
					Value:    18,
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND u.age > $1",
			wantArgs: []any{18, testProjectID},
			wantErr:  false,
		},
		"in operator": {
			name: "in operator",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".country",
					Operator: rules.OperatorAny,
					Value:    []string{"US", "UK", "CA"},
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND u.country = ANY($1)",
			wantArgs: []any{[]string{"US", "UK", "CA"}, testProjectID},
			wantErr:  false,
		},
		"not in operator": {
			name: "not in operator",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".status",
					Operator: rules.OperatorNone,
					Value:    []string{"banned", "deleted"},
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND u.status != ALL($1)",
			wantArgs: []any{[]string{"banned", "deleted"}, testProjectID},
			wantErr:  false,
		},
		"is set operator": {
			name: "is set operator",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".phone",
					Operator: rules.OperatorIsSet,
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $1 AND u.phone IS NOT NULL",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
		"is not set operator": {
			name: "is not set operator",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".deleted_at",
					Operator: rules.OperatorIsNotSet,
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $1 AND u.deleted_at IS NULL",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
		"empty operator for string": {
			name: "empty operator for string",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".name",
					Operator: rules.OperatorEmpty,
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $1 AND (u.name IS NULL OR u.name = '')",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
		"empty operator for array": {
			name: "empty operator for array",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeArray,
					Group:    rules.RuleGroupUser,
					Path:     ".tags",
					Operator: rules.OperatorEmpty,
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $1 AND (u.tags IS NULL OR array_length(u.tags, 1) IS NULL)",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
		"not contain operator": {
			name: "not contain operator",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".email",
					Operator: rules.OperatorNotContain,
					Value:    "spam",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND u.email NOT ILIKE $1",
			wantArgs: []any{"%spam%", testProjectID},
			wantErr:  false,
		},
		"not start with operator": {
			name: "not start with operator",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".email",
					Operator: rules.OperatorNotStartWith,
					Value:    "test",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND u.email NOT ILIKE $1",
			wantArgs: []any{"test%", testProjectID},
			wantErr:  false,
		},
		"is same day operator": {
			name: "is same day operator",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeDate,
					Group:    rules.RuleGroupUser,
					Path:     ".created_at",
					Operator: rules.OperatorIsSameDay,
					Value:    "2024-01-15",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND DATE(u.created_at) = DATE($1)",
			wantArgs: []any{"2024-01-15", testProjectID},
			wantErr:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			qb := NewQueryBuilder(testProjectID, nil)
			result, err := qb.Query(tc.ruleSet)

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
		ruleSet  rules.RuleSet
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}

	tests := map[string]test{
		"and with two conditions": {
			name: "and with two conditions",
			ruleSet: rules.RuleSet{
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
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $3 AND (u.email = $1 AND u.age > $2)",
			wantArgs: []any{"user@example.com", 18, testProjectID},
			wantErr:  false,
		},
		"or with two conditions": {
			name: "or with two conditions",
			ruleSet: rules.RuleSet{
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
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $3 AND (u.status = $1 OR u.status = $2)",
			wantArgs: []any{"active", "pending", testProjectID},
			wantErr:  false,
		},
		"nested and/or": {
			name: "nested and/or",
			ruleSet: rules.RuleSet{
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
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $4 AND (u.country = $1 AND (u.age > $2 OR u.verified = $3))",
			wantArgs: []any{"US", 18, true, testProjectID},
			wantErr:  false,
		},
		"single child unwraps": {
			name: "single child unwraps",
			ruleSet: rules.RuleSet{
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
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND u.email = $1",
			wantArgs: []any{"user@example.com", testProjectID},
			wantErr:  false,
		},
		"empty wrapper": {
			name: "empty wrapper",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{},
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $1",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			qb := NewQueryBuilder(testProjectID, nil)
			result, err := qb.Query(tc.ruleSet)

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
		ruleSet  rules.RuleSet
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}

	tests := map[string]test{
		"single nested jsonb path": {
			name: "single nested jsonb path",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data.subscription",
					Operator: rules.OperatorEquals,
					Value:    "premium",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND (u.data->>'subscription')::text = $1",
			wantArgs: []any{"premium", testProjectID},
			wantErr:  false,
		},
		"bracket notation with spaces": {
			name: "bracket notation with spaces",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data['purchase agreement']",
					Operator: rules.OperatorEquals,
					Value:    "signed",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND (u.data->>'purchase agreement')::text = $1",
			wantArgs: []any{"signed", testProjectID},
			wantErr:  false,
		},
		"mixed bracket and dot notation": {
			name: "mixed bracket and dot notation",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data['purchase agreement'].value",
					Operator: rules.OperatorEquals,
					Value:    "active",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND (u.data->'purchase agreement'->>'value')::text = $1",
			wantArgs: []any{"active", testProjectID},
			wantErr:  false,
		},
		"bracket notation with double quotes": {
			name: "bracket notation with double quotes",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     `.data["user info"].name`,
					Operator: rules.OperatorEquals,
					Value:    "John",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND (u.data->'user info'->>'name')::text = $1",
			wantArgs: []any{"John", testProjectID},
			wantErr:  false,
		},
		"complex path with special characters": {
			name: "complex path with special characters",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data['customer-id'].verified",
					Operator: rules.OperatorEquals,
					Value:    true,
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND (u.data->'customer-id'->>'verified')::text = $1",
			wantArgs: []any{true, testProjectID},
			wantErr:  false,
		},
		"deeply nested with brackets": {
			name: "deeply nested with brackets",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeNumber,
					Group:    rules.RuleGroupUser,
					Path:     ".data['billing info']['payment method'].priority",
					Operator: rules.OperatorGreaterThan,
					Value:    5,
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND (u.data->'billing info'->'payment method'->>'priority')::numeric > $1",
			wantArgs: []any{5, testProjectID},
			wantErr:  false,
		},
		"double nested jsonb path": {
			name: "double nested jsonb path",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data.subscription.tier",
					Operator: rules.OperatorEquals,
					Value:    "gold",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND (u.data->'subscription'->>'tier')::text = $1",
			wantArgs: []any{"gold", testProjectID},
			wantErr:  false,
		},
		"triple nested jsonb path": {
			name: "triple nested jsonb path",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data.preferences.notifications.email",
					Operator: rules.OperatorEquals,
					Value:    true,
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND (u.data->'preferences'->'notifications'->>'email')::text = $1",
			wantArgs: []any{true, testProjectID},
			wantErr:  false,
		},
		"nested jsonb with text extraction": {
			name: "nested jsonb with text extraction",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data.company.name",
					Operator: rules.OperatorContains,
					Value:    "Tech",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND (u.data->'company'->>'name')::text ILIKE $1",
			wantArgs: []any{"%Tech%", testProjectID},
			wantErr:  false,
		},
		"nested jsonb number comparison": {
			name: "nested jsonb number comparison",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeNumber,
					Group:    rules.RuleGroupUser,
					Path:     ".data.metrics.score",
					Operator: rules.OperatorGreaterThan,
					Value:    100,
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND (u.data->'metrics'->>'score')::numeric > $1",
			wantArgs: []any{100, testProjectID},
			wantErr:  false,
		},
		"nested jsonb is set": {
			name: "nested jsonb is set",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data.settings.theme",
					Operator: rules.OperatorIsSet,
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $1 AND (u.data->'settings'->>'theme')::text IS NOT NULL",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
		"nested jsonb is not set": {
			name: "nested jsonb is not set",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".data.billing.card",
					Operator: rules.OperatorIsNotSet,
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $1 AND (u.data->'billing'->>'card')::text IS NULL",
			wantArgs: []any{testProjectID},
			wantErr:  false,
		},
		"nested jsonb in complex wrapper": {
			name: "nested jsonb in complex wrapper",
			ruleSet: rules.RuleSet{
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
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $3 AND ((u.data->'subscription'->>'plan')::text = $1 AND (u.data->'subscription'->>'seats')::numeric >= $2)",
			wantArgs: []any{"enterprise", 10, testProjectID},
			wantErr:  false,
		},
		"mixed regular and nested jsonb fields": {
			name: "mixed regular and nested jsonb fields",
			ruleSet: rules.RuleSet{
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
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $3 AND (u.email ILIKE $1 AND (u.data->'company'->>'verified')::text = $2)",
			wantArgs: []any{"%@enterprise.com", true, testProjectID},
			wantErr:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			qb := NewQueryBuilder(testProjectID, nil)
			result, err := qb.Query(tc.ruleSet)

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
		ruleSet rules.RuleSet
		wantErr bool
	}

	tests := map[string]test{
		"unsupported rule group": {
			name: "unsupported rule group",
			ruleSet: rules.RuleSet{
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
			ruleSet: rules.RuleSet{
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
			ruleSet: rules.RuleSet{
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
			qb := NewQueryBuilder(testProjectID, nil)
			_, err := qb.Query(tc.ruleSet)

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

func TestQueryBuilderBuildQuery(t *testing.T) {
	type test struct {
		name     string
		ruleSet  rules.RuleSet
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}

	tests := map[string]test{
		"simple user attribute": {
			name: "simple user attribute",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".email",
					Operator: rules.OperatorEquals,
					Value:    "user@example.com",
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $2 AND u.email = $1",
			wantArgs: []any{"user@example.com", testProjectID},
			wantErr:  false,
		},
		"simple event with JOIN": {
			name: "simple event with JOIN",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "login",
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT DISTINCT ue.user_id FROM user_events ue JOIN events e ON e.id = ue.event_id WHERE e.name = $1) e1 ON e1.user_id = u.id WHERE u.project_id = $2",
			wantArgs: []any{"login", testProjectID},
			wantErr:  false,
		},
		"frequency with JOIN HAVING": {
			name: "frequency with JOIN HAVING",
			ruleSet: rules.RuleSet{
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
							Unit:  rules.PeriodUnitDay,
							Value: 30,
						},
					},
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT ue.user_id FROM user_events ue JOIN events e ON e.id = ue.event_id WHERE ue.created_at >= NOW() - $1::interval AND e.name = $2 GROUP BY ue.user_id HAVING COUNT(*) >= $3) e1 ON e1.user_id = u.id WHERE u.project_id = $4",
			wantArgs: []any{"30 days", "purchase", 2, testProjectID},
			wantErr:  false,
		},
		"event with attributes": {
			name: "event with attributes",
			ruleSet: rules.RuleSet{
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
							Path:     ".product_category",
							Operator: rules.OperatorEquals,
							Value:    "electronics",
						},
					},
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT DISTINCT ue.user_id FROM user_events ue JOIN events e ON e.id = ue.event_id WHERE e.name = $1 AND ue.product_category = $2) e1 ON e1.user_id = u.id WHERE u.project_id = $3",
			wantArgs: []any{"purchase", "electronics", testProjectID},
			wantErr:  false,
		},
		"multiple events with AND": {
			name: "multiple events with AND",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupEvent,
							Path:     "",
							Operator: rules.OperatorEquals,
							Value:    "login",
						},
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupEvent,
							Path:     "",
							Operator: rules.OperatorEquals,
							Value:    "purchase",
						},
					},
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT DISTINCT ue.user_id FROM user_events ue JOIN events e ON e.id = ue.event_id WHERE e.name = $1) e1 ON e1.user_id = u.id JOIN (SELECT DISTINCT ue.user_id FROM user_events ue JOIN events e ON e.id = ue.event_id WHERE e.name = $2) e2 ON e2.user_id = u.id WHERE u.project_id = $3",
			wantArgs: []any{"login", "purchase", testProjectID},
			wantErr:  false,
		},
		"user attribute AND event": {
			name: "user attribute AND event",
			ruleSet: rules.RuleSet{
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
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupEvent,
							Path:     "",
							Operator: rules.OperatorEquals,
							Value:    "purchase",
						},
					},
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT DISTINCT ue.user_id FROM user_events ue JOIN events e ON e.id = ue.event_id WHERE e.name = $2) e1 ON e1.user_id = u.id WHERE u.project_id = $3 AND u.country = $1",
			wantArgs: []any{"US", "purchase", testProjectID},
			wantErr:  false,
		},
		"complex frequency with nested jsonb": {
			name: "complex frequency with nested jsonb",
			ruleSet: rules.RuleSet{
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
							Unit:  rules.PeriodUnitDay,
							Value: 30,
						},
					},
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
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT ue.user_id FROM user_events ue JOIN events e ON e.id = ue.event_id WHERE ue.created_at >= NOW() - $1::interval AND e.name = $2 AND (ue.data->'product'->>'category')::text = $3 AND (ue.data->'product'->>'price')::numeric >= $4 GROUP BY ue.user_id HAVING COUNT(*) >= $5) e1 ON e1.user_id = u.id WHERE u.project_id = $6",
			wantArgs: []any{"30 days", "purchase", "electronics", 1000, 2, testProjectID},
			wantErr:  false,
		},
		"frequency less than": {
			name: "frequency less than",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "error",
					Frequency: &rules.Frequency{
						Count:    3,
						Operator: rules.OperatorLessThan,
						Period: rules.Period{
							Type:  rules.PeriodTypeRolling,
							Unit:  rules.PeriodUnitDay,
							Value: 7,
						},
					},
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT ue.user_id FROM user_events ue JOIN events e ON e.id = ue.event_id WHERE ue.created_at >= NOW() - $1::interval AND e.name = $2 GROUP BY ue.user_id HAVING COUNT(*) < $3) e1 ON e1.user_id = u.id WHERE u.project_id = $4",
			wantArgs: []any{"7 days", "error", 3, testProjectID},
			wantErr:  false,
		},
		"frequency equals zero": {
			name: "frequency equals zero",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     "",
					Operator: rules.OperatorEquals,
					Value:    "login",
					Frequency: &rules.Frequency{
						Count:    0,
						Operator: rules.OperatorEquals,
						Period: rules.Period{
							Type:  rules.PeriodTypeRolling,
							Unit:  rules.PeriodUnitHour,
							Value: 24,
						},
					},
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT ue.user_id FROM user_events ue JOIN events e ON e.id = ue.event_id WHERE ue.created_at >= NOW() - $1::interval AND e.name = $2 GROUP BY ue.user_id HAVING COUNT(*) = $3) e1 ON e1.user_id = u.id WHERE u.project_id = $4",
			wantArgs: []any{"24 hours", "login", 0, testProjectID},
			wantErr:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			qb := NewQueryBuilder(testProjectID, nil)
			result, err := qb.Query(tc.ruleSet)

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

func TestQueryBuilderOrganizationRules(t *testing.T) {
	type test struct {
		name     string
		ruleSet  rules.RuleSet
		wantSQL  string
		wantArgs []any
		wantErr  bool
	}

	tests := map[string]test{
		"simple organization attribute": {
			name: "simple organization attribute",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupOrganization,
					Path:     ".name",
					Operator: rules.OperatorEquals,
					Value:    "Acme Corp",
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE o.project_id = $2 AND o.name = $1) e1 ON e1.user_id = u.id WHERE u.project_id = $3",
			wantArgs: []any{"Acme Corp", testProjectID, testProjectID},
			wantErr:  false,
		},
		"organization nested jsonb data": {
			name: "organization nested jsonb data",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupOrganization,
					Path:     ".data.tier",
					Operator: rules.OperatorEquals,
					Value:    "gold",
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE o.project_id = $2 AND (o.data->>'tier')::text = $1) e1 ON e1.user_id = u.id WHERE u.project_id = $3",
			wantArgs: []any{"gold", testProjectID, testProjectID},
			wantErr:  false,
		},
		"organization user role": {
			name: "organization user role",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupOrganizationUser,
					Path:     ".data.role",
					Operator: rules.OperatorEquals,
					Value:    "admin",
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE o.project_id = $2 AND (ou.data->>'role')::text = $1) e1 ON e1.user_id = u.id WHERE u.project_id = $3",
			wantArgs: []any{"admin", testProjectID, testProjectID},
			wantErr:  false,
		},
		"combined organization and organization user with AND": {
			name: "combined organization and organization user with AND",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupOrganization,
							Path:     ".data.tier",
							Operator: rules.OperatorEquals,
							Value:    "gold",
						},
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupOrganizationUser,
							Path:     ".data.role",
							Operator: rules.OperatorEquals,
							Value:    "admin",
						},
					},
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE o.project_id = $3 AND ((o.data->>'tier')::text = $1 AND (ou.data->>'role')::text = $2)) e1 ON e1.user_id = u.id WHERE u.project_id = $4",
			wantArgs: []any{"gold", "admin", testProjectID, testProjectID},
			wantErr:  false,
		},
		"combined organization and organization user with OR": {
			name: "combined organization and organization user with OR",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorOr,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupOrganization,
							Path:     ".data.tier",
							Operator: rules.OperatorEquals,
							Value:    "gold",
						},
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupOrganizationUser,
							Path:     ".data.role",
							Operator: rules.OperatorEquals,
							Value:    "admin",
						},
					},
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE o.project_id = $3 AND ((o.data->>'tier')::text = $1 OR (ou.data->>'role')::text = $2)) e1 ON e1.user_id = u.id WHERE u.project_id = $4",
			wantArgs: []any{"gold", "admin", testProjectID, testProjectID},
			wantErr:  false,
		},
		"user attribute AND organization attribute": {
			name: "user attribute AND organization attribute",
			ruleSet: rules.RuleSet{
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
							Group:    rules.RuleGroupOrganization,
							Path:     ".data.tier",
							Operator: rules.OperatorEquals,
							Value:    "enterprise",
						},
					},
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE o.project_id = $3 AND (o.data->>'tier')::text = $2) e1 ON e1.user_id = u.id WHERE u.project_id = $4 AND u.email ILIKE $1",
			wantArgs: []any{"%@enterprise.com", "enterprise", testProjectID, testProjectID},
			wantErr:  false,
		},
		"multiple organization conditions": {
			name: "multiple organization conditions",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupOrganization,
							Path:     ".data.tier",
							Operator: rules.OperatorEquals,
							Value:    "gold",
						},
						{
							Type:     rules.RuleTypeNumber,
							Group:    rules.RuleGroupOrganization,
							Path:     ".data.seats",
							Operator: rules.OperatorGreaterEqual,
							Value:    10,
						},
					},
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE o.project_id = $3 AND ((o.data->>'tier')::text = $1 AND (o.data->>'seats')::numeric >= $2)) e1 ON e1.user_id = u.id WHERE u.project_id = $4",
			wantArgs: []any{"gold", 10, testProjectID, testProjectID},
			wantErr:  false,
		},
		"organization with is set operator": {
			name: "organization with is set operator",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupOrganization,
					Path:     ".data.verified_at",
					Operator: rules.OperatorIsSet,
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE o.project_id = $1 AND (o.data->>'verified_at')::text IS NOT NULL) e1 ON e1.user_id = u.id WHERE u.project_id = $2",
			wantArgs: []any{testProjectID, testProjectID},
			wantErr:  false,
		},
		"user OR organization - should error": {
			name: "user OR organization - should error",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorOr,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".email",
							Operator: rules.OperatorEndsWith,
							Value:    "@admin.com",
						},
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupOrganization,
							Path:     ".data.tier",
							Operator: rules.OperatorEquals,
							Value:    "gold",
						},
					},
				},
			},
			wantSQL:  "",
			wantArgs: nil,
			wantErr:  true,
		},
		"user OR organization_user - should error": {
			name: "user OR organization_user - should error",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorOr,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".email",
							Operator: rules.OperatorEndsWith,
							Value:    "@admin.com",
						},
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupOrganizationUser,
							Path:     ".data.role",
							Operator: rules.OperatorEquals,
							Value:    "admin",
						},
					},
				},
			},
			wantSQL:  "",
			wantArgs: nil,
			wantErr:  true,
		},
		"nested user OR organization - should error": {
			name: "nested user OR organization - should error",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".name",
							Operator: rules.OperatorEquals,
							Value:    "John",
						},
						{
							Type:     rules.RuleTypeWrapper,
							Group:    rules.RuleGroupParent,
							Operator: rules.OperatorOr,
							Children: []rules.Rule{
								{
									Type:     rules.RuleTypeString,
									Group:    rules.RuleGroupUser,
									Path:     ".email",
									Operator: rules.OperatorEndsWith,
									Value:    "@vip.com",
								},
								{
									Type:     rules.RuleTypeString,
									Group:    rules.RuleGroupOrganization,
									Path:     ".data.tier",
									Operator: rules.OperatorEquals,
									Value:    "enterprise",
								},
							},
						},
					},
				},
			},
			wantSQL:  "",
			wantArgs: nil,
			wantErr:  true,
		},
		"deeply nested wrapper in OR - should error": {
			name: "deeply nested wrapper in OR - should error",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorOr,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeWrapper,
							Group:    rules.RuleGroupParent,
							Operator: rules.OperatorAnd,
							Children: []rules.Rule{
								{
									Type:     rules.RuleTypeString,
									Group:    rules.RuleGroupUser,
									Path:     ".email",
									Operator: rules.OperatorContains,
									Value:    "test",
								},
							},
						},
						{
							Type:     rules.RuleTypeWrapper,
							Group:    rules.RuleGroupParent,
							Operator: rules.OperatorAnd,
							Children: []rules.Rule{
								{
									Type:     rules.RuleTypeString,
									Group:    rules.RuleGroupOrganization,
									Path:     ".data.tier",
									Operator: rules.OperatorEquals,
									Value:    "gold",
								},
							},
						},
					},
				},
			},
			wantSQL:  "",
			wantArgs: nil,
			wantErr:  true,
		},
		"organization OR organization_user - should work": {
			name: "organization OR organization_user - should work",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorOr,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupOrganization,
							Path:     ".data.tier",
							Operator: rules.OperatorEquals,
							Value:    "gold",
						},
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupOrganizationUser,
							Path:     ".data.role",
							Operator: rules.OperatorEquals,
							Value:    "admin",
						},
					},
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE o.project_id = $3 AND ((o.data->>'tier')::text = $1 OR (ou.data->>'role')::text = $2)) e1 ON e1.user_id = u.id WHERE u.project_id = $4",
			wantArgs: []any{"gold", "admin", testProjectID, testProjectID},
			wantErr:  false,
		},
		"user OR user - should work": {
			name: "user OR user - should work",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorOr,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".email",
							Operator: rules.OperatorEndsWith,
							Value:    "@admin.com",
						},
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupUser,
							Path:     ".data.role",
							Operator: rules.OperatorEquals,
							Value:    "superuser",
						},
					},
				},
			},
			wantSQL:  "SELECT u.id FROM users u WHERE u.project_id = $3 AND (u.email ILIKE $1 OR (u.data->>'role')::text = $2)",
			wantArgs: []any{"%@admin.com", "superuser", testProjectID},
			wantErr:  false,
		},
		"nested org wrapper with parent - falls through to buildWrapper": {
			name: "nested org wrapper with parent - falls through to buildWrapper",
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeWrapper,
					Group:    rules.RuleGroupParent,
					Operator: rules.OperatorAnd,
					Children: []rules.Rule{
						{
							Type:     rules.RuleTypeString,
							Group:    rules.RuleGroupOrganization,
							Path:     ".data.tier",
							Operator: rules.OperatorEquals,
							Value:    "gold",
						},
						{
							Type:     rules.RuleTypeWrapper,
							Group:    rules.RuleGroupParent,
							Operator: rules.OperatorOr,
							Children: []rules.Rule{
								{
									Type:     rules.RuleTypeString,
									Group:    rules.RuleGroupOrganizationUser,
									Path:     ".data.role",
									Operator: rules.OperatorEquals,
									Value:    "admin",
								},
								{
									Type:     rules.RuleTypeString,
									Group:    rules.RuleGroupOrganizationUser,
									Path:     ".data.role",
									Operator: rules.OperatorEquals,
									Value:    "owner",
								},
							},
						},
					},
				},
			},
			wantSQL:  "SELECT u.id FROM users u JOIN (SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE o.project_id = $2 AND (o.data->>'tier')::text = $1) e1 ON e1.user_id = u.id JOIN (SELECT DISTINCT ou.user_id FROM organization_users ou JOIN organizations o ON o.id = ou.organization_id WHERE o.project_id = $5 AND ((ou.data->>'role')::text = $3 OR (ou.data->>'role')::text = $4)) e2 ON e2.user_id = u.id WHERE u.project_id = $6",
			wantArgs: []any{"gold", testProjectID, "admin", "owner", testProjectID, testProjectID},
			wantErr:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			qb := NewQueryBuilder(testProjectID, nil)
			result, err := qb.Query(tc.ruleSet)

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
