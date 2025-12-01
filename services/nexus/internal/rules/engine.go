package rules

import (
	"fmt"

	"github.com/google/uuid"
)

type RuleCheck interface {
	Check(params RuleCheckParams) bool
	Query(params RuleQueryParams) string
}

type RuleCheckParams struct {
	Registry Registry
	Input    RuleCheckInput
	Rule     *RuleTree
	Value    map[string]interface{}
}

type RuleQueryParams struct {
	Registry  Registry
	ProjectID uuid.UUID
	Rule      *RuleTree
}

type Registry interface {
	Register(ruleType RuleType, checker RuleCheck) Registry
	Get(ruleType RuleType) RuleCheck
}

type registry struct {
	registered map[RuleType]RuleCheck
}

func NewRegistry() Registry {
	return &registry{
		registered: make(map[RuleType]RuleCheck),
	}
}

func (r *registry) Register(ruleType RuleType, checker RuleCheck) Registry {
	r.registered[ruleType] = checker
	return r
}

func (r *registry) Get(ruleType RuleType) RuleCheck {
	return r.registered[ruleType]
}

type RuleEvalError struct {
	Rule    *RuleTree
	Message string
}

func (e *RuleEvalError) Error() string {
	return fmt.Sprintf("rule eval error: %s", e.Message)
}

var defaultRegistry Registry

func init() {
	defaultRegistry = NewRegistry()
	defaultRegistry.Register(RuleTypeNumber, &NumberRule{})
	defaultRegistry.Register(RuleTypeString, &StringRule{})
	defaultRegistry.Register(RuleTypeBoolean, &BooleanRule{})
	defaultRegistry.Register(RuleTypeDate, &DateRule{})
	defaultRegistry.Register(RuleTypeArray, &ArrayRule{})
	defaultRegistry.Register(RuleTypeWrapper, &WrapperRule{})
}

func Check(input RuleCheckInput, rules interface{}) bool {
	var rule *RuleTree
	
	switch v := rules.(type) {
	case *RuleTree:
		rule = v
	case []*RuleTree:
		rule = Make(MakeParams{
			Type:     RuleTypeWrapper,
			Group:    RuleGroupParent,
			Operator: OpAnd,
			Children: v,
		})
	default:
		return false
	}

	value := make(map[string]interface{})
	for k, v := range input.User {
		value[k] = v
	}
	value["journey"] = input.Journey

	checker := defaultRegistry.Get(rule.Type)
	if checker == nil {
		return false
	}

	return checker.Check(RuleCheckParams{
		Registry: defaultRegistry,
		Input:    input,
		Rule:     rule,
		Value:    value,
	})
}

func GetRuleQuery(projectID uuid.UUID, rules interface{}) string {
	var rule *RuleTree

	switch v := rules.(type) {
	case *RuleTree:
		rule = v
	case []*RuleTree:
		rule = Make(MakeParams{
			Type:     RuleTypeWrapper,
			Operator: OpAnd,
			Children: v,
		})
	default:
		return ""
	}

	checker := defaultRegistry.Get(rule.Type)
	if checker == nil {
		return ""
	}

	return checker.Query(RuleQueryParams{
		Registry:  defaultRegistry,
		ProjectID: projectID,
		Rule:      rule,
	})
}

type MakeParams struct {
	Type      RuleType
	Group     RuleGroup
	Path      string
	Operator  Operator
	Value     interface{}
	Children  []*RuleTree
	Frequency *EventRuleFrequency
}

func Make(params MakeParams) *RuleTree {
	if params.Group == "" {
		params.Group = RuleGroupUser
	}
	if params.Path == "" {
		params.Path = "$"
	}
	if params.Operator == "" {
		params.Operator = OpEqual
	}

	ruleUUID := uuid.New().String()
	
	var value []byte
	if params.Value != nil {
		v, _ := marshalValue(params.Value)
		value = v
	}

	rule := &RuleTree{
		Rule: Rule{
			UUID:     ruleUUID,
			Type:     params.Type,
			Group:    params.Group,
			Path:     params.Path,
			Operator: params.Operator,
			Value:    value,
		},
		Children:  params.Children,
		Frequency: params.Frequency,
	}

	for _, child := range params.Children {
		child.ParentUUID = &ruleUUID
	}

	return rule
}
