package oapi

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rules"
)

// StringOrInt is a JSON type that accepts both numbers and strings.
// It unmarshals JSON numbers (e.g. 5) and strings (e.g. "5" or "{{ var }}")
// into a Go string, providing backward compatibility with existing integer
// fields while also supporting Liquid template expressions.
type StringOrInt string

func (s *StringOrInt) UnmarshalJSON(data []byte) error {
	// Try string first
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*s = StringOrInt(str)
		return nil
	}

	// Try number
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		*s = StringOrInt(strconv.Itoa(int(num)))
		return nil
	}

	return fmt.Errorf("StringOrInt: cannot unmarshal %s", string(data))
}

func (s StringOrInt) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

func (s StringOrInt) String() string {
	return string(s)
}

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
	Trigger          *string        `json:"trigger,omitempty"`
	EventName        *string        `json:"event_name,omitempty"`
	ScheduleOffsetID *uuid.UUID     `json:"schedule_offset_id,omitempty"`
	Rule             *rules.RuleSet `json:"rule,omitempty"`
	UserRule         *rules.RuleSet `json:"user_rule,omitempty"`
	Concurrent       *bool          `json:"concurrent,omitempty"`
	Multiple         *bool          `json:"multiple,omitempty"`

	// List trigger ("list"): the user enters the journey when they join (or
	// leave) the referenced list. ListDirection is "joins" (default) or
	// "leaves". When ExitOnLeave is true (only meaningful for the "joins"
	// direction) the user is exited from the journey when they leave the list.
	ListID        *uuid.UUID `json:"list_id,omitempty"`
	ListDirection *string    `json:"list_direction,omitempty"`
	ExitOnLeave   *bool      `json:"exit_on_leave,omitempty"`
}

// ExitStepData represents data for exit step - exits user from journey
type ExitStepData struct {
	EntranceUuid string `json:"entrance_uuid"`
}

// DelayStepData represents data for delay step - wait before proceeding.
// The Minutes, Hours, Days and Time fields use StringOrInt so they accept
// both legacy integer/string JSON values and Liquid template expressions.
type DelayStepData struct {
	Format        DelayStepDataFormat `json:"format"`
	Minutes       *StringOrInt        `json:"minutes,omitempty"`
	Hours         *StringOrInt        `json:"hours,omitempty"`
	Days          *StringOrInt        `json:"days,omitempty"`
	Time          *StringOrInt        `json:"time,omitempty"`
	Date          *string             `json:"date,omitempty"`
	ExclusionDays *[]int              `json:"exclusion_days,omitempty"`
}

// CampaignStepData represents data for campaign step - send campaign
type CampaignStepData struct {
	CampaignId uuid.UUID         `json:"campaign_id" yaml:"campaign_id"`
	Data       map[string]string `json:"data,omitempty" yaml:"data,omitempty"`
}

// ActionStepData represents data for action step - execute WASM action
type ActionStepData struct {
	ActionId   uuid.UUID       `json:"action_id"`
	FunctionId string          `json:"function_id"`
	Input      json.RawMessage `json:"input,omitempty"`
}

// GateStepData represents data for gate step - conditional branching
type GateStepData struct {
	Rule rules.RuleSet `json:"rule"`
}

// ExperimentStepData represents data for experiment step - A/B testing
type ExperimentStepData struct {
	Name *string `json:"name,omitempty"`
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

// ScheduleStepData represents data for schedule step - assign user to a schedule
type ScheduleStepData struct {
	ScheduleName string  `json:"schedule_name"`
	ScheduledAt  *string `json:"scheduled_at,omitempty"` // Liquid template rendering to ISO 8601 timestamp
	Interval     *string `json:"interval,omitempty"`     // Liquid template rendering to PG interval (e.g. "1 month")
	StartAt      *string `json:"start_at,omitempty"`     // Liquid template rendering to ISO 8601 timestamp
	Template     *string `json:"template,omitempty"`     // Liquid template rendering to JSON data payload
}
