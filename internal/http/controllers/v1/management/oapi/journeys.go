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

// TriggerKind selects which entrance trigger sub-object is active.
type TriggerKind string

const (
	// TriggerManual is an API/manually triggered entrance: the user is entered
	// out-of-band (e.g. via the trigger endpoint) and no trigger sub-object is
	// set.
	TriggerManual    TriggerKind = "none"
	TriggerEvent     TriggerKind = "event"
	TriggerScheduled TriggerKind = "scheduled"
	TriggerList      TriggerKind = "list"
)

// ListDirection is the membership change that fires a list trigger.
type ListDirection string

const (
	ListJoins  ListDirection = "joins"
	ListLeaves ListDirection = "leaves"
)

// EntranceStepData is the entry point into a journey.
//
// It is a tagged union: Trigger selects the kind and exactly the matching
// sub-object (Event, Scheduled or List) is populated. Each sub-object carries
// only the fields meaningful for that kind, so trigger-specific requirements
// live in the type system rather than in documentation. Concurrent and Multiple
// apply to every kind. Call Validate to enforce the union invariant.
type EntranceStepData struct {
	Trigger TriggerKind `json:"trigger"`

	Event     *EventTrigger     `json:"event,omitempty"`
	Scheduled *ScheduledTrigger `json:"scheduled,omitempty"`
	List      *ListTrigger      `json:"list,omitempty"`

	// Concurrent allows a user to be in more than one concurrent run of the
	// journey; Multiple allows re-entry after a previous run has completed.
	Concurrent bool `json:"concurrent,omitempty"`
	Multiple   bool `json:"multiple,omitempty"`
}

// EventTrigger enters the user when a custom event named Name is received.
type EventTrigger struct {
	Name string `json:"name"`
	// Rule is an optional condition evaluated against the event data; the user
	// only enters when it matches.
	Rule *rules.RuleSet `json:"rule,omitempty"`
	// UserRule filters which organization members are enrolled when the event
	// is organization-level.
	UserRule *rules.RuleSet `json:"user_rule,omitempty"`
}

// ScheduledTrigger enters the user when a schedule offset fires.
type ScheduledTrigger struct {
	Name     string         `json:"name"`
	OffsetID *uuid.UUID     `json:"offset_id,omitempty"`
	Rule     *rules.RuleSet `json:"rule,omitempty"`
	UserRule *rules.RuleSet `json:"user_rule,omitempty"`
}

// ListTrigger enters the user when they join or leave the referenced list.
type ListTrigger struct {
	ID uuid.UUID `json:"id"`
	// Direction is "joins" (default) or "leaves".
	Direction ListDirection `json:"direction,omitempty"`
	// ExitOnLeave exits the user from the journey when they leave the list. It
	// is only meaningful for the "joins" direction.
	ExitOnLeave bool `json:"exit_on_leave,omitempty"`
}

// EntranceRule returns the optional entrance condition for the active trigger,
// evaluated against the triggering event's data. It is nil when the trigger
// carries none (list triggers are matched in the backend instead).
func (e EntranceStepData) EntranceRule() *rules.RuleSet {
	switch e.Trigger {
	case TriggerEvent:
		if e.Event != nil {
			return e.Event.Rule
		}
	case TriggerScheduled:
		if e.Scheduled != nil {
			return e.Scheduled.Rule
		}
	}
	return nil
}

// MemberRule returns the optional organization-member filter for the active
// trigger, applied when enrolling members on an organization-level event.
func (e EntranceStepData) MemberRule() *rules.RuleSet {
	switch e.Trigger {
	case TriggerEvent:
		if e.Event != nil {
			return e.Event.UserRule
		}
	case TriggerScheduled:
		if e.Scheduled != nil {
			return e.Scheduled.UserRule
		}
	}
	return nil
}

// Validate enforces the entrance union invariant: Trigger must be a known kind
// and exactly the matching sub-object must be populated with its required
// fields.
func (e EntranceStepData) Validate() error {
	var present []TriggerKind
	if e.Event != nil {
		present = append(present, TriggerEvent)
	}
	if e.Scheduled != nil {
		present = append(present, TriggerScheduled)
	}
	if e.List != nil {
		present = append(present, TriggerList)
	}

	switch e.Trigger {
	case TriggerManual:
		// API/manual entrances carry no trigger block.
		if len(present) != 0 {
			return fmt.Errorf("entrance: manual trigger must not set a trigger block, got %v", present)
		}
		return nil
	case TriggerEvent, TriggerScheduled, TriggerList:
	default:
		return fmt.Errorf("entrance: unknown trigger %q", e.Trigger)
	}

	if len(present) != 1 || present[0] != e.Trigger {
		return fmt.Errorf("entrance: trigger %q requires exactly its own data block, got %v", e.Trigger, present)
	}

	switch e.Trigger {
	case TriggerEvent:
		if e.Event.Name == "" {
			return fmt.Errorf("entrance: event trigger requires event.name")
		}
	case TriggerScheduled:
		if e.Scheduled.Name == "" {
			return fmt.Errorf("entrance: scheduled trigger requires scheduled.name")
		}
	case TriggerList:
		if e.List.ID == uuid.Nil {
			return fmt.Errorf("entrance: list trigger requires list.id")
		}
		switch e.List.Direction {
		case "", ListJoins, ListLeaves:
		default:
			return fmt.Errorf("entrance: list trigger has invalid direction %q", e.List.Direction)
		}
	}

	return nil
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
