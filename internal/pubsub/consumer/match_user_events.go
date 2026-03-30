package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// MatchUserEventsHandler creates a handler that resolves a JSONB match filter
// into individual user IDs and publishes a UserEvent for each matched user.
func MatchUserEventsHandler(logger *zap.Logger, usrs *subjects.State, pub pubsub.Publisher) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var event schemas.MatchUserEvent
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			logger.Error("failed to unmarshal match user event", zap.Error(err))
			return Permanent(err)
		}

		logger.Info("processing match user event", zap.String("name", event.Name), zap.Stringer("project_id", event.ProjectID), zap.Any("match", event.Match))

		// Derive a stable origin ID from the incoming NATS message so that
		// redeliveries produce the same dedup keys and don't fan out twice.
		origin := msg.Headers().Get(MsgIDHeader)
		if origin == "" {
			origin = uuid.New().String()
		}

		users, err := usrs.ScanUsersMatchingData(ctx, event.ProjectID, event.Match, func(userID uuid.UUID) error {
			userEvent := schemas.UserEvent{
				ProjectID: event.ProjectID,
				Name:      event.Name,
				UserID:    userID,
				Data:      event.Data,
			}
			id := fmt.Sprintf("match-user:%s:%s", origin, userID)
			return pub.Publish(ctx, schemas.UserEventsProcess(event.ProjectID), userEvent, pubsub.WithMsgID(id))
		})
		if err != nil {
			logger.Error("failed to scan matching users", zap.Error(err))
			return err
		}

		logger.Info("match user event processed",
			zap.String("name", event.Name),
			zap.Stringer("project_id", event.ProjectID),
			zap.Int("matched_users", users),
		)

		metrics.MatchEventsProcessedTotal.WithLabelValues("user").Inc()
		metrics.MatchEventsMatchedTotal.WithLabelValues("user").Add(float64(users))
		return nil
	}
}
