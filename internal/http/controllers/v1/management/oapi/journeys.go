package oapi

import (
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rules"
)

// I have been fighting with oneoffs for too long, so here is all step data in one file
// might want to take a better approach later

// Step data format constants
const (
	Duration DelayStepDataFormat = "duration"
	Time     DelayStepDataFormat = "time"
	Date     DelayStepDataFormat = "date"
)

type DelayStepDataFormat string

// EntranceStepData represents data for entrance step - entry point into journey
type EntranceStepData struct {
	Trigger    *string        `json:"trigger,omitempty"`
	EventName  *string        `json:"event_name,omitempty"`
	ListId     *string        `json:"list_id,omitempty"`
	Schedule   *string        `json:"schedule,omitempty"`
	Rule       *rules.RuleSet `json:"rule,omitempty"`
	UserRule   *rules.RuleSet `json:"user_rule,omitempty"`
	Concurrent *bool          `json:"concurrent,omitempty"`
	Multiple   *bool          `json:"multiple,omitempty"`
}

// ExitStepData represents data for exit step - exits user from journey
type ExitStepData struct {
	EntranceUuid string `json:"entrance_uuid"`
}

// DelayStepData represents data for delay step - wait before proceeding
type DelayStepData struct {
	Format        DelayStepDataFormat `json:"format"`
	Minutes       *int                `json:"minutes,omitempty"`
	Hours         *int                `json:"hours,omitempty"`
	Days          *int                `json:"days,omitempty"`
	Time          *string             `json:"time,omitempty"`
	Date          *string             `json:"date,omitempty"`
	ExclusionDays *[]int              `json:"exclusion_days,omitempty"`
}

// ActionStepData represents data for action step - send campaign
type ActionStepData struct {
	CampaignId uuid.UUID `json:"campaign_id"`
}

// GateStepData represents data for gate step - conditional branching
type GateStepData struct {
	Rule rules.RuleSet `json:"rule"`
}

// ExperimentStepData represents data for experiment step - A/B testing
type ExperimentStepData struct {
	Name *string `json:"name,omitempty"`
}

// LinkStepData represents data for link step - add user to another journey
type LinkStepData struct {
	TargetId string  `json:"target_id"`
	Delay    *string `json:"delay,omitempty"`
}

// StickyStepData represents data for sticky step - placeholder for sticky logic
type StickyStepData struct{}

// BalancerStepData represents data for balancer step - rate-limited distribution
type BalancerStepData struct {
	RateLimit int    `json:"rate_limit"`
	Interval  string `json:"interval"`
}

// UpdateStepData represents data for update step - update user data
type UpdateStepData struct {
	Template string `json:"template"`
}

// EventStepData represents data for event step - trigger custom event
type EventStepData struct {
	EventName string  `json:"event_name"`
	Template  *string `json:"template,omitempty"`
}
