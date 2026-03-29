//go:build enterprise

package consumer

import (
	"context"
	"encoding/json"
	"fmt"

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
func BroadcastProcessHandler(logger *zap.Logger, mgmt *management.State, usrs *subjects.State, pub pubsub.Publisher, ns Namespace) HandlerFunc {
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

		campaign, err := mgmt.CampaignsStore.GetCampaign(ctx, event.ProjectID, broadcast.CampaignID)
		if err != nil {
			logger.Error("failed to get campaign", zap.Error(err))
			return Permanent(err)
		}

		if campaign.Provider == nil {
			logger.Error("campaign has no provider configured", zap.String("campaign_id", campaign.ID.String()))
			return Permanent(fmt.Errorf("campaign %s has no provider configured", campaign.ID))
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

			err = pub.Publish(ctx, schemas.CampaignsSend(event.ProjectID, broadcast.CampaignID), sendEvent)
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

		// Last batch — mark the broadcast as completed.
		if err := mgmt.BroadcastsStore.UpdateBroadcastState(ctx, event.ProjectID, broadcast.ID, management.BroadcastStateCompleted, totalProcessed, nil); err != nil {
			logger.Error("failed to update broadcast state to completed", zap.Error(err))
			return Permanent(err)
		}

		logger.Info("broadcast completed", zap.Int("total", totalProcessed))
		return nil
	}
}
