package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	iredis "github.com/lunogram/platform/internal/redis"
	"github.com/lunogram/platform/internal/rules/eval"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// MatchUserEventsHandler creates a handler that resolves a JSONB match filter
// into individual user IDs, inserts event records for all matched users in a
// single database query, and then publishes schema, list-recompute, and
// journey entrance messages. Journey entrance rules are evaluated once for
// all matched users and individual JourneyEntrance messages are published to
// NATS for each user × matching dependency.
func MatchUserEventsHandler(logger *zap.Logger, usrs *subjects.State, jrny *journey.State, pub pubsub.Publisher, schemaCache *iredis.SchemaCache) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		start := time.Now()
		var event schemas.MatchUserEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			logger.Error("failed to unmarshal match user event", zap.Error(err))
			return Permanent(err)
		}

		logger.Info("processing match user event",
			zap.String("name", event.Name),
			zap.Stringer("project_id", event.ProjectID),
			zap.Any("match", event.Match),
		)

		// Upsert the event definition once – same for every matched user.
		eventID, err := usrs.UpsertEvent(ctx, event.ProjectID, event.Name, subjects.SubjectTypeUser)
		if err != nil {
			logger.Error("failed to upsert event", zap.Error(err))
			return err
		}

		// Insert event records for all matching users in a single query and
		// get back the matched IDs.
		userIDs, err := usrs.InsertMatchingUserEvents(ctx, event.ProjectID, eventID, event.Match, event.Data)
		if err != nil {
			logger.Error("failed to insert matching user events", zap.Error(err))
			return err
		}

		matched := len(userIDs)
		logger.Info("matched users",
			zap.String("name", event.Name),
			zap.Stringer("project_id", event.ProjectID),
			zap.Int("matched_users", matched),
		)

		if matched == 0 {
			metrics.MatchEventsProcessedTotal.WithLabelValues("user").Inc()
			metrics.MatchEventsMatchedTotal.WithLabelValues("user").Add(0)
			return nil
		}

		userEvent := schemas.UserEvent{
			ID:        eventID,
			Name:      event.Name,
			ProjectID: event.ProjectID,
			Data:      event.Data,
		}

		wg, wgCtx := errgroup.WithContext(ctx)
		wg.Go(PublishUserEventSchema(wgCtx, logger, pub, userEvent, schemaCache))
		wg.Go(PublishUserEventListDependencies(wgCtx, logger, usrs, pub, userEvent))
		wg.Go(PublishMatchedUserEntrances(wgCtx, logger, jrny, pub, eventID, event, userIDs))

		if err := wg.Wait(); err != nil {
			logger.Error("failed to process dependent events", zap.Error(err))
			return err
		}

		logger.Info("match user event processed",
			zap.String("name", event.Name),
			zap.Stringer("project_id", event.ProjectID),
			zap.Int("matched_users", matched),
		)

		metrics.MatchEventsProcessedTotal.WithLabelValues("user").Inc()
		metrics.MatchEventsMatchedTotal.WithLabelValues("user").Add(float64(matched))
		metrics.EventsProcessedTotal.WithLabelValues("user").Add(float64(matched))
		metrics.EventsProcessingDurationSeconds.WithLabelValues("user").Observe(time.Since(start).Seconds())
		return nil
	}
}

// PublishMatchedUserEntrances fetches journey entrance dependencies for the
// given event, evaluates entrance rules once, and publishes a JourneyEntrance
// message for every matched user × qualifying dependency combination.
func PublishMatchedUserEntrances(ctx context.Context, logger *zap.Logger, jrny *journey.State, pub pubsub.Publisher, eventID uuid.UUID, event schemas.MatchUserEvent, userIDs []uuid.UUID) func() error {
	return func() error {
		deps, err := jrny.ListEventJourneyDependencies(ctx, eventID)
		if err != nil {
			logger.Error("failed to list journey event dependencies", zap.Error(err))
			return err
		}

		if len(deps) == 0 {
			return nil
		}

		evaluator := eval.NewEvaluator()

		for _, dep := range deps {
			entrance := oapi.EntranceStepData{}
			if dep.Data != nil {
				if err := json.Unmarshal(*dep.Data, &entrance); err != nil {
					return err
				}
			}

			if rule := entrance.EntranceRule(); rule != nil {
				data := map[string]any{
					"data": event.Data,
				}

				match, err := evaluator.Evaluate(*rule, data)
				if err != nil {
					logger.Error("failed to evaluate journey entrance rule", zap.Error(err))
					return err
				}

				if !match {
					continue
				}
			}

			childrenJSON, err := json.Marshal(dep.Children)
			if err != nil {
				logger.Error("failed to marshal entrance children", zap.Error(err))
				return err
			}

			multiple := entrance.Multiple
			concurrent := entrance.Concurrent

			logger.Info("publishing journey entrances for matched users",
				zap.Stringer("journey_id", dep.JourneyID),
				zap.Int("user_count", len(userIDs)),
			)

			for _, userID := range userIDs {
				msg := schemas.JourneyEntrance{
					ProjectID:      event.ProjectID,
					JourneyID:      dep.JourneyID,
					VersionID:      dep.VersionID,
					UserID:         userID,
					ExternalStepID: dep.ExternalID,
					Multiple:       multiple,
					Concurrent:     concurrent,
					Data:           event.Data,
					Children:       childrenJSON,
				}

				if err := pub.Publish(ctx, schemas.JourneysEntrance(event.ProjectID, dep.JourneyID, userID), msg); err != nil {
					logger.Error("failed to publish journey entrance", zap.Error(err), zap.Stringer("user_id", userID))
					return err
				}
			}

			metrics.EventsJourneyTriggersTotal.WithLabelValues("user").Add(float64(len(userIDs)))
		}

		return nil
	}
}
