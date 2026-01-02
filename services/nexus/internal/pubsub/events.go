package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/rules"
	"github.com/lunogram/platform/services/nexus/internal/rules/eval"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

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

// EventsProjectSubject returns the NATS subject for project-specific events.
func EventsProjectSubject(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("events.projects.%s", projectID))
}

// EventsSchemaSubject returns the NATS subject for event schema updates.
func EventsSchemaSubject(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("events.schemas.%s", projectID))
}

// EventsHandler creates a handler that processes incoming events and stores them in the database.
func EventsHandler(logger *zap.Logger, db *sqlx.DB, pub Publisher) HandlerFunc {
	state := store.NewState(db)

	return func(ctx context.Context, msg jetstream.Msg) error {
		event := Event{}
		err := json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal event message", zap.Error(err))
			return err
		}

		tx, err := db.BeginTxx(ctx, nil)
		if err != nil {
			logger.Error("failed to begin transaction", zap.Error(err))
			return err
		}

		defer tx.Rollback() //nolint:errcheck

		events := store.NewEventsStore(tx)
		users := store.NewUsersStore(tx)

		logger.Info("incoming event", zap.String("name", event.Name), zap.Stringer("project_id", event.ProjectID))

		event.ID, err = events.UpsertEvent(ctx, event.ProjectID, event.Name)
		if err != nil {
			logger.Error("failed to upsert event", zap.Error(err))
			return err
		}

		if event.UserID == uuid.Nil {
			logger.Info("looking up user ID", zap.Stringp("external_id", event.ExternalId), zap.Stringp("anonymous_id", event.AnonymousId))

			event.UserID, err = users.LookupUserID(ctx, event.ProjectID, event.ExternalId, event.AnonymousId)
			if err != nil {
				logger.Error("failed to lookup user ID", zap.Error(err))
				return err
			}
		}

		_, err = users.InsertUserEvent(ctx, event.UserID, event.ID, event.Data)
		if err != nil {
			logger.Error("failed to insert user event", zap.Error(err))
			return err
		}

		err = tx.Commit()
		if err != nil {
			logger.Error("failed to commit transaction", zap.Error(err))
			return err
		}

		wg, ctx := errgroup.WithContext(ctx)
		wg.Go(PublishEventSchema(ctx, logger, pub, event))
		wg.Go(PublishEventListDependencies(ctx, logger, state, pub, event))
		wg.Go(PublishEventJourneyDependencies(ctx, logger, state, pub, event))

		err = wg.Wait()
		if err != nil {
			logger.Error("failed to publish dependent events", zap.Error(err))
			return err
		}

		logger.Info("event processed successfully", zap.Stringer("event_id", event.ID))
		return nil
	}
}

// PublishEventSchema returns a function that publishes the event schema to the schema subject
// if the event contains data properties.
func PublishEventSchema(ctx context.Context, logger *zap.Logger, pub Publisher, event Event) func() error {
	return func() error {
		if event.Data != nil {
			err := pub.Publish(ctx, EventsSchemaSubject(event.ProjectID), event)
			if err != nil {
				logger.Error("failed to publish event to project subject", zap.Error(err))
				return err
			}
		}

		return nil
	}
}

// PublishEventListDependencies returns a function that publishes recompute messages for all lists
// that depend on the given event through rule conditions.
func PublishEventListDependencies(ctx context.Context, logger *zap.Logger, state *store.State, pub Publisher, event Event) func() error {
	return func() error {
		lists, err := state.ListEventListDependencies(ctx, event.ID)
		if err != nil {
			logger.Error("failed to list rule event dependencies", zap.Error(err))
			return err
		}

		for _, id := range lists {
			list := RecomputeList{
				ID:        id,
				ProjectID: event.ProjectID,
			}

			err = pub.Publish(ctx, RecomputeListSubject(event.ProjectID, list.ID), list)
			if err != nil {
				logger.Error("failed to publish rule to project subject", zap.Error(err))
				return err
			}
		}

		return nil
	}
}

// PublishEventJourneyDependencies returns a function that triggers journey entrance steps
// for all journeys configured with event-based entrance conditions matching the given event.
func PublishEventJourneyDependencies(ctx context.Context, logger *zap.Logger, state *store.State, pub Publisher, event Event) func() error {
	evaluator := eval.NewEvaluator()

	return func() error {
		deps, err := state.ListEventJourneyDependencies(ctx, event.ID)
		if err != nil {
			logger.Error("failed to list rule event dependencies", zap.Error(err))
			return err
		}

		for _, dep := range deps {
			entrance := oapi.EntranceStepData{}
			if dep.Data != nil {
				err := json.Unmarshal(*dep.Data, &entrance)
				if err != nil {
					return err
				}
			}

			if entrance.Rule != nil {
				match, err := evaluator.Evaluate(*entrance.Rule, event.Data)
				if err != nil {
					logger.Error("failed to evaluate journey entrance rule", zap.Error(err))
					return err
				}

				if !match {
					continue
				}
			}

			logger.Info("triggering journey entrance step", zap.Stringer("journey_id", dep.JourneyID), zap.Stringer("step_id", dep.StepID))

			entry, err := uuid.NewRandom()
			if err != nil {
				logger.Error("failed to generate journey entry ID", zap.Error(err))
				return err
			}

			data, err := json.Marshal(event.Data)
			if err != nil {
				logger.Error("failed to marshal journey entry data", zap.Error(err))
				return err
			}

			// TODO: include support to pin to specific journey version
			now := time.Now()
			rawData := json.RawMessage(data)
			result := store.JourneyUserState{
				JourneyID:      dep.JourneyID,
				JourneyEntryID: entry,
				UserID:         event.UserID,
				ExternalStepID: dep.ExternalID,
				Data:           &rawData,
				CompletedAt:    &now,
			}

			_, err = state.CreateUserJourneyState(ctx, result)
			if err != nil {
				logger.Error("failed to create journey user state", zap.Error(err))
				return err
			}

			for _, child := range dep.Children {
				step := JourneyStep{
					JourneyID:      dep.JourneyID,
					JourneyEntryID: entry,
					ExternalStepID: child.ChildExternalID,
					UserID:         event.UserID,
				}

				err = pub.Publish(ctx, JourneyStepSubject(event.ProjectID), step)
				if err != nil {
					logger.Error("failed to publish journey state to project subject", zap.Error(err))
					return err
				}
			}

		}

		return nil
	}
}

// EventSchemasHandler creates a handler that extracts and stores event schema information.
func EventSchemasHandler(logger *zap.Logger, db *sqlx.DB) HandlerFunc {
	events := store.NewEventsStore(db)

	return func(ctx context.Context, msg jetstream.Msg) error {
		event := Event{}
		err := json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal event message", zap.Error(err))
			return err
		}

		logger.Info("incoming event schema", zap.Stringer("event_id", event.ID), zap.Stringer("project_id", event.ProjectID))

		paths := rules.ParsePaths(event.Data)
		err = events.UpsertEventSchema(ctx, event.ProjectID, event.ID, paths)
		if err != nil {
			logger.Error("failed to upsert event schema", zap.Error(err))
			return err
		}

		logger.Info("event schema processed successfully", zap.Stringer("event_id", event.ID))
		return nil
	}
}
