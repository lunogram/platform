package rules

import (
	"github.com/google/uuid"
)

// RuleType defines the type of rule node
type RuleType string

const (
	RuleTypeWrapper RuleType = "wrapper"
	RuleTypeString  RuleType = "string"
	RuleTypeNumber  RuleType = "number"
	RuleTypeBoolean RuleType = "boolean"
	RuleTypeDate    RuleType = "date"
	RuleTypeArray   RuleType = "array"
)

// SQL returns the PostgreSQL type for casting JSONB text extraction
func (rt RuleType) SQL() string {
	switch rt {
	case RuleTypeNumber:
		return "numeric"
	case RuleTypeBoolean:
		return "boolean"
	case RuleTypeDate:
		return "timestamp"
	case RuleTypeString, RuleTypeArray:
		return "text"
	default:
		return "text"
	}
}

// RuleGroup defines the group a rule belongs to
type RuleGroup string

const (
	RuleGroupParent RuleGroup = "parent"
	RuleGroupUser   RuleGroup = "user"
	RuleGroupEvent  RuleGroup = "event"
)

// Operator defines logical and comparison operators
type Operator string

const (
	// Logical operators
	OperatorAnd Operator = "and"
	OperatorOr  Operator = "or"

	// Comparison operators
	OperatorEquals       Operator = "="
	OperatorNotEquals    Operator = "!="
	OperatorLessThan     Operator = "<"
	OperatorLessEqual    Operator = "<="
	OperatorGreaterThan  Operator = ">"
	OperatorGreaterEqual Operator = ">="

	// Existence operators
	OperatorIsSet    Operator = "is set"
	OperatorIsNotSet Operator = "is not set"
	OperatorEmpty    Operator = "empty"

	// String operators
	OperatorContains     Operator = "contains"
	OperatorNotContain   Operator = "not contain"
	OperatorStartsWith   Operator = "starts with"
	OperatorNotStartWith Operator = "not start with"
	OperatorEndsWith     Operator = "ends with"

	// Array operators
	OperatorAny  Operator = "any"
	OperatorNone Operator = "none"

	// Date operators
	OperatorIsSameDay Operator = "is same day"
)

func (op Operator) SQL() string {
	switch op {
	case OperatorAnd:
		return "AND"
	case OperatorOr:
		return "OR"
	default:
		return "AND"
	}
}

// PeriodType defines the type of time period
type PeriodType string

const (
	PeriodTypeRolling  PeriodType = "rolling"
	PeriodTypeAbsolute PeriodType = "absolute"
)

// PeriodUnit defines the unit of time for a period
type PeriodUnit string

const (
	PeriodUnitMinute PeriodUnit = "minute"
	PeriodUnitHour   PeriodUnit = "hour"
	PeriodUnitDay    PeriodUnit = "day"
	PeriodUnitWeek   PeriodUnit = "week"
	PeriodUnitMonth  PeriodUnit = "month"
	PeriodUnitYear   PeriodUnit = "year"
)

// SQL returns the PostgreSQL interval unit name
func (unit PeriodUnit) SQL() string {
	switch unit {
	case PeriodUnitMinute:
		return "minutes"
	case PeriodUnitHour:
		return "hours"
	case PeriodUnitDay:
		return "days"
	case PeriodUnitWeek:
		return "weeks"
	case PeriodUnitMonth:
		return "months"
	case PeriodUnitYear:
		return "years"
	default:
		return "days"
	}
}

// Period defines a time period for frequency rules
type Period struct {
	Type  PeriodType `json:"type"`
	Unit  PeriodUnit `json:"unit"`
	Value int        `json:"value"`
}

// Frequency defines how often an event should occur
type Frequency struct {
	Count    int      `json:"count"`
	Period   Period   `json:"period"`
	Operator Operator `json:"operator"`
}

// Rule represents a rule node in the rules tree
type Rule struct {
	Path       string     `json:"path"`
	Type       RuleType   `json:"type"`
	UUID       uuid.UUID  `json:"uuid"`
	Group      RuleGroup  `json:"group"`
	Value      any        `json:"value,omitempty"`
	Operator   Operator   `json:"operator,omitempty"`
	Children   []Rule     `json:"children,omitempty"`
	Frequency  *Frequency `json:"frequency,omitempty"`
	RootUUID   *uuid.UUID `json:"root_uuid,omitempty"`
	ParentUUID *uuid.UUID `json:"parent_uuid,omitempty"`
}

// RuleSet represents the complete rule configuration
type RuleSet struct {
	Rule
}

// IsWrapper returns true if the rule is a wrapper (container) node
func (r *Rule) IsWrapper() bool {
	return r.Type == RuleTypeWrapper
}

// HasChildren returns true if the rule has child rules
func (r *Rule) HasChildren() bool {
	return len(r.Children) > 0
}

// IsRoot returns true if the rule is a root node
func (r *Rule) IsRoot() bool {
	return r.ParentUUID == nil
}
