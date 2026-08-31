package schemas

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
)

// Subject represents a NATS subject for publishing messages.
type Subject string

const (
	EventUserCreated     = "user.created"
	EventUserUpdated     = "user.updated"
	EventListUserAdded   = "list.user.added"
	EventListUserRemoved = "list.user.removed"

	EventOrganizationCreated     = "organization.created"
	EventOrganizationUpdated     = "organization.updated"
	EventOrganizationUserAdded   = "organization.user.added"
	EventOrganizationUserUpdated = "organization.user.updated"
	EventOrganizationUserRemoved = "organization.user.removed"
	EventInboxMessageCreated     = "inbox.message.created"
	EventInboxMessageRead        = "inbox.message.read"
	EventInboxMessageArchived    = "inbox.message.archived"
	EventInboxMessageUnarchived  = "inbox.message.unarchived"
	EventInboxMessageUnread      = "inbox.message.unread"
	EventInboxMessageSent        = "inbox.message.sent"
	// EventInboxMessageFailed is the terminal counterpart of
	// EventInboxMessageSent: the message will never be delivered and nothing
	// further will happen to it.
	EventInboxMessageFailed = "inbox.message.failed"

	EventProjectCreated = "project.created"

	// EventJourneyStepEntered is the tracked user event emitted whenever a user
	// enters a journey step. It flows through the standard user-event pipeline so
	// it is registered, stored, and countable by rules — enabling gate steps such
	// as "continue if the user has entered step X at least N times".
	EventJourneyStepEntered = "journey.step.entered"
)

// InboxDispatchMsgID returns the deterministic JetStream Msg-Id used to dedupe
// push inbox dispatch messages. Each (messageID, providerID) pair produces a
// stable ID so fan-out retries are collapsed by the broker.
func InboxDispatchMsgID(messageID uuid.UUID, providerID uuid.UUID) string {
	return fmt.Sprintf("inbox-dispatch:%s:%s", messageID, providerID)
}

// InboxProcessMsgID returns the deterministic JetStream Msg-Id used to dedupe
// inbox.process publishes for a scheduled message. The scheduler re-scans due
// messages every tick until sent_at is set; a stable per-message ID collapses
// those repeated re-injections so a message is dispatched at most once within
// the stream's Duplicates window.
func InboxProcessMsgID(messageID uuid.UUID) string {
	return fmt.Sprintf("inbox-process:%s", messageID)
}

// JourneyStepEnteredMsgID returns the deterministic JetStream Msg-Id used to
// dedupe the journey.step.entered event for a single step entry. The source
// stream sequence of the inbound step message is stable across redeliveries but
// unique per distinct step entry (a re-entry on a journey loop is a separate
// inbound message with its own sequence), so the count stays accurate while
// redeliveries collapse within the user-events stream's Duplicates window. The
// entry and step identifiers are included for readability and namespacing.
func JourneyStepEnteredMsgID(journeyEntryID uuid.UUID, externalStepID string, sourceSeq uint64) string {
	return fmt.Sprintf("journey-step-entered:%s:%s:%d", journeyEntryID, externalStepID, sourceSeq)
}

// ExternalID is an alias for subjects.ExternalIDParam so that pubsub messages
// use the same type as the store layer without manual conversion.
type ExternalID = subjects.ExternalIDParam

// UserEvent represents a tracked event with associated user and project information.
type UserEvent struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	ProjectID   uuid.UUID      `json:"project_id"`
	UserID      uuid.UUID      `json:"user_id"`
	Identifiers []ExternalID   `json:"identifiers,omitempty"`
	Data        map[string]any `json:"data"`
}

type User struct {
	ID          uuid.UUID      `json:"id"`
	ProjectID   uuid.UUID      `json:"project_id"`
	Identifiers []ExternalID   `json:"identifiers,omitempty"`
	Email       *string        `json:"email"`
	Phone       *string        `json:"phone"`
	Timezone    *string        `json:"timezone"`
	Locale      *string        `json:"locale"`
	Data        map[string]any `json:"data"`
	Version     int32          `json:"version"`
}

func (u User) UserEvent(name string) UserEvent {
	return UserEvent{
		Name:        name,
		ProjectID:   u.ProjectID,
		Identifiers: u.Identifiers,
		Data: map[string]any{
			"id":       u.ID,
			"email":    u.Email,
			"phone":    u.Phone,
			"timezone": u.Timezone,
			"locale":   u.Locale,
			"traits":   u.Data,
			"version":  u.Version,
		},
	}
}

const (
	InboxSourceInbox     = "inbox"
	InboxSourceCampaign  = "campaign"
	InboxSourceBroadcast = "broadcast"
	InboxSourceJourney   = "journey"
)

// SendCampaignData holds provenance metadata for the campaign send.
// It mirrors the inbox convention: provenance lives in Data.
type SendCampaignData struct {
	JourneyID      *uuid.UUID `json:"journey_id,omitempty"`
	JourneyEntryID *uuid.UUID `json:"journey_entry_id,omitempty"`
	JourneyStepID  *string    `json:"journey_step_id,omitempty"`
}

type SendCampaign struct {
	ProjectID   uuid.UUID         `json:"project_id"`
	UserID      uuid.UUID         `json:"user_id"`
	CampaignID  uuid.UUID         `json:"campaign_id"`
	BroadcastID *uuid.UUID        `json:"broadcast_id,omitempty"`
	Data        *SendCampaignData `json:"data,omitempty"`
	Variables   map[string]string `json:"variables,omitempty"`
	// Variant overrides the campaign's own rule for picking a template variant.
	// Nil defers to the campaign.
	//
	// A journey step resolves its expression before publishing and sends a
	// static selector, because the journey context it renders against - entrance
	// data, earlier step state - does not exist by the time the send is
	// rendered. A broadcast passes its selector through untouched, since an
	// expression there has to run once per recipient.
	Variant *management.VariantSelector `json:"variant,omitempty"`
}

// InboxOrigin resolves the inbox source label and the external_id key used
// to dedupe campaign-driven inbox rows.
//
// Segments are appended most-specific → least-specific (journey, broadcast,
// campaign) so the first segment doubles as the source label. Every layer
// of context that is present is included in the key, making deduplication
// as precise as the available provenance allows.
//
// providerPart is the SenderIdentity UUID for email/SMS and the literal
// "multi" for push fan-outs.
func (e SendCampaign) InboxOrigin(providerPart string) (string, string) {
	var parts []string

	if e.Data != nil && e.Data.JourneyID != nil && e.Data.JourneyEntryID != nil && e.Data.JourneyStepID != nil {
		parts = append(parts, "journey", e.Data.JourneyEntryID.String(), *e.Data.JourneyStepID)
	}

	if e.BroadcastID != nil {
		parts = append(parts, "broadcast", e.BroadcastID.String())
	}

	parts = append(parts, "campaign", e.CampaignID.String(), "user", e.UserID.String(), "provider", providerPart)

	return parts[0], strings.Join(parts, ":")
}

type JourneyStep struct {
	ProjectID      uuid.UUID  `json:"project_id"`
	JourneyID      uuid.UUID  `json:"journey_id"`
	JourneyEntryID uuid.UUID  `json:"journey_entry_id"`
	VersionID      *uuid.UUID `json:"version_id,omitempty"`
	UserID         uuid.UUID  `json:"user_id"`
	ExternalStepID string     `json:"external_step_id"`
	StepType       string     `json:"step_type"`
	StateID        *uuid.UUID `json:"state_id,omitempty"`
}

// JourneyStepEntered builds the tracked user event emitted when a user enters a
// journey step. It is published as a regular UserEvent (to UserEventsProcess) so
// it is registered, stored in user_events, and countable by the rules engine —
// letting gate steps reason about how often a user has reached a given step. The
// step descriptor lives under Data alongside the journey and entry identifiers so
// gate rules can filter on data.journey_id, data.step_id, etc.
func JourneyStepEntered(projectID, journeyID, journeyEntryID, userID uuid.UUID, versionID *uuid.UUID, stepID, stepType string, stepName *string) UserEvent {
	data := map[string]any{
		"journey_id":       journeyID,
		"journey_entry_id": journeyEntryID,
		"step_id":          stepID,
		"step_type":        stepType,
	}
	if versionID != nil {
		data["version_id"] = *versionID
	}
	if stepName != nil {
		data["step_name"] = *stepName
	}

	return UserEvent{
		Name:      EventJourneyStepEntered,
		ProjectID: projectID,
		UserID:    userID,
		Data:      data,
	}
}

// JourneyStepExecuted is published when a user completes their final step in a journey.
type JourneyStepExecuted struct {
	ProjectID      uuid.UUID `json:"project_id"`
	JourneyID      uuid.UUID `json:"journey_id"`
	JourneyEntryID uuid.UUID `json:"journey_entry_id"`
	UserID         uuid.UUID `json:"user_id"`
	ExternalStepID string    `json:"external_step_id"`
	StepType       string    `json:"step_type"`
}

// JourneyEntrance is published when an event matches a journey entrance rule.
// A dedicated handler performs the eligibility check, creates the initial
// journey state, and advances the user into the first journey step(s).
type JourneyEntrance struct {
	ProjectID      uuid.UUID       `json:"project_id"`
	JourneyID      uuid.UUID       `json:"journey_id"`
	VersionID      uuid.UUID       `json:"version_id"`
	UserID         uuid.UUID       `json:"user_id"`
	ExternalStepID string          `json:"external_step_id"`
	Multiple       bool            `json:"multiple"`
	Concurrent     bool            `json:"concurrent"`
	Data           map[string]any  `json:"data"`
	Children       json.RawMessage `json:"children"`
}

// Organization represents an organization with associated project information.
type Organization struct {
	ID          uuid.UUID      `json:"id"`
	ProjectID   uuid.UUID      `json:"project_id"`
	Identifiers []ExternalID   `json:"identifiers,omitempty"`
	Name        *string        `json:"name"`
	Data        map[string]any `json:"data"`
	Version     int32          `json:"version"`
}

// OrganizationEvent creates an OrganizationEvent from this Organization with the given event name.
func (o Organization) OrganizationEvent(name string) OrganizationEvent {
	return OrganizationEvent{
		Name:                    name,
		ProjectID:               o.ProjectID,
		OrganizationID:          o.ID,
		OrganizationIdentifiers: o.Identifiers,
		Data: map[string]any{
			"id":          o.ID,
			"identifiers": o.Identifiers,
			"name":        o.Name,
			"traits":      o.Data,
			"version":     o.Version,
		},
	}
}

// OrganizationUser represents a user's membership in an organization.
type OrganizationUser struct {
	OrganizationID          uuid.UUID      `json:"organization_id"`
	OrganizationIdentifiers []ExternalID   `json:"organization_identifiers,omitempty"`
	UserID                  uuid.UUID      `json:"user_id"`
	ProjectID               uuid.UUID      `json:"project_id"`
	Data                    map[string]any `json:"data"`
	Version                 int32          `json:"version"`
}

// OrganizationEvent creates an OrganizationEvent from this OrganizationUser with the given event name.
func (ou OrganizationUser) OrganizationEvent(name string) OrganizationEvent {
	return OrganizationEvent{
		Name:                    name,
		ProjectID:               ou.ProjectID,
		OrganizationID:          ou.OrganizationID,
		OrganizationIdentifiers: ou.OrganizationIdentifiers,
		Data: map[string]any{
			"organization_id":          ou.OrganizationID,
			"organization_identifiers": ou.OrganizationIdentifiers,
			"user_id":                  ou.UserID,
			"traits":                   ou.Data,
			"version":                  ou.Version,
		},
	}
}

// OrganizationEvent represents an event that occurs on an organization (not a user).
type OrganizationEvent struct {
	ID                      uuid.UUID      `json:"id"`
	Name                    string         `json:"name"`
	ProjectID               uuid.UUID      `json:"project_id"`
	OrganizationID          uuid.UUID      `json:"organization_id"`
	OrganizationIdentifiers []ExternalID   `json:"organization_identifiers,omitempty"`
	Data                    map[string]any `json:"data"`
}

// InboxMessage is the wire shape published on users.inbox.process /
// organizations.inbox.process when a row is created. It mirrors the
// InboxMessage row: render output (title, body, format, link_url, ...)
// lives in Content; provenance (template_id, journey_*) lives in Data.
type InboxMessage struct {
	ProjectID        uuid.UUID       `json:"project_id"`
	MessageID        uuid.UUID       `json:"message_id,omitempty"`
	SubjectID        uuid.UUID       `json:"subject_id,omitempty"`
	Identifiers      []ExternalID    `json:"identifiers,omitempty"`
	ExternalID       *string         `json:"external_id,omitempty"`
	Channel          string          `json:"channel"`
	SenderIdentityID *uuid.UUID      `json:"sender_identity_id,omitempty"`
	CampaignID       *uuid.UUID      `json:"campaign_id,omitempty"`
	BroadcastID      *uuid.UUID      `json:"broadcast_id,omitempty"`
	Content          json.RawMessage `json:"content,omitempty"`
	Data             json.RawMessage `json:"data,omitempty"`
	Tags             []string        `json:"tags,omitempty"`
	Priority         *int16          `json:"priority,omitempty"`
	Source           *string         `json:"source,omitempty"`
	ScheduledAt      *time.Time      `json:"scheduled_at,omitempty"`
	ExpiresAt        *time.Time      `json:"expires_at,omitempty"`
}

// InboxPushDispatch is the wire shape published on
// {users,organizations}.inbox.dispatch.<projectID> when a push inbox
// message fans out to its per-provider dispatches. Each provider gets its
// own NATS message so that a single provider failure only retries that
// provider, not the whole fan-out. The Msg-Id is
// "inbox-dispatch:<inbox_message_id>:<provider_id>" so JetStream
// deduplicates retries from the upstream inbox handler.
type InboxPushDispatch struct {
	ProjectID      uuid.UUID       `json:"project_id"`
	InboxMessageID uuid.UUID       `json:"inbox_message_id"`
	ProviderID     uuid.UUID       `json:"provider_id"`
	Scope          string          `json:"scope"` // "user" or "organization"
	Payload        json.RawMessage `json:"payload"`
}

// InboxStateEvent is the wire payload for an inbox state-change command
// (opened, archived, ...). The specific lifecycle action is encoded by the
// NATS subject the message is published on (UserInboxRead,
// UserInboxArchived, OrganizationInboxRead, ...), not by a field on the
// payload, so each action gets its own consumer with its own retry policy
// and broker-side filtering.
type InboxStateEvent struct {
	ProjectID   uuid.UUID    `json:"project_id"`
	MessageID   uuid.UUID    `json:"message_id"`
	SubjectID   uuid.UUID    `json:"subject_id,omitempty"`
	Identifiers []ExternalID `json:"identifiers,omitempty"`
}

// ProcessBroadcast represents a request to process a broadcast send.
type ProcessBroadcast struct {
	ProjectID   uuid.UUID `json:"project_id"`
	BroadcastID uuid.UUID `json:"broadcast_id"`
}

// ProcessBroadcastBatch represents a single batch of users to fan out during
// broadcast processing. Each batch publishes SendCampaign messages for a page
// of list users and then chains the next batch until the list is exhausted.
type ProcessBroadcastBatch struct {
	ProjectID   uuid.UUID `json:"project_id"`
	BroadcastID uuid.UUID `json:"broadcast_id"`
	Offset      int       `json:"offset"`
	BatchSize   int       `json:"batch_size"`
	Processed   int       `json:"processed"` // running total from prior batches
}

// BroadcastsProcess returns the NATS subject for broadcast processing.
func BroadcastsProcess(projectID, broadcastID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("broadcasts.process.%s.%s", projectID, broadcastID))
}

// BroadcastsBatch returns the NATS subject for broadcast batch processing.
func BroadcastsBatch(projectID, broadcastID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("broadcasts.batch.%s.%s", projectID, broadcastID))
}

// CampaignsSend returns the NATS subject for campaign sending.
func CampaignsSend(projectID uuid.UUID, campaignID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("campaigns.send.%s.%s", projectID, campaignID))
}

// UsersProcess returns the NATS subject for user processing.
func UsersProcess(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.process.%s", projectID))
}

// UsersSchema returns the NATS subject for user schema updates.
func UsersSchema(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.schema.%s", projectID))
}

// UserInboxProcess returns the NATS subject for user inbox message creation.
func UserInboxProcess(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.inbox.process.%s", projectID))
}

// UserInboxRead returns the NATS subject for user inbox "read" state
// commands. Each lifecycle action gets its own subject (and consumer) so
// that producers and brokers can route by action without payload
// introspection.
func UserInboxRead(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.inbox.read.%s", projectID))
}

// UserInboxArchived returns the NATS subject for user inbox "archived"
// state commands.
func UserInboxArchived(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.inbox.archived.%s", projectID))
}

// UserInboxSent returns the NATS subject for user inbox "sent" completion
// events, emitted after all providers have been dispatched.
func UserInboxSent(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.inbox.sent.%s", projectID))
}

// UserInboxFailed returns the NATS subject for user inbox "failed" terminal
// events, emitted when a message will never be delivered.
func UserInboxFailed(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.inbox.failed.%s", projectID))
}

// UserInboxDispatch returns the NATS subject for per-provider push inbox
// dispatch fan-out (user scope).
func UserInboxDispatch(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.inbox.dispatch.%s", projectID))
}

// UserEventsProcess returns the NATS subject for user event processing.
func UserEventsProcess(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.events.process.%s", projectID))
}

// UserEventsSchema returns the NATS subject for user event schema updates.
func UserEventsSchema(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.events.schema.%s", projectID))
}

// ListsRecompute returns the NATS subject for list recomputation.
func ListsRecompute(projectID uuid.UUID, listID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("lists.recompute.%s.%s", projectID, listID))
}

// JourneysAdvance returns the NATS subject for journey advancement.
func JourneysAdvance(projectID uuid.UUID, journeyID uuid.UUID, userID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("journeys.advance.%s.%s.%s", projectID, journeyID, userID))
}

// JourneysStepExecuted returns the NATS subject for journey step execution notifications.
func JourneysStepExecuted(projectID uuid.UUID, journeyID uuid.UUID, userID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("journeys.step_executed.%s.%s.%s", projectID, journeyID, userID))
}

// JourneysEntrance returns the NATS subject for journey entrance processing.
func JourneysEntrance(projectID uuid.UUID, journeyID uuid.UUID, userID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("journeys.entrance.%s.%s.%s", projectID, journeyID, userID))
}

// OrganizationsProcess returns the NATS subject for organization processing.
func OrganizationsProcess(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.process.%s", projectID))
}

// OrganizationsSchema returns the NATS subject for organization schema updates.
func OrganizationsSchema(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.schema.%s", projectID))
}

// OrganizationInboxProcess returns the NATS subject for organization inbox message creation.
func OrganizationInboxProcess(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.inbox.process.%s", projectID))
}

// OrganizationInboxRead returns the NATS subject for organization inbox
// "read" state commands.
func OrganizationInboxRead(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.inbox.read.%s", projectID))
}

// OrganizationInboxArchived returns the NATS subject for organization
// inbox "archived" state commands.
func OrganizationInboxArchived(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.inbox.archived.%s", projectID))
}

// OrganizationInboxSent returns the NATS subject for organization inbox
// "sent" completion events, emitted after all providers have been dispatched.
func OrganizationInboxSent(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.inbox.sent.%s", projectID))
}

// OrganizationInboxFailed returns the NATS subject for organization inbox
// "failed" terminal events, emitted when a message will never be delivered.
func OrganizationInboxFailed(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.inbox.failed.%s", projectID))
}

// OrganizationInboxDispatch returns the NATS subject for per-provider push
// inbox dispatch fan-out (organization scope).
func OrganizationInboxDispatch(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.inbox.dispatch.%s", projectID))
}

// OrganizationUsersProcess returns the NATS subject for organization user processing.
func OrganizationUsersProcess(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.users.process.%s", projectID))
}

// OrganizationUsersSchema returns the NATS subject for organization user schema updates.
func OrganizationUsersSchema(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.users.schema.%s", projectID))
}

// OrganizationEventsProcess returns the NATS subject for organization event processing.
func OrganizationEventsProcess(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.events.process.%s", projectID))
}

// OrganizationEventsSchema returns the NATS subject for organization event schema updates.
func OrganizationEventsSchema(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.events.schema.%s", projectID))
}

// MatchUserEvent is published when a user event uses a JSONB match filter
// instead of explicit identifiers. The consumer resolves matching users and
// publishes individual UserEvent messages for each match.
type MatchUserEvent struct {
	Name      string         `json:"name"`
	ProjectID uuid.UUID      `json:"project_id"`
	Match     map[string]any `json:"match"`
	Data      map[string]any `json:"data"`
}

// MatchOrganizationEvent is published when an organization event uses a JSONB
// match filter instead of explicit identifiers. The consumer resolves matching
// organizations and publishes individual OrganizationEvent messages for each.
type MatchOrganizationEvent struct {
	Name      string         `json:"name"`
	ProjectID uuid.UUID      `json:"project_id"`
	Match     map[string]any `json:"match"`
	Data      map[string]any `json:"data"`
}

// UserEventsMatch returns the NATS subject for user event match/fan-out processing.
func UserEventsMatch(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.events.match.%s", projectID))
}

// OrganizationEventsMatch returns the NATS subject for organization event match/fan-out processing.
func OrganizationEventsMatch(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.events.match.%s", projectID))
}

// ExecuteAction represents a request to execute an action via NATS.
type ExecuteAction struct {
	ProjectID  uuid.UUID      `json:"project_id"`
	ActionID   uuid.UUID      `json:"action_id"`
	Type       string         `json:"type"`
	FunctionID string         `json:"function_id"`
	Config     map[string]any `json:"config"`
	Input      any            `json:"input,omitempty"`
}

// ExecuteActionResponse is the reply sent back through the NATS inbox.
type ExecuteActionResponse struct {
	StatusCode int            `json:"status_code"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// ActionsExecute returns the NATS subject for action execution requests.
func ActionsExecute(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("actions.execute.%s", projectID))
}

// ValidateAction represents a request to validate an action's configuration via NATS.
type ValidateAction struct {
	ProjectID uuid.UUID      `json:"project_id"`
	Type      string         `json:"type"`
	Config    map[string]any `json:"config"`
}

// ValidateActionResponse is the reply sent back through the NATS inbox.
type ValidateActionResponse struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message,omitempty"`
}

// ActionsValidate returns the NATS subject for action validation requests.
func ActionsValidate(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("actions.validate.%s", projectID))
}

// ActionSchema represents an action execution result for schema extraction.
type ActionSchema struct {
	ProjectID  uuid.UUID      `json:"project_id"`
	ActionID   uuid.UUID      `json:"action_id"`
	FunctionID string         `json:"function_id"`
	Metadata   map[string]any `json:"metadata"`
}

// ActionsSchema returns the NATS subject for action schema updates.
func ActionsSchema(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("actions.schema.%s", projectID))
}

// ProjectEvent represents an event that occurs on a project.
type ProjectEvent struct {
	ID             uuid.UUID      `json:"id"`
	Name           string         `json:"name"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	Data           map[string]any `json:"data"`
}

// ProjectEventsProcess returns the NATS subject for project event processing.
func ProjectEventsProcess(organizationID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("projects.events.%s", organizationID))
}

// CompileEmail represents a request to compile a React Email JSX template via NATS.
type CompileEmail struct {
	Source string `json:"source"`
}

// CompileEmailResponse is the reply sent back through the NATS inbox.
type CompileEmailResponse struct {
	CompiledJS string `json:"compiled_js"`
	Error      string `json:"error,omitempty"`
}

// EmailCompile returns the NATS subject for email compilation requests.
func EmailCompile(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("email.compile.%s", projectID))
}

// RenderEmail represents a request to render a pre-compiled email template via NATS.
type RenderEmail struct {
	CompiledJS string         `json:"compiled_js"`
	Props      map[string]any `json:"props"`
}

// RenderEmailResponse is the reply sent back through the NATS inbox.
type RenderEmailResponse struct {
	HTML      string `json:"html"`
	PlainText string `json:"plain_text"`
	Error     string `json:"error,omitempty"`
}

// EmailRender returns the NATS subject for email rendering requests.
func EmailRender(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("email.render.%s", projectID))
}

// ProviderWebhookEvent represents a delivery event received from a provider webhook.
type ProviderWebhookEvent struct {
	ProjectID  uuid.UUID `json:"project_id"`
	ProviderID uuid.UUID `json:"provider_id"`
	Module     string    `json:"module"`
	Channel    string    `json:"channel"`
	EventName  string    `json:"event_name"`
	MessageID  string    `json:"reference_id"`
	// InboxMessageID is the platform inbox-message UUID echoed back from the
	// provider via its native custom-metadata mechanism. Zero value means the
	// originating send did not propagate the metadata; consumers must fall
	// back to MessageID-based correlation in that case.
	InboxMessageID uuid.UUID      `json:"inbox_message_id,omitempty"`
	UserID         uuid.UUID      `json:"user_id,omitempty"` // Resolved downstream, zero if unknown
	Timestamp      string         `json:"timestamp,omitempty"`
	Data           map[string]any `json:"data,omitempty"`
}

// ProvidersWebhook returns the NATS subject for provider webhook events.
func ProvidersWebhook(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("providers.webhooks.%s", projectID))
}

// ScheduledProcess returns the NATS subject for scheduled processing (both user and organization).
func ScheduledProcess(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("scheduled.process.%s", projectID))
}

// ScheduledSchema returns the NATS subject for scheduled schema updates (both user and organization).
func ScheduledSchema(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("scheduled.schema.%s", projectID))
}

// ScheduledBackfill returns the NATS subject for backfilling scheduled events
// when a new offset is created on an existing schedule definition.
func ScheduledBackfill(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("scheduled.backfill.%s", projectID))
}

// ScheduleOffsetBackfillMsg is published when a new schedule offset is created
// so that a consumer can asynchronously backfill user_scheduled_events and
// organization_scheduled_events for all existing schedule assignments.
type ScheduleOffsetBackfillMsg struct {
	ProjectID  uuid.UUID `json:"project_id"`
	ScheduleID uuid.UUID `json:"schedule_id"`
	OffsetID   uuid.UUID `json:"offset_id"`
	Offset     string    `json:"offset"`
	Direction  string    `json:"direction"`
}

// ScheduledMsg represents a scheduled event for a user or organization.
type ScheduledMsg struct {
	ID             uuid.UUID      `json:"id"`
	ProjectID      uuid.UUID      `json:"project_id"`
	ScheduledID    uuid.UUID      `json:"scheduled_id"`
	AssignmentID   uuid.UUID      `json:"assignment_id,omitempty"` // the per-subject assignment (user_schedules/organization_schedules) id; zero means create a new assignment
	Name           string         `json:"name"`
	Type           string         `json:"type"`         // "single" or "recurring"
	SubjectType    string         `json:"subject_type"` // "user" or "organization"
	ScheduledAt    time.Time      `json:"scheduled_at"`
	StartAt        *time.Time     `json:"start_at,omitempty"`
	Interval       *string        `json:"interval,omitempty"`
	Data           map[string]any `json:"data,omitempty"`
	UserID         uuid.UUID      `json:"user_id,omitempty"`
	OrganizationID uuid.UUID      `json:"organization_id,omitempty"`
	Identifiers    []ExternalID   `json:"identifiers,omitempty"`
}
