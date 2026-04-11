package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	iredis "github.com/lunogram/platform/internal/redis"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/rules/eval"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/subjects"
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
	StepType       string     `json:"step_type"`
	StateID        *uuid.UUID `json:"state_id,omitempty"`
}

// UserEventsHandler creates a handler that processes incoming user events and stores them in the database.
func UserEventsHandler(logger *zap.Logger, usrs *subjects.State, jrny *journey.State, pub pubsub.Publisher, schemaCache *iredis.SchemaCache) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		start := time.Now()
		event := schemas.UserEvent{}
		err := json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal event message", zap.Error(err))
			metrics.EventsProcessingErrorsTotal.WithLabelValues("user").Inc()
			return Permanent(err)
		}

		logger.Info("incoming event", zap.String("name", event.Name), zap.Stringer("project_id", event.ProjectID))

		event.ID, err = usrs.UpsertEvent(ctx, event.ProjectID, event.Name, subjects.SubjectTypeUser)
		if err != nil {
			logger.Error("failed to upsert event", zap.Error(err))
			return err
		}

		if event.UserID == uuid.Nil {
			if len(event.Identifiers) == 0 {
				logger.Error("user_id or identifiers are required")
				return Permanent(fmt.Errorf("user_id or identifiers are required for user event"))
			}

			logger.Info("looking up user ID from identifiers")

			event.UserID, err = usrs.LookupUserID(ctx, event.ProjectID, event.Identifiers)
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
		wg.Go(PublishUserEventSchema(ctx, logger, pub, event, schemaCache))
		wg.Go(PublishUserEventListDependencies(ctx, logger, usrs, pub, event))
		wg.Go(PublishUserEventJourneyDependencies(ctx, logger, jrny, pub, event))

		err = wg.Wait()
		if err != nil {
			logger.Error("failed to publish dependent events", zap.Error(err))
			return err
		}

		logger.Info("user event processed successfully", zap.Stringer("event_id", event.ID))
		metrics.EventsProcessedTotal.WithLabelValues("user").Inc()
		metrics.EventsProcessingDurationSeconds.WithLabelValues("user").Observe(time.Since(start).Seconds())
		return nil
	}
}

// PublishUserEventSchema returns a function that publishes the user event schema to the schema subject
// if the event contains data properties and the data shape has not been seen before.
func PublishUserEventSchema(ctx context.Context, logger *zap.Logger, pub pubsub.Publisher, event schemas.UserEvent, schemaCache *iredis.SchemaCache) func() error {
	return func() error {
		if event.Data != nil {
			if schemaCache.Seen(ctx, iredis.Event, event.ProjectID, event.Data) {
				return nil
			}

			err := pub.Publish(ctx, schemas.UserEventsSchema(event.ProjectID), event)
			if err != nil {
				logger.Error("failed to publish event to project subject", zap.Error(err))
				return err
			}
		}

		return nil
	}
}

// PublishUserEventListDependencies returns a function that publishes recompute messages for all lists
// that depend on the given user event through rule conditions.
func PublishUserEventListDependencies(ctx context.Context, logger *zap.Logger, usrs *subjects.State, pub pubsub.Publisher, event schemas.UserEvent) func() error {
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

			metrics.EventsListRecomputesTotal.Inc()
		}

		return nil
	}
}

// PublishUserEventJourneyDependencies returns a function that evaluates journey
// entrance rules for the given user event and publishes a JourneyEntrance
// message for each matching dependency. The actual eligibility check, state
// creation, and child step advancement are handled by JourneyEntranceHandler.
func PublishUserEventJourneyDependencies(ctx context.Context, logger *zap.Logger, jrny *journey.State, pub pubsub.Publisher, event schemas.UserEvent) func() error {
	evaluator := eval.NewEvaluator()

	return func() error {
		deps, err := jrny.ListEventJourneyDependencies(ctx, event.ID)
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
				data := map[string]any{
					"data": event.Data,
				}

				match, err := evaluator.Evaluate(*entrance.Rule, data)
				if err != nil {
					logger.Error("failed to evaluate journey entrance rule", zap.Error(err))
					return err
				}

				if !match {
					continue
				}
			}

			logger.Info("publishing journey entrance",
				zap.Stringer("journey_id", dep.JourneyID),
				zap.Stringer("user_id", event.UserID),
			)

			childrenJSON, err := json.Marshal(dep.Children)
			if err != nil {
				logger.Error("failed to marshal entrance children", zap.Error(err))
				return err
			}

			msg := schemas.JourneyEntrance{
				ProjectID:      event.ProjectID,
				JourneyID:      dep.JourneyID,
				VersionID:      dep.VersionID,
				UserID:         event.UserID,
				ExternalStepID: dep.ExternalID,
				Multiple:       entrance.Multiple != nil && *entrance.Multiple,
				Concurrent:     entrance.Concurrent != nil && *entrance.Concurrent,
				Data:           event.Data,
				Children:       childrenJSON,
			}

			err = pub.Publish(ctx, schemas.JourneysEntrance(event.ProjectID, dep.JourneyID, event.UserID), msg)
			if err != nil {
				logger.Error("failed to publish journey entrance", zap.Error(err))
				return err
			}

			metrics.EventsJourneyTriggersTotal.WithLabelValues("user").Inc()
		}

		return nil
	}
}

// UserEventSchemasHandler creates a handler that extracts and stores user event schema information.
func UserEventSchemasHandler(logger *zap.Logger, usrs *subjects.State) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		event := schemas.UserEvent{}
		err := json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal event message", zap.Error(err))
			return Permanent(err)
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
