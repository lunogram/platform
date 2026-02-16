package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/rules/eval"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/users"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type JourneyStep struct {
	ProjectID      uuid.UUID  `json:"project_id"`
	JourneyID      uuid.UUID  `json:"journey_id"`
	JourneyEntryID uuid.UUID  `json:"journey_entry_id"`
	VersionID      *uuid.UUID `json:"version_id,omitempty"`
	UserID         uuid.UUID  `json:"user_id"`
	ExternalStepID string     `json:"external_step_id"`
	StateID        *uuid.UUID `json:"state_id,omitempty"`
}

// EventsHandler creates a handler that processes incoming events and stores them in the database.
func EventsHandler(logger *zap.Logger, usrs *users.State, jrny *journey.State, pub pubsub.Publisher) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		event := schemas.Event{}
		err := json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal event message", zap.Error(err))
			return err
		}

		logger.Info("incoming event", zap.String("name", event.Name), zap.Stringer("project_id", event.ProjectID))

		event.ID, err = usrs.UpsertEvent(ctx, event.ProjectID, event.Name)
		if err != nil {
			logger.Error("failed to upsert event", zap.Error(err))
			return err
		}

		if event.UserID == uuid.Nil {
			logger.Info("looking up user ID", zap.Stringp("external_id", event.ExternalId), zap.Stringp("anonymous_id", event.AnonymousId))

			event.UserID, err = usrs.LookupUserID(ctx, event.ProjectID, event.ExternalId, event.AnonymousId)
			if err != nil {
				logger.Error("failed to lookup user ID", zap.Error(err))
				return err
			}
		}

		_, err = usrs.InsertUserEvent(ctx, event.UserID, event.ID, event.Data)
		if err != nil {
			logger.Error("failed to insert user event", zap.Error(err))
			return err
		}

		wg, ctx := errgroup.WithContext(ctx)
		wg.Go(PublishEventSchema(ctx, logger, pub, event))
		wg.Go(PublishEventListDependencies(ctx, logger, usrs, pub, event))
		wg.Go(PublishEventJourneyDependencies(ctx, logger, usrs, jrny, pub, event))

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
func PublishEventSchema(ctx context.Context, logger *zap.Logger, pub pubsub.Publisher, event schemas.Event) func() error {
	return func() error {
		if event.Data != nil {
			err := pub.Publish(ctx, schemas.EventsSchema(event.ProjectID), event)
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
func PublishEventListDependencies(ctx context.Context, logger *zap.Logger, usrs *users.State, pub pubsub.Publisher, event schemas.Event) func() error {
	return func() error {
		lists, err := usrs.ListEventListDependencies(ctx, event.ID)
		if err != nil {
			logger.Error("failed to list rule event dependencies", zap.Error(err))
			return err
		}

		for _, id := range lists {
			list := RecomputeList{
				ID:        id,
				ProjectID: event.ProjectID,
			}

			err = pub.Publish(ctx, schemas.ListsRecompute(event.ProjectID, list.ID), list)
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
func PublishEventJourneyDependencies(ctx context.Context, logger *zap.Logger, usrs *users.State, jrny *journey.State, pub pubsub.Publisher, event schemas.Event) func() error {
	evaluator := eval.NewEvaluator()

	return func() error {
		deps, err := usrs.ListEventJourneyDependencies(ctx, event.ID)
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
			result := journey.JourneyUserState{
				JourneyID:      dep.JourneyID,
				JourneyEntryID: entry,
				UserID:         event.UserID,
				ExternalStepID: dep.ExternalID,
				Data:           json.RawMessage(data),
				CompletedAt:    &now,
			}

			_, err = jrny.CreateUserJourneyState(ctx, result)
			if err != nil {
				logger.Error("failed to create journey user state", zap.Error(err))
				return err
			}

			for _, child := range dep.Children {
				step := JourneyStep{
					ProjectID:      event.ProjectID,
					JourneyID:      dep.JourneyID,
					JourneyEntryID: entry,
					ExternalStepID: child.ChildExternalID,
					UserID:         event.UserID,
				}

				err = pub.Publish(ctx, schemas.JourneysAdvance(event.ProjectID, dep.JourneyID), step)
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
func EventSchemasHandler(logger *zap.Logger, usrs *users.State) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		event := schemas.Event{}
		err := json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal event message", zap.Error(err))
			return err
		}

		logger.Info("incoming event schema", zap.Stringer("event_id", event.ID), zap.Stringer("project_id", event.ProjectID))

		paths := rules.ParsePaths(event.Data)
		err = usrs.UpsertEventSchema(ctx, event.ProjectID, event.ID, paths)
		if err != nil {
			logger.Error("failed to upsert event schema", zap.Error(err))
			return err
		}

		logger.Info("event schema processed successfully", zap.Stringer("event_id", event.ID))
		return nil
	}
}
