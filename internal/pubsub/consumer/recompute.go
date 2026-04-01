package consumer

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/redis"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

type RecomputeList struct {
	ID        uuid.UUID `json:"id"`
	ProjectID uuid.UUID `json:"project_id"`
}

// RecomputeListHandler returns a NATS message handler that processes list
// recomputation requests.
//
// Each message is expected to contain a JSON payload with a list ID and
// project ID. For every message received, the handler:
//
//   - loads the list definition from the database
//   - skips recomputation if the list has no rule
//   - evaluates the ruleset and updates list membership
//   - publishes events for each user added or removed from the list
//
// Duplicate recomputes are avoided through a distributed generation-counter
// mechanism backed by Redis (see [RecomputeLocker]). When multiple messages
// arrive for the same list in quick succession, only one handler actively
// recomputes while the others ACK and return. The active handler re-loops
// if new messages arrived during its run, guaranteeing that no state change
// is missed.
//
// When the locker is nil (Redis not configured) the handler falls back to
// processing every message unconditionally, which is safe because the
// underlying database operation is idempotent.
func RecomputeListHandler(logger *zap.Logger, usrs *subjects.State, pub pubsub.Publisher, locker *redis.RecomputeLocker) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		event := RecomputeList{}
		err := json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal recompute list message", zap.Error(err))
			return Permanent(err)
		}

		logger := logger.With(zap.Stringer("project_id", event.ProjectID), zap.Stringer("list_id", event.ID))

		acquired, generation := locker.Acquire(ctx, event.ProjectID, event.ID)
		if !acquired {
			logger.Debug("recompute already in progress, skipping (generation bumped)")
			return nil
		}

		logger.Info("recomputing list")

		list, err := usrs.GetList(ctx, event.ProjectID, event.ID)
		if err != nil {
			logger.Error("failed to get list for recompute", zap.Error(err))
			locker.Release(ctx, event.ProjectID, event.ID)
			return err
		}

		if list.Rule == nil {
			logger.Info("list has no rule, skipping recompute")
			locker.Release(ctx, event.ProjectID, event.ID)
			return nil
		}

		for {
			recomputed, err := usrs.RecomputeList(ctx, event.ProjectID, list.ID, list.Rule.Data)
			if err != nil {
				logger.Error("failed to recompute list", zap.Error(err))
				locker.Release(ctx, event.ProjectID, event.ID)
				return err
			}

			err = PublishListRecomputeEvents(ctx, logger, pub, event.ProjectID, event.ID, recomputed)
			if err != nil {
				logger.Error("failed to publish list recompute events", zap.Error(err))
				locker.Release(ctx, event.ProjectID, event.ID)
				return err
			}

			locker.Release(ctx, event.ProjectID, event.ID)

			next := locker.Generation(ctx, event.ProjectID, event.ID)
			if next == generation {
				break
			}

			logger.Info("generation advanced during recompute, re-acquiring",
				zap.Int64("previous_generation", generation),
				zap.Int64("next_generation", next),
			)

			acquired, generation = locker.Acquire(ctx, event.ProjectID, event.ID)
			if !acquired {
				logger.Debug("another handler acquired the lock, skipping re-loop")
				break
			}
		}

		logger.Info("successfully recomputed list")
		return nil
	}
}

func PublishListRecomputeEvents(ctx context.Context, logger *zap.Logger, pub pubsub.Publisher, projectID uuid.UUID, listID uuid.UUID, recomputed []subjects.Recomputed) (err error) {
	for _, applied := range recomputed {
		switch applied.Action {
		case subjects.RecomputeActionInserted:
			event := schemas.UserEvent{
				Name:      schemas.EventListUserAdded,
				UserID:    applied.UserID,
				ProjectID: projectID,
				Data: map[string]any{
					"list_id": listID,
				},
			}

			err = pub.Publish(ctx, schemas.UserEventsProcess(event.ProjectID), event)
			if err != nil {
				logger.Error("failed to publish user list inserted event", zap.Error(err))
				return err
			}
		case subjects.RecomputeActionDeleted:
			event := schemas.UserEvent{
				Name:      schemas.EventListUserRemoved,
				UserID:    applied.UserID,
				ProjectID: projectID,
				Data: map[string]any{
					"list_id": listID,
				},
			}

			err = pub.Publish(ctx, schemas.UserEventsProcess(event.ProjectID), event)
			if err != nil {
				logger.Error("failed to publish user list removed event", zap.Error(err))
				return err
			}
		}
	}

	return nil
}
