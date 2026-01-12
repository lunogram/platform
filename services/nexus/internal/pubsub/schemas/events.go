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
)

// Event represents a tracked event with associated user and project information.
type Event struct {
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

func (u User) Event(name string) Event {
	return Event{
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

type JourneyStep struct {
	ProjectID      uuid.UUID  `json:"project_id"`
	JourneyID      uuid.UUID  `json:"journey_id"`
	JourneyEntryID uuid.UUID  `json:"journey_entry_id"`
	VersionID      *uuid.UUID `json:"version_id,omitempty"`
	UserID         uuid.UUID  `json:"user_id"`
	ExternalStepID string     `json:"external_step_id"`
	StateID        *uuid.UUID `json:"state_id,omitempty"`
}

// EventsProjectSubject returns the NATS subject for project-specific events.
func EventsProjectSubject(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("events.projects.%s", projectID))
}

// EventsSchemaSubject returns the NATS subject for event schema updates.
func EventsSchemaSubject(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("events.schemas.%s", projectID))
}

// UsersProjectSubject returns the NATS subject for project-specific user events.
func UsersProjectSubject(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.projects.%s", projectID))
}

// UsersSchemaSubject returns the NATS subject for user schema updates.
func UsersSchemaSubject(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("users.schemas.%s", projectID))
}

// RecomputeListSubject returns the NATS subject for list recomputation.
func RecomputeListSubject(projectID uuid.UUID, listID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("recompute.lists.%s.%s", projectID, listID))
}

// JourneyStepSubject returns the NATS subject for journey step processing.
func JourneyStepSubject(projectID uuid.UUID, journeyID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("journeys.state.%s.%s", projectID, journeyID))
}
