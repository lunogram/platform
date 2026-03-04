package consumer

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
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
// Membership recomputation is idempotent and handled entirely in the database;
// the handler itself does not batch, debounce, or persist recompute state.
func RecomputeListHandler(logger *zap.Logger, usrs *subjects.State, pub pubsub.Publisher) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		event := RecomputeList{}
		err := json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal recompute list message", zap.Error(err))
			return err
		}

		logger := logger.With(zap.Stringer("project_id", event.ProjectID), zap.Stringer("list_id", event.ID))
		logger.Info("recomputing list", zap.Stringer("list_id", event.ID))

		list, err := usrs.GetList(ctx, event.ProjectID, event.ID)
		if err != nil {
			logger.Error("failed to get list for recompute", zap.Error(err))
			return err
		}

		if list.Rule == nil {
			logger.Info("list has no rule, skipping recompute")
			return nil
		}

		recomputed, err := usrs.RecomputeList(ctx, event.ProjectID, list.ID, list.Rule.Data)
		if err != nil {
			logger.Error("failed to recompute list", zap.Error(err))
			return err
		}

		err = PublishListRecomputeEvents(ctx, logger, pub, event.ProjectID, event.ID, recomputed)
		if err != nil {
			logger.Error("failed to publish list recompute events", zap.Error(err))
			return err
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
