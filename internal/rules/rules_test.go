package rules

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuleEvents(t *testing.T) {
	t.Parallel()

	type test struct {
		rule     Rule
		expected []string
	}

	tests := map[string]test{
		"single event rule": {
			rule: Rule{
				Type:  RuleTypeWrapper,
				Group: RuleGroupEvent,
				Value: "user.created",
			},
			expected: []string{"user.created"},
		},
		"multiple event rules": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "user.created",
					},
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "user.updated",
					},
				},
			},
			expected: []string{"user.created", "user.updated"},
		},
		"nested event rules": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "order.created",
					},
					{
						Type:     RuleTypeWrapper,
						Group:    RuleGroupParent,
						Operator: OperatorOr,
						Children: []Rule{
							{
								Type:  RuleTypeWrapper,
								Group: RuleGroupEvent,
								Value: "payment.completed",
							},
							{
								Type:  RuleTypeWrapper,
								Group: RuleGroupEvent,
								Value: "payment.failed",
							},
						},
					},
				},
			},
			expected: []string{"order.created", "payment.completed", "payment.failed"},
		},
		"no events": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupUser,
						Path:     "email",
						Operator: OperatorContains,
						Value:    "example.com",
					},
				},
			},
			expected: nil,
		},
		"mixed event and user rules": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "purchase.completed",
					},
					{
						Type:     RuleTypeNumber,
						Group:    RuleGroupUser,
						Path:     "data.age",
						Operator: OperatorGreaterThan,
						Value:    18,
					},
				},
			},
			expected: []string{"purchase.completed"},
		},
		"empty rule": {
			rule:     Rule{},
			expected: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			events := test.rule.UserEvents()
			assert.Equal(t, test.expected, events)
		})
	}
}

func TestRuleOrganizationEvents(t *testing.T) {
	t.Parallel()

	type test struct {
		rule     Rule
		expected []string
	}

	tests := map[string]test{
		"single organization event rule": {
			rule: Rule{
				Type:  RuleTypeWrapper,
				Group: RuleGroupOrganizationEvent,
				Value: "org.created",
			},
			expected: []string{"org.created"},
		},
		"multiple organization event rules": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupOrganizationEvent,
						Value: "org.created",
					},
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupOrganizationEvent,
						Value: "org.updated",
					},
				},
			},
			expected: []string{"org.created", "org.updated"},
		},
		"nested organization event rules": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupOrganizationEvent,
						Value: "subscription.created",
					},
					{
						Type:     RuleTypeWrapper,
						Group:    RuleGroupParent,
						Operator: OperatorOr,
						Children: []Rule{
							{
								Type:  RuleTypeWrapper,
								Group: RuleGroupOrganizationEvent,
								Value: "payment.completed",
							},
							{
								Type:  RuleTypeWrapper,
								Group: RuleGroupOrganizationEvent,
								Value: "payment.failed",
							},
						},
					},
				},
			},
			expected: []string{"subscription.created", "payment.completed", "payment.failed"},
		},
		"no organization events": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupOrganization,
						Path:     "name",
						Operator: OperatorContains,
						Value:    "example",
					},
				},
			},
			expected: nil,
		},
		"mixed organization event and user event rules": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupOrganizationEvent,
						Value: "org.purchase.completed",
					},
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "user.login",
					},
				},
			},
			expected: []string{"org.purchase.completed"},
		},
		"empty rule": {
			rule:     Rule{},
			expected: nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			events := test.rule.OrganizationEvents()
			assert.Equal(t, test.expected, events)
		})
	}
}

func TestRuleDependsOnEvents(t *testing.T) {
	t.Parallel()

	type test struct {
		rule     Rule
		expected bool
	}

	tests := map[string]test{
		"single event rule": {
			rule: Rule{
				Type:  RuleTypeWrapper,
				Group: RuleGroupEvent,
				Value: "user.login",
			},
			expected: true,
		},
		"nested with event": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupUser,
						Path:     "email",
						Operator: OperatorContains,
						Value:    "test",
					},
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "email.sent",
					},
				},
			},
			expected: true,
		},
		"deeply nested with event": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupUser,
						Path:     "name",
						Operator: OperatorEquals,
						Value:    "John",
					},
					{
						Type:     RuleTypeWrapper,
						Group:    RuleGroupParent,
						Operator: OperatorOr,
						Children: []Rule{
							{
								Type:     RuleTypeNumber,
								Group:    RuleGroupUser,
								Path:     "age",
								Operator: OperatorLessThan,
								Value:    30,
							},
							{
								Type:  RuleTypeWrapper,
								Group: RuleGroupEvent,
								Value: "subscription.renewed",
							},
						},
					},
				},
			},
			expected: true,
		},
		"only user rules": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupUser,
						Path:     "email",
						Operator: OperatorContains,
						Value:    "example.com",
					},
					{
						Type:     RuleTypeBoolean,
						Group:    RuleGroupUser,
						Path:     "data.verified",
						Operator: OperatorEquals,
						Value:    true,
					},
				},
			},
			expected: false,
		},
		"empty rule": {
			rule:     Rule{},
			expected: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := test.rule.DependsOnEvents()
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestRuleDependsOnUsers(t *testing.T) {
	t.Parallel()

	type test struct {
		rule     Rule
		expected bool
	}

	tests := map[string]test{
		"single user rule": {
			rule: Rule{
				Type:     RuleTypeString,
				Group:    RuleGroupUser,
				Path:     "email",
				Operator: OperatorContains,
				Value:    "@example.com",
			},
			expected: true,
		},
		"nested with user rule": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "page.viewed",
					},
					{
						Type:     RuleTypeNumber,
						Group:    RuleGroupUser,
						Path:     "data.visits",
						Operator: OperatorGreaterEqual,
						Value:    10,
					},
				},
			},
			expected: true,
		},
		"deeply nested with user rule": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorOr,
				Children: []Rule{
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "cart.abandoned",
					},
					{
						Type:     RuleTypeWrapper,
						Group:    RuleGroupParent,
						Operator: OperatorAnd,
						Children: []Rule{
							{
								Type:     RuleTypeString,
								Group:    RuleGroupUser,
								Path:     "locale",
								Operator: OperatorEquals,
								Value:    "en",
							},
							{
								Type:     RuleTypeBoolean,
								Group:    RuleGroupUser,
								Path:     "data.subscribed",
								Operator: OperatorEquals,
								Value:    true,
							},
						},
					},
				},
			},
			expected: true,
		},
		"only event rules": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "order.placed",
					},
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "payment.received",
					},
				},
			},
			expected: false,
		},
		"empty rule": {
			rule:     Rule{},
			expected: false,
		},
		"parent wrapper with no children matches all users": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
			},
			expected: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := test.rule.DependsOnUsers()
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestRuleHasChildren(t *testing.T) {
	t.Parallel()

	type test struct {
		rule     Rule
		expected bool
	}

	tests := map[string]test{
		"with children": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:  RuleTypeString,
						Group: RuleGroupUser,
						Path:  "name",
					},
				},
			},
			expected: true,
		},
		"without children": {
			rule: Rule{
				Type:  RuleTypeString,
				Group: RuleGroupUser,
				Path:  "email",
			},
			expected: false,
		},
		"empty children slice": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Children: []Rule{},
			},
			expected: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := test.rule.HasChildren()
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestRuleIsRoot(t *testing.T) {
	t.Parallel()

	type test struct {
		rule     Rule
		expected bool
	}

	parentUUID := uuid.New()

	tests := map[string]test{
		"root rule (nil parent)": {
			rule: Rule{
				UUID:       uuid.New(),
				ParentUUID: nil,
			},
			expected: true,
		},
		"child rule (has parent)": {
			rule: Rule{
				UUID:       uuid.New(),
				ParentUUID: &parentUUID,
			},
			expected: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := test.rule.IsRoot()
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestRuleDependsOnOrganizations(t *testing.T) {
	t.Parallel()

	type test struct {
		rule     Rule
		expected bool
	}

	tests := map[string]test{
		"single organization rule": {
			rule: Rule{
				Type:     RuleTypeString,
				Group:    RuleGroupOrganization,
				Path:     ".data.tier",
				Operator: OperatorEquals,
				Value:    "gold",
			},
			expected: true,
		},
		"nested with organization rule": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupUser,
						Path:     "email",
						Operator: OperatorContains,
						Value:    "test",
					},
					{
						Type:     RuleTypeString,
						Group:    RuleGroupOrganization,
						Path:     ".name",
						Operator: OperatorEquals,
						Value:    "Acme",
					},
				},
			},
			expected: true,
		},
		"deeply nested with organization rule": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupUser,
						Path:     "name",
						Operator: OperatorEquals,
						Value:    "John",
					},
					{
						Type:     RuleTypeWrapper,
						Group:    RuleGroupParent,
						Operator: OperatorOr,
						Children: []Rule{
							{
								Type:     RuleTypeNumber,
								Group:    RuleGroupUser,
								Path:     "age",
								Operator: OperatorLessThan,
								Value:    30,
							},
							{
								Type:     RuleTypeString,
								Group:    RuleGroupOrganization,
								Path:     ".data.plan",
								Operator: OperatorEquals,
								Value:    "enterprise",
							},
						},
					},
				},
			},
			expected: true,
		},
		"only user rules": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupUser,
						Path:     "email",
						Operator: OperatorContains,
						Value:    "example.com",
					},
					{
						Type:     RuleTypeBoolean,
						Group:    RuleGroupUser,
						Path:     "data.verified",
						Operator: OperatorEquals,
						Value:    true,
					},
				},
			},
			expected: false,
		},
		"only organization user rules": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupOrganizationUser,
						Path:     ".data.role",
						Operator: OperatorEquals,
						Value:    "admin",
					},
				},
			},
			expected: false,
		},
		"empty rule": {
			rule:     Rule{},
			expected: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := test.rule.DependsOnOrganizations()
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestRuleDependsOnOrganizationUsers(t *testing.T) {
	t.Parallel()

	type test struct {
		rule     Rule
		expected bool
	}

	tests := map[string]test{
		"single organization user rule": {
			rule: Rule{
				Type:     RuleTypeString,
				Group:    RuleGroupOrganizationUser,
				Path:     ".data.role",
				Operator: OperatorEquals,
				Value:    "admin",
			},
			expected: true,
		},
		"nested with organization user rule": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupUser,
						Path:     "email",
						Operator: OperatorContains,
						Value:    "test",
					},
					{
						Type:     RuleTypeString,
						Group:    RuleGroupOrganizationUser,
						Path:     ".data.role",
						Operator: OperatorEquals,
						Value:    "member",
					},
				},
			},
			expected: true,
		},
		"deeply nested with organization user rule": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupUser,
						Path:     "name",
						Operator: OperatorEquals,
						Value:    "John",
					},
					{
						Type:     RuleTypeWrapper,
						Group:    RuleGroupParent,
						Operator: OperatorOr,
						Children: []Rule{
							{
								Type:     RuleTypeNumber,
								Group:    RuleGroupUser,
								Path:     "age",
								Operator: OperatorLessThan,
								Value:    30,
							},
							{
								Type:     RuleTypeString,
								Group:    RuleGroupOrganizationUser,
								Path:     ".data.permissions",
								Operator: OperatorContains,
								Value:    "write",
							},
						},
					},
				},
			},
			expected: true,
		},
		"only user rules": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupUser,
						Path:     "email",
						Operator: OperatorContains,
						Value:    "example.com",
					},
					{
						Type:     RuleTypeBoolean,
						Group:    RuleGroupUser,
						Path:     "data.verified",
						Operator: OperatorEquals,
						Value:    true,
					},
				},
			},
			expected: false,
		},
		"only organization rules": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupOrganization,
						Path:     ".data.tier",
						Operator: OperatorEquals,
						Value:    "gold",
					},
				},
			},
			expected: false,
		},
		"mixed organization and organization user": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupOrganization,
						Path:     ".data.tier",
						Operator: OperatorEquals,
						Value:    "gold",
					},
					{
						Type:     RuleTypeString,
						Group:    RuleGroupOrganizationUser,
						Path:     ".data.role",
						Operator: OperatorEquals,
						Value:    "admin",
					},
				},
			},
			expected: true,
		},
		"empty rule": {
			rule:     Rule{},
			expected: false,
		},
		"organization wrapper with member conditions": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeWrapper,
						Group:    RuleGroupOrganization,
						Operator: OperatorAnd,
						UserMatch: &UserMatch{
							Type: UserMatchConditions,
							MemberConditions: &Rule{
								Type:     RuleTypeWrapper,
								Group:    RuleGroupParent,
								Operator: OperatorAnd,
								Children: []Rule{
									{
										Type:     RuleTypeBoolean,
										Group:    RuleGroupUser,
										Path:     "is_primary_contact",
										Operator: OperatorEquals,
										Value:    false,
									},
								},
							},
						},
					},
				},
			},
			expected: true,
		},
		"organization wrapper with user_match all": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeWrapper,
						Group:    RuleGroupOrganization,
						Operator: OperatorAnd,
						UserMatch: &UserMatch{
							Type: UserMatchAll,
						},
					},
				},
			},
			expected: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := test.rule.DependsOnOrganizationUsers()
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestRuleTypeSQL(t *testing.T) {
	t.Parallel()

	type test struct {
		ruleType RuleType
		expected string
	}

	tests := map[string]test{
		"number": {
			ruleType: RuleTypeNumber,
			expected: "numeric",
		},
		"boolean": {
			ruleType: RuleTypeBoolean,
			expected: "boolean",
		},
		"date": {
			ruleType: RuleTypeDate,
			expected: "timestamp",
		},
		"string": {
			ruleType: RuleTypeString,
			expected: "text",
		},
		"array": {
			ruleType: RuleTypeArray,
			expected: "text",
		},
		"wrapper": {
			ruleType: RuleTypeWrapper,
			expected: "text",
		},
		"unknown": {
			ruleType: RuleType("unknown"),
			expected: "text",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := test.ruleType.SQL()
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestOperatorSQL(t *testing.T) {
	t.Parallel()

	type test struct {
		operator Operator
		expected string
	}

	tests := map[string]test{
		"and": {
			operator: OperatorAnd,
			expected: "AND",
		},
		"or": {
			operator: OperatorOr,
			expected: "OR",
		},
		"equals (default)": {
			operator: OperatorEquals,
			expected: "AND",
		},
		"unknown (default)": {
			operator: Operator("unknown"),
			expected: "AND",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := test.operator.SQL()
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestPeriodUnitSQL(t *testing.T) {
	t.Parallel()

	type test struct {
		unit     PeriodUnit
		expected string
	}

	tests := map[string]test{
		"minute": {
			unit:     PeriodUnitMinute,
			expected: "minutes",
		},
		"hour": {
			unit:     PeriodUnitHour,
			expected: "hours",
		},
		"day": {
			unit:     PeriodUnitDay,
			expected: "days",
		},
		"week": {
			unit:     PeriodUnitWeek,
			expected: "weeks",
		},
		"month": {
			unit:     PeriodUnitMonth,
			expected: "months",
		},
		"year": {
			unit:     PeriodUnitYear,
			expected: "years",
		},
		"unknown (default)": {
			unit:     PeriodUnit("unknown"),
			expected: "days",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := test.unit.SQL()
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestPeriodUnitRecomputeInterval(t *testing.T) {
	t.Parallel()

	type test struct {
		unit     PeriodUnit
		expected time.Duration
	}

	tests := map[string]test{
		"minute": {
			unit:     PeriodUnitMinute,
			expected: time.Minute,
		},
		"hour": {
			unit:     PeriodUnitHour,
			expected: 5 * time.Minute,
		},
		"day": {
			unit:     PeriodUnitDay,
			expected: time.Hour,
		},
		"week": {
			unit:     PeriodUnitWeek,
			expected: 6 * time.Hour,
		},
		"month": {
			unit:     PeriodUnitMonth,
			expected: 24 * time.Hour,
		},
		"year": {
			unit:     PeriodUnitYear,
			expected: 7 * 24 * time.Hour,
		},
		"unknown (default)": {
			unit:     PeriodUnit("unknown"),
			expected: time.Hour,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := test.unit.RecomputeInterval()
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestRuleDependsOnTime(t *testing.T) {
	t.Parallel()

	type test struct {
		rule     Rule
		expected bool
	}

	tests := map[string]test{
		"event rule with rolling frequency": {
			rule: Rule{
				Type:  RuleTypeWrapper,
				Group: RuleGroupEvent,
				Value: "purchase",
				Frequency: &Frequency{
					Count:    1,
					Operator: OperatorGreaterEqual,
					Period: Period{
						Type:  PeriodTypeRolling,
						Unit:  PeriodUnitDay,
						Value: 30,
					},
				},
			},
			expected: true,
		},
		"event rule without frequency": {
			rule: Rule{
				Type:  RuleTypeWrapper,
				Group: RuleGroupEvent,
				Value: "purchase",
			},
			expected: false,
		},
		"event rule with since_entered period": {
			rule: Rule{
				Type:  RuleTypeWrapper,
				Group: RuleGroupEvent,
				Value: "purchase",
				Frequency: &Frequency{
					Count:    1,
					Operator: OperatorGreaterEqual,
					Period: Period{
						Type:  PeriodTypeSinceEntered,
						Unit:  PeriodUnitDay,
						Value: 7,
					},
				},
			},
			expected: false,
		},
		"nested rolling frequency": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupUser,
						Path:     "email",
						Operator: OperatorContains,
						Value:    "@example.com",
					},
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "page.viewed",
						Frequency: &Frequency{
							Count:    5,
							Operator: OperatorGreaterEqual,
							Period: Period{
								Type:  PeriodTypeRolling,
								Unit:  PeriodUnitHour,
								Value: 24,
							},
						},
					},
				},
			},
			expected: true,
		},
		"user-only rule": {
			rule: Rule{
				Type:     RuleTypeString,
				Group:    RuleGroupUser,
				Path:     "email",
				Operator: OperatorContains,
				Value:    "@example.com",
			},
			expected: false,
		},
		"organization event with rolling frequency": {
			rule: Rule{
				Type:  RuleTypeWrapper,
				Group: RuleGroupOrganizationEvent,
				Value: "subscription.renewed",
				Frequency: &Frequency{
					Count:    1,
					Operator: OperatorGreaterEqual,
					Period: Period{
						Type:  PeriodTypeRolling,
						Unit:  PeriodUnitMonth,
						Value: 1,
					},
				},
			},
			expected: true,
		},
		"empty rule": {
			rule:     Rule{},
			expected: false,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result := test.rule.DependsOnTime()
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestRuleSetRecomputeInterval(t *testing.T) {
	t.Parallel()

	t.Run("no rolling periods returns nil", func(t *testing.T) {
		rs := RuleSet{
			Rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupUser,
						Path:     "email",
						Operator: OperatorContains,
						Value:    "@example.com",
					},
				},
			},
		}
		assert.Nil(t, rs.RecomputeInterval())
	})

	t.Run("single rolling period returns tier interval", func(t *testing.T) {
		rs := RuleSet{
			Rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "purchase",
						Frequency: &Frequency{
							Count:    1,
							Operator: OperatorGreaterEqual,
							Period: Period{
								Type:  PeriodTypeRolling,
								Unit:  PeriodUnitDay,
								Value: 30,
							},
						},
					},
				},
			},
		}
		interval := rs.RecomputeInterval()
		require.NotNil(t, interval)
		assert.Equal(t, time.Hour, *interval)
	})

	t.Run("multiple rolling periods returns smallest interval", func(t *testing.T) {
		rs := RuleSet{
			Rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "purchase",
						Frequency: &Frequency{
							Count:    1,
							Operator: OperatorGreaterEqual,
							Period: Period{
								Type:  PeriodTypeRolling,
								Unit:  PeriodUnitMonth,
								Value: 1,
							},
						},
					},
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "page.viewed",
						Frequency: &Frequency{
							Count:    5,
							Operator: OperatorGreaterEqual,
							Period: Period{
								Type:  PeriodTypeRolling,
								Unit:  PeriodUnitHour,
								Value: 2,
							},
						},
					},
				},
			},
		}
		interval := rs.RecomputeInterval()
		require.NotNil(t, interval)
		// hour tier (5 min) < month tier (24 hours), so hour wins
		assert.Equal(t, 5*time.Minute, *interval)
	})

	t.Run("mixed rolling and non-rolling", func(t *testing.T) {
		rs := RuleSet{
			Rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{
					{
						Type:     RuleTypeString,
						Group:    RuleGroupUser,
						Path:     "email",
						Operator: OperatorContains,
						Value:    "@example.com",
					},
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "login",
						Frequency: &Frequency{
							Count:    1,
							Operator: OperatorGreaterEqual,
							Period: Period{
								Type:  PeriodTypeRolling,
								Unit:  PeriodUnitWeek,
								Value: 1,
							},
						},
					},
					{
						Type:  RuleTypeWrapper,
						Group: RuleGroupEvent,
						Value: "session.started",
						Frequency: &Frequency{
							Count:    1,
							Operator: OperatorGreaterEqual,
							Period: Period{
								Type:  PeriodTypeSinceEntered,
								Unit:  PeriodUnitDay,
								Value: 7,
							},
						},
					},
				},
			},
		}
		interval := rs.RecomputeInterval()
		require.NotNil(t, interval)
		// Only the week rolling period counts; since_entered is not rolling
		assert.Equal(t, 6*time.Hour, *interval)
	})
}

func TestRuleSetPartitions(t *testing.T) {
	t.Parallel()

	userRule := Rule{Type: RuleTypeString, Group: RuleGroupUser, Path: ".tier", Operator: OperatorEquals, Value: "pro"}
	journeyRule := Rule{Type: RuleTypeNumber, Group: RuleGroupJourney, Path: "journey.entrance.amount", Operator: OperatorGreaterThan, Value: 10}
	stepRule := Rule{Type: RuleTypeNumber, Group: RuleGroupJourneyStep, Operator: OperatorGreaterThan, Value: 3}

	type test struct {
		ruleSet    RuleSet
		local      []Rule
		stepVisits []Rule
		historical []Rule
	}

	tests := map[string]test{
		"all three groups": {
			ruleSet: RuleSet{Rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{userRule, journeyRule, stepRule},
			}},
			local:      []Rule{journeyRule},
			stepVisits: []Rule{stepRule},
			historical: []Rule{userRule},
		},
		"step visits only": {
			ruleSet: RuleSet{Rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{stepRule},
			}},
			stepVisits: []Rule{stepRule},
		},
		"historical only": {
			ruleSet: RuleSet{Rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Operator: OperatorAnd,
				Children: []Rule{userRule},
			}},
			historical: []Rule{userRule},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assertPartition(t, "local", tc.ruleSet.Local(), tc.local)
			assertPartition(t, "step visits", tc.ruleSet.StepVisits(), tc.stepVisits)
			assertPartition(t, "historical", tc.ruleSet.Historical(), tc.historical)
		})
	}
}

func assertPartition(t *testing.T, name string, got *RuleSet, expected []Rule) {
	t.Helper()

	if len(expected) == 0 {
		assert.Nil(t, got, "%s partition should be empty", name)
		return
	}

	require.NotNil(t, got, "%s partition should not be empty", name)
	assert.Equal(t, expected, got.Children, "%s partition children", name)
}

func TestRuleDependsOnJourneySteps(t *testing.T) {
	t.Parallel()

	type test struct {
		rule     Rule
		expected bool
	}

	tests := map[string]test{
		"step visit rule": {
			rule:     Rule{Type: RuleTypeNumber, Group: RuleGroupJourneyStep},
			expected: true,
		},
		"nested step visit rule": {
			rule: Rule{
				Type:     RuleTypeWrapper,
				Group:    RuleGroupParent,
				Children: []Rule{{Type: RuleTypeNumber, Group: RuleGroupJourneyStep}},
			},
			expected: true,
		},
		"user rule": {
			rule:     Rule{Type: RuleTypeString, Group: RuleGroupUser},
			expected: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, tc.rule.DependsOnJourneySteps())
		})
	}
}

func TestRuleScope(t *testing.T) {
	t.Parallel()

	journeyScope := StepScopeJourney
	entryScope := StepScopeEntry

	assert.Equal(t, StepScopeEntry, Rule{}.Scope())
	assert.Equal(t, StepScopeEntry, Rule{StepScope: &entryScope}.Scope())
	assert.Equal(t, StepScopeJourney, Rule{StepScope: &journeyScope}.Scope())
}
