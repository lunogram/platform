//go:build enterprise

package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	internalProviders "github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

const (
	// DefaultBroadcastBatchSize is the number of users processed per batch message.
	DefaultBroadcastBatchSize = 1000
)

// BroadcastProcessHandler returns a handler that initiates broadcast processing.
// It validates the broadcast and campaign, then publishes the first batch message
// to begin the fan-out process.
func BroadcastProcessHandler(logger *zap.Logger, mgmt *management.State, usrs *subjects.State, registry *internalProviders.Registry, pub pubsub.Publisher, ns Namespace) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var event schemas.ProcessBroadcast
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			logger.Error("failed to unmarshal process broadcast message", zap.Error(err))
			return Permanent(err)
		}

		logger := logger.With(
			zap.String("project_id", event.ProjectID.String()),
			zap.String("broadcast_id", event.BroadcastID.String()),
		)
		logger.Info("processing broadcast")

		broadcast, err := mgmt.BroadcastsStore.GetBroadcast(ctx, event.ProjectID, event.BroadcastID)
		if err != nil {
			logger.Error("failed to get broadcast", zap.Error(err))
			return Permanent(err)
		}

		if broadcast.State != management.BroadcastStateSending {
			logger.Info("broadcast is not in sending state, skipping", zap.String("state", string(broadcast.State)))
			return Permanent(fmt.Errorf("broadcast %s is in state %s, expected sending", broadcast.ID, broadcast.State))
		}

		batchEvent := schemas.ProcessBroadcastBatch{
			ProjectID:   event.ProjectID,
			BroadcastID: event.BroadcastID,
			Offset:      0,
			BatchSize:   DefaultBroadcastBatchSize,
			Processed:   0,
		}

		if err := pub.Publish(ctx, schemas.BroadcastsBatch(event.ProjectID, event.BroadcastID), batchEvent); err != nil {
			logger.Error("failed to publish first broadcast batch", zap.Error(err))
			return err
		}

		logger.Info("broadcast batch processing initiated")
		return nil
	}
}

// BroadcastBatchHandler returns a handler that processes a single batch of
// broadcast users. It loads a page of list users, publishes a SendCampaign
// message for each, then either chains the next batch or marks the broadcast
// as completed when the list is exhausted.
func BroadcastBatchHandler(logger *zap.Logger, mgmt *management.State, usrs *subjects.State, pub pubsub.Publisher, ns Namespace) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var event schemas.ProcessBroadcastBatch
		if err := json.Unmarshal(msg.Data(), &event); err != nil {
			logger.Error("failed to unmarshal broadcast batch message", zap.Error(err))
			return Permanent(err)
		}

		logger := logger.With(
			zap.String("project_id", event.ProjectID.String()),
			zap.String("broadcast_id", event.BroadcastID.String()),
			zap.Int("offset", event.Offset),
			zap.Int("batch_size", event.BatchSize),
		)

		broadcast, err := mgmt.BroadcastsStore.GetBroadcast(ctx, event.ProjectID, event.BroadcastID)
		if err != nil {
			logger.Error("failed to get broadcast", zap.Error(err))
			return Permanent(err)
		}

		// If the broadcast is no longer in sending state (e.g. cancelled),
		// stop the batch chain.
		if broadcast.State != management.BroadcastStateSending {
			logger.Info("broadcast is no longer in sending state, stopping batch chain", zap.String("state", string(broadcast.State)))
			return Permanent(fmt.Errorf("broadcast %s is in state %s, expected sending", broadcast.ID, broadcast.State))
		}

		userIDs, err := usrs.ListsStore.ListUsersBatch(ctx, broadcast.ListID, event.BatchSize, event.Offset)
		if err != nil {
			logger.Error("failed to list users batch", zap.Error(err))
			return err
		}

		for _, userID := range userIDs {
			sendEvent := schemas.SendCampaign{
				ProjectID:   event.ProjectID,
				UserID:      userID,
				CampaignID:  broadcast.CampaignID,
				BroadcastID: &event.BroadcastID,
			}

			// Deterministic message ID so that if NATS redelivers this batch
			// (e.g. ack lost after publishes succeeded) the server-side
			// DuplicateWindow silently discards the repeated publishes
			// instead of creating duplicate SendCampaign messages that
			// would each call Reserve and inflate rate-limit delays.
			msgID := fmt.Sprintf("bc:%s:%s", event.BroadcastID, userID)

			err = pub.Publish(ctx, schemas.CampaignsSend(event.ProjectID, broadcast.CampaignID), sendEvent, pubsub.WithMsgID(msgID))
			if err != nil {
				logger.Error("failed to publish send campaign", zap.Error(err))
				return err
			}
		}

		batchCount := len(userIDs)
		totalProcessed := event.Processed + batchCount

		// Track incremental progress so the SSE endpoint can report it.
		if batchCount > 0 {
			if err := mgmt.BroadcastsStore.IncrementBroadcastTotal(ctx, event.ProjectID, event.BroadcastID, batchCount); err != nil {
				logger.Error("failed to increment broadcast total", zap.Error(err))
				return err
			}
		}

		// If the batch was full, there are likely more users — chain the next batch.
		if batchCount == event.BatchSize {
			nextBatch := schemas.ProcessBroadcastBatch{
				ProjectID:   event.ProjectID,
				BroadcastID: event.BroadcastID,
				Offset:      event.Offset + event.BatchSize,
				BatchSize:   event.BatchSize,
				Processed:   totalProcessed,
			}

			if err := pub.Publish(ctx, schemas.BroadcastsBatch(event.ProjectID, event.BroadcastID), nextBatch); err != nil {
				logger.Error("failed to publish next broadcast batch", zap.Error(err))
				return err
			}

			logger.Info("broadcast batch processed, chaining next", zap.Int("batch_count", batchCount), zap.Int("total_processed", totalProcessed))
			return nil
		}

		// No users matched the list — mark the broadcast as completed
		// immediately so it doesn't stay in "sending" state forever.
		if totalProcessed == 0 {
			if err := mgmt.BroadcastsStore.UpdateBroadcastState(ctx, event.ProjectID, event.BroadcastID, management.BroadcastStateCompleted, 0, nil); err != nil {
				logger.Error("failed to mark empty broadcast as completed", zap.Error(err))
				return err
			}
			logger.Info("broadcast completed (no matching users)")
			return nil
		}

		// Last batch — all SendCampaign messages have been published to the
		// stream but have not been sent yet (most are rate-limited and
		// scheduled for future delivery). The broadcast stays in "sending"
		// state; the campaign send handler will transition it to "completed"
		// once the last message has actually been delivered.
		logger.Info("broadcast queued, waiting for all sends to complete", zap.Int("total", totalProcessed))
		return nil
	}
}
