package schemas

import (
	"fmt"

	"github.com/google/uuid"
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
)

// UserEvent represents a tracked event with associated user and project information.
type UserEvent struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	ProjectID   uuid.UUID      `json:"project_id"`
	UserID      uuid.UUID      `json:"user_id"`
	AnonymousId *string        `json:"anonymous_id"`
	ExternalId  *string        `json:"external_id"`
	Data        map[string]any `json:"data"`
}

type User struct {
	ID          uuid.UUID      `json:"id"`
	ProjectID   uuid.UUID      `json:"project_id"`
	AnonymousID *string        `json:"anonymous_id"`
	ExternalID  *string        `json:"external_id"`
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
		AnonymousId: u.AnonymousID,
		ExternalId:  u.ExternalID,
		Data: map[string]any{
			"id":       u.ID,
			"email":    u.Email,
			"phone":    u.Phone,
			"timezone": u.Timezone,
			"locale":   u.Locale,
			"data":     u.Data,
			"version":  u.Version,
		},
	}
}

type SendCampaign struct {
	ProjectID  uuid.UUID `json:"project_id"`
	UserID     uuid.UUID `json:"user_id"`
	CampaignID uuid.UUID `json:"campaign_id"`
}

type JourneyStep struct {
	ProjectID      uuid.UUID  `json:"project_id"`
	JourneyID      uuid.UUID  `json:"journey_id"`
	JourneyEntryID uuid.UUID  `json:"journey_entry_id"`
	VersionID      *uuid.UUID `json:"version_id,omitempty"`
	UserID         uuid.UUID  `json:"user_id"`
	ExternalStepID string     `json:"external_step_id"`
	StateID        *uuid.UUID `json:"state_id,omitempty"`
}

// Organization represents an organization with associated project information.
type Organization struct {
	ID         uuid.UUID      `json:"id"`
	ProjectID  uuid.UUID      `json:"project_id"`
	ExternalID string         `json:"external_id"`
	Name       *string        `json:"name"`
	Data       map[string]any `json:"data"`
	Version    int32          `json:"version"`
}

// OrganizationEvent creates an OrganizationEvent from this Organization with the given event name.
func (o Organization) OrganizationEvent(name string) OrganizationEvent {
	return OrganizationEvent{
		Name:                   name,
		ProjectID:              o.ProjectID,
		OrganizationID:         o.ID,
		OrganizationExternalID: o.ExternalID,
		Data: map[string]any{
			"id":          o.ID,
			"external_id": o.ExternalID,
			"name":        o.Name,
			"data":        o.Data,
			"version":     o.Version,
		},
	}
}

// OrganizationUser represents a user's membership in an organization.
type OrganizationUser struct {
	OrganizationID         uuid.UUID      `json:"organization_id"`
	OrganizationExternalID string         `json:"organization_external_id"`
	UserID                 uuid.UUID      `json:"user_id"`
	ProjectID              uuid.UUID      `json:"project_id"`
	Data                   map[string]any `json:"data"`
	Version                int32          `json:"version"`
}

// OrganizationEvent creates an OrganizationEvent from this OrganizationUser with the given event name.
func (ou OrganizationUser) OrganizationEvent(name string) OrganizationEvent {
	return OrganizationEvent{
		Name:                   name,
		ProjectID:              ou.ProjectID,
		OrganizationID:         ou.OrganizationID,
		OrganizationExternalID: ou.OrganizationExternalID,
		Data: map[string]any{
			"organization_id":          ou.OrganizationID,
			"organization_external_id": ou.OrganizationExternalID,
			"user_id":                  ou.UserID,
			"data":                     ou.Data,
			"version":                  ou.Version,
		},
	}
}

// OrganizationEvent represents an event that occurs on an organization (not a user).
type OrganizationEvent struct {
	ID                     uuid.UUID      `json:"id"`
	Name                   string         `json:"name"`
	ProjectID              uuid.UUID      `json:"project_id"`
	OrganizationID         uuid.UUID      `json:"organization_id"`
	OrganizationExternalID string         `json:"organization_external_id"`
	Data                   map[string]any `json:"data"`
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
func JourneysAdvance(projectID uuid.UUID, journeyID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("journeys.advance.%s.%s", projectID, journeyID))
}

// OrganizationsProcess returns the NATS subject for organization processing.
func OrganizationsProcess(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.process.%s", projectID))
}

// OrganizationsSchema returns the NATS subject for organization schema updates.
func OrganizationsSchema(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("organizations.schema.%s", projectID))
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

// ExecuteAction represents a request to execute an action via NATS.
type ExecuteAction struct {
	ProjectID uuid.UUID      `json:"project_id"`
	ActionID  uuid.UUID      `json:"action_id"`
	Type      string         `json:"type"`
	Config    map[string]any `json:"config"`
	Payload   any            `json:"payload,omitempty"`
	Variables map[string]any `json:"variables,omitempty"`
}

// ExecuteActionResponse is the reply sent back through the NATS inbox.
type ExecuteActionResponse struct {
	Status     string         `json:"status"`
	StatusCode *int           `json:"status_code,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// ActionsExecute returns the NATS subject for action execution requests.
func ActionsExecute(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("actions.execute.%s", projectID))
}

// ActionSchema represents an action execution result for schema extraction.
type ActionSchema struct {
	ProjectID uuid.UUID      `json:"project_id"`
	ActionID  uuid.UUID      `json:"action_id"`
	Metadata  map[string]any `json:"metadata"`
}

// ActionsSchema returns the NATS subject for action schema updates.
func ActionsSchema(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("actions.schema.%s", projectID))
}
