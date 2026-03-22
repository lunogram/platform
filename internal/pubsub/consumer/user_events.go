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
func UserEventsHandler(logger *zap.Logger, usrs *subjects.State, jrny *journey.State, pub pubsub.Publisher) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		event := schemas.UserEvent{}
		err := json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal event message", zap.Error(err))
			return Permanent(err)
		}

		logger.Info("incoming event", zap.String("name", event.Name), zap.Stringer("project_id", event.ProjectID))

		event.ID, err = usrs.UpsertEvent(ctx, event.ProjectID, event.Name, subjects.SubjectTypeUser)
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
		wg.Go(PublishUserEventSchema(ctx, logger, pub, event))
		wg.Go(PublishUserEventListDependencies(ctx, logger, usrs, pub, event))
		wg.Go(PublishUserEventJourneyDependencies(ctx, logger, usrs, jrny, pub, event))

		err = wg.Wait()
		if err != nil {
			logger.Error("failed to publish dependent events", zap.Error(err))
			return err
		}

		logger.Info("user event processed successfully", zap.Stringer("event_id", event.ID))
		return nil
	}
}

// PublishUserEventSchema returns a function that publishes the user event schema to the schema subject
// if the event contains data properties.
func PublishUserEventSchema(ctx context.Context, logger *zap.Logger, pub pubsub.Publisher, event schemas.UserEvent) func() error {
	return func() error {
		if event.Data != nil {
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
		}

		return nil
	}
}

// PublishUserEventJourneyDependencies returns a function that triggers journey entrance steps
// for all journeys configured with event-based entrance conditions matching the given user event.
func PublishUserEventJourneyDependencies(ctx context.Context, logger *zap.Logger, usrs *subjects.State, jrny *journey.State, pub pubsub.Publisher, event schemas.UserEvent) func() error {
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
				// TODO: we might want to pass the entire event
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

			logger.Info("triggering journey entrance step", zap.Stringer("journey_id", dep.JourneyID), zap.Stringer("step_id", dep.StepID))

			multiple := entrance.Multiple != nil && *entrance.Multiple
			concurrent := entrance.Concurrent != nil && *entrance.Concurrent

			eligible, err := jrny.CheckEntryEligibility(ctx, dep.JourneyID, event.UserID, dep.ExternalID, multiple, concurrent)
			if err != nil {
				logger.Error("failed to check journey entry eligibility", zap.Error(err))
				return err
			}

			if !eligible {
				logger.Info("user not eligible to enter journey", zap.Stringer("journey_id", dep.JourneyID), zap.Stringer("user_id", event.UserID), zap.Bool("multiple", multiple), zap.Bool("concurrent", concurrent))
				continue
			}

			data, err := json.Marshal(map[string]any{"data": event.Data})
			if err != nil {
				logger.Error("failed to marshal journey entry data", zap.Error(err))
				return err
			}

			// TODO: include support to pin to specific journey version
			now := time.Now()
			result := journey.JourneyUserState{
				JourneyID:      dep.JourneyID,
				JourneyEntryID: uuid.New(),
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
				stepType, err := jrny.GetStepType(ctx, dep.VersionID, child.ChildExternalID)
				if err != nil {
					logger.Error("failed to get step type for child", zap.Error(err))
					continue
				}

				step := JourneyStep{
					ProjectID:      event.ProjectID,
					JourneyID:      dep.JourneyID,
					JourneyEntryID: result.JourneyEntryID,
					ExternalStepID: child.ChildExternalID,
					UserID:         event.UserID,
					StepType:       stepType,
				}

				err = pub.Publish(ctx, schemas.JourneysAdvance(event.ProjectID, dep.JourneyID, event.UserID), step)
				if err != nil {
					logger.Error("failed to publish journey state to project subject", zap.Error(err))
					return err
				}
			}

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
