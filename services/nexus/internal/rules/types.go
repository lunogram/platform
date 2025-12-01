package rules

import (
	"encoding/json"

	"github.com/google/uuid"
)

type Operator string

const (
	OpEqual          Operator = "="
	OpNotEqual       Operator = "!="
	OpLessThan       Operator = "<"
	OpLessThanEq     Operator = "<="
	OpGreaterThan    Operator = ">"
	OpGreaterThanEq  Operator = ">="
	OpIsSet          Operator = "is set"
	OpIsNotSet       Operator = "is not set"
	OpOr             Operator = "or"
	OpAnd            Operator = "and"
	OpEmpty          Operator = "empty"
	OpContains       Operator = "contains"
	OpNotContain     Operator = "not contain"
	OpStartsWith     Operator = "starts with"
	OpNotStartWith   Operator = "not start with"
	OpEndsWith       Operator = "ends with"
	OpAny            Operator = "any"
	OpNone           Operator = "none"
	OpIsSameDay      Operator = "is same day"
)

type RuleType string

const (
	RuleTypeWrapper RuleType = "wrapper"
	RuleTypeString  RuleType = "string"
	RuleTypeNumber  RuleType = "number"
	RuleTypeBoolean RuleType = "boolean"
	RuleTypeDate    RuleType = "date"
	RuleTypeArray   RuleType = "array"
)

type RuleGroup string

const (
	RuleGroupUser   RuleGroup = "user"
	RuleGroupEvent  RuleGroup = "event"
	RuleGroupParent RuleGroup = "parent"
)

type Rule struct {
	UUID       string          `json:"uuid"`
	RootUUID   *string         `json:"root_uuid,omitempty"`
	ParentUUID *string         `json:"parent_uuid,omitempty"`
	Type       RuleType        `json:"type"`
	Group      RuleGroup       `json:"group"`
	Path       string          `json:"path"`
	Operator   Operator        `json:"operator"`
	Value      json.RawMessage `json:"value,omitempty"`
}

type PeriodType string

const (
	PeriodTypeRolling PeriodType = "rolling"
	PeriodTypeFixed   PeriodType = "fixed"
)

type TimeUnit string

const (
	TimeUnitHour   TimeUnit = "hour"
	TimeUnitMinute TimeUnit = "minute"
	TimeUnitDay    TimeUnit = "day"
	TimeUnitWeek   TimeUnit = "week"
	TimeUnitMonth  TimeUnit = "month"
	TimeUnitYear   TimeUnit = "year"
)

type EventRulePeriod struct {
	Type      PeriodType `json:"type"`
	Unit      *TimeUnit  `json:"unit,omitempty"`
	Value     *int       `json:"value,omitempty"`
	StartDate *string    `json:"start_date,omitempty"`
	EndDate   *string    `json:"end_date,omitempty"`
}

type EventRuleFrequency struct {
	Period   EventRulePeriod `json:"period"`
	Operator Operator        `json:"operator"`
	Count    int             `json:"count"`
}

type RuleTree struct {
	Rule
	Children  []*RuleTree         `json:"children,omitempty"`
	ID        *uuid.UUID          `json:"id,omitempty"`
	Frequency *EventRuleFrequency `json:"frequency,omitempty"`
}

type TemplateUser map[string]interface{}

type TemplateEvent map[string]interface{}

type RuleCheckInput struct {
	User    TemplateUser           `json:"user"`
	Events  []TemplateEvent        `json:"events"`
	Journey map[string]interface{} `json:"journey"`
}
