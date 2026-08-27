package rules

import (
	"time"

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
	RuleGroupParent            RuleGroup = "parent"
	RuleGroupUser              RuleGroup = "user"
	RuleGroupEvent             RuleGroup = "event"
	RuleGroupOrganization      RuleGroup = "organization"
	RuleGroupOrganizationUser  RuleGroup = "organization_user"
	RuleGroupOrganizationEvent RuleGroup = "organization_event"
	RuleGroupJourney           RuleGroup = "journey"
	RuleGroupJourneyStep       RuleGroup = "journey_step"
)

// StepScope defines which journey runs a journey_step rule counts visits over.
type StepScope string

const (
	// StepScopeEntry counts visits made during the user's current run through
	// the journey.
	StepScopeEntry StepScope = "entry"
	// StepScopeJourney counts visits across every run the user made through the
	// journey.
	StepScopeJourney StepScope = "journey"
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
	PeriodTypeRolling      PeriodType = "rolling"
	PeriodTypeAbsolute     PeriodType = "absolute"
	PeriodTypeSinceEntered PeriodType = "since_entered"
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

// RecomputeInterval returns the recommended recomputation interval for a
// rolling period with this unit. The intervals are chosen so that lists are
// recomputed frequently enough to keep membership reasonably fresh without
// being wasteful.
func (unit PeriodUnit) RecomputeInterval() time.Duration {
	switch unit {
	case PeriodUnitMinute:
		return time.Minute
	case PeriodUnitHour:
		return 5 * time.Minute
	case PeriodUnitDay:
		return time.Hour
	case PeriodUnitWeek:
		return 6 * time.Hour
	case PeriodUnitMonth:
		return 24 * time.Hour
	case PeriodUnitYear:
		return 7 * 24 * time.Hour
	default:
		return time.Hour
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

// UserMatchType defines how to match users in organization event rules
type UserMatchType string

const (
	// UserMatchAll includes all members of organizations matching the event criteria
	UserMatchAll UserMatchType = "all"
	// UserMatchConditions includes only members matching specific property conditions
	UserMatchConditions UserMatchType = "conditions"
)

// UserMatch defines how to match users within organizations for organization event rules
type UserMatch struct {
	// Type defines how to select users: "all" or "conditions"
	Type UserMatchType `json:"type"`
	// MemberConditions defines filter rules for organization user properties (when Type is "conditions")
	MemberConditions *Rule `json:"member_conditions,omitempty"`
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
	// UserMatch defines how to match users for organization event rules
	UserMatch *UserMatch `json:"user_match,omitempty"`
	// StepScope selects which journey runs a journey_step rule counts visits
	// over. It is ignored by every other group.
	StepScope *StepScope `json:"step_scope,omitempty"`
}

func (r Rule) IsWrapper() bool {
	return r.Type == RuleTypeWrapper
}

// Scope returns the scope a journey_step rule counts visits over, defaulting to
// the current journey run.
func (r Rule) Scope() StepScope {
	if r.StepScope != nil && *r.StepScope == StepScopeJourney {
		return StepScopeJourney
	}
	return StepScopeEntry
}

func (r Rule) UserEvents() (result []string) {
	if r.Group == RuleGroupEvent && r.Type == RuleTypeWrapper {
		if v, ok := r.Value.(string); ok {
			result = append(result, v)
		}
	}

	for _, child := range r.Children {
		events := child.UserEvents()
		if len(events) > 0 {
			result = append(result, events...)
		}
	}

	return result
}

func (r Rule) OrganizationEvents() (result []string) {
	if r.Group == RuleGroupOrganizationEvent && r.Type == RuleTypeWrapper {
		if v, ok := r.Value.(string); ok {
			result = append(result, v)
		}
	}

	for _, child := range r.Children {
		events := child.OrganizationEvents()
		if len(events) > 0 {
			result = append(result, events...)
		}
	}

	return result
}

func (r Rule) DependsOnEvents() bool {
	if r.Group == RuleGroupEvent {
		return true
	}

	for _, child := range r.Children {
		if child.DependsOnEvents() {
			return true
		}
	}

	return false
}

func (r Rule) DependsOnUsers() bool {
	if r.Group == RuleGroupUser {
		return true
	}

	// A parent wrapper with no children matches all users, so it depends on
	// user data (any newly created user should appear in the list).
	if r.Type == RuleTypeWrapper && r.Group == RuleGroupParent && len(r.Children) == 0 {
		return true
	}

	for _, child := range r.Children {
		if child.DependsOnUsers() {
			return true
		}
	}

	return false
}

func (r Rule) DependsOnOrganizations() bool {
	if r.Group == RuleGroupOrganization {
		return true
	}

	for _, child := range r.Children {
		if child.DependsOnOrganizations() {
			return true
		}
	}

	return false
}

func (r Rule) DependsOnOrganizationUsers() bool {
	if r.Group == RuleGroupOrganizationUser {
		return true
	}

	// If this rule has member conditions in UserMatch, it depends on organization_users
	// data regardless of the group field (frontend sends group="user" for member conditions
	// but the query builder correctly queries the organization_users table).
	if r.UserMatch != nil && r.UserMatch.Type == UserMatchConditions && r.UserMatch.MemberConditions != nil {
		return true
	}

	for _, child := range r.Children {
		if child.DependsOnOrganizationUsers() {
			return true
		}
	}

	return false
}

func (r Rule) DependsOnJourneySteps() bool {
	if r.Group == RuleGroupJourneyStep {
		return true
	}

	for _, child := range r.Children {
		if child.DependsOnJourneySteps() {
			return true
		}
	}

	return false
}

func (r Rule) DependsOnJourney() bool {
	if r.Group == RuleGroupJourney {
		return true
	}

	for _, child := range r.Children {
		if child.DependsOnJourney() {
			return true
		}
	}

	return false
}

// DependsOnTime returns true if any rule node in the tree uses a rolling time
// period. Lists with such rules need periodic recomputation because users can
// fall out of the time window without any triggering event.
func (r Rule) DependsOnTime() bool {
	if r.Frequency != nil && r.Frequency.Period.Type == PeriodTypeRolling {
		return true
	}

	if r.UserMatch != nil && r.UserMatch.MemberConditions != nil {
		if r.UserMatch.MemberConditions.DependsOnTime() {
			return true
		}
	}

	for _, child := range r.Children {
		if child.DependsOnTime() {
			return true
		}
	}

	return false
}

// smallestRecomputeInterval recursively finds the smallest recompute interval
// across all rolling period nodes in the rule tree. Returns nil when no rolling
// periods exist.
func (r Rule) smallestRecomputeInterval() *time.Duration {
	var smallest *time.Duration

	if r.Frequency != nil && r.Frequency.Period.Type == PeriodTypeRolling {
		d := r.Frequency.Period.Unit.RecomputeInterval()
		smallest = &d
	}

	if r.UserMatch != nil && r.UserMatch.MemberConditions != nil {
		if child := r.UserMatch.MemberConditions.smallestRecomputeInterval(); child != nil {
			if smallest == nil || *child < *smallest {
				smallest = child
			}
		}
	}

	for _, child := range r.Children {
		if child := child.smallestRecomputeInterval(); child != nil {
			if smallest == nil || *child < *smallest {
				smallest = child
			}
		}
	}

	return smallest
}

// RuleSet represents the complete rule configuration
type RuleSet struct {
	Rule
}

// RecomputeInterval returns the recommended recomputation interval for the
// entire rule set. It walks the rule tree and returns the smallest tier-based
// interval across all rolling period nodes. Returns nil when the rule set
// contains no rolling periods (i.e. no time-based reconciliation needed).
func (rs RuleSet) RecomputeInterval() *time.Duration {
	return rs.Rule.smallestRecomputeInterval()
}

// HasChildren returns true if the rule has child rules
func (r *Rule) HasChildren() bool {
	return len(r.Children) > 0
}

// IsRoot returns true if the rule is a root node
func (r *Rule) IsRoot() bool {
	return r.ParentUUID == nil
}

// partition returns the subset of top level rules for which keep reports true,
// preserving the root wrapper. Returns nil when no child qualifies.
func (rs RuleSet) partition(keep func(Rule) bool) *RuleSet {
	var children []Rule
	for _, child := range rs.Children {
		if keep(child) {
			children = append(children, child)
		}
	}

	if len(children) == 0 {
		return nil
	}

	if len(children) == len(rs.Children) {
		return &rs
	}

	r := rs.Rule
	r.Children = children
	return &RuleSet{Rule: r}
}

// Local returns the subset of rules that are evaluated in-memory against
// journey data. Returns nil when no such rules exist.
func (rs RuleSet) Local() *RuleSet {
	if !rs.Rule.IsWrapper() || !rs.Rule.HasChildren() {
		return nil
	}

	return rs.partition(func(child Rule) bool {
		return child.Group == RuleGroupJourney
	})
}

// StepVisits returns the subset of rules that compare how often a user reached
// a journey step. Returns nil when no such rules exist.
func (rs RuleSet) StepVisits() *RuleSet {
	if !rs.Rule.IsWrapper() || !rs.Rule.HasChildren() {
		return nil
	}

	return rs.partition(func(child Rule) bool {
		return child.Group == RuleGroupJourneyStep
	})
}

// Historical returns the subset of rules that are evaluated via SQL against
// the database. Returns nil when no such rules exist. If the root rule is not
// a wrapper or has no children, the entire RuleSet is returned.
func (rs RuleSet) Historical() *RuleSet {
	if !rs.Rule.IsWrapper() || !rs.Rule.HasChildren() {
		return &rs
	}

	return rs.partition(func(child Rule) bool {
		return child.Group != RuleGroupJourney && child.Group != RuleGroupJourneyStep
	})
}
