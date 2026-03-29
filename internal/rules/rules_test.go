package rules

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
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
