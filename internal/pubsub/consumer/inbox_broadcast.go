package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

// parseSentMessageID unmarshals an inbox.message.sent event and extracts the
// message ID from the data payload. All errors are permanent: a malformed
// event will never become valid on retry.
func parseSentMessageID(msg jetstream.Msg) (uuid.UUID, error) {
	var event struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return uuid.Nil, fmt.Errorf("unmarshal inbox sent event: %w", err)
	}

	messageIDStr, _ := event.Data["message_id"].(string)
	if messageIDStr == "" {
		return uuid.Nil, fmt.Errorf("inbox sent event missing message_id")
	}

	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("inbox sent event invalid message_id %q: %w", messageIDStr, err)
	}

	return messageID, nil
}

// completeBroadcastIfDone is called after an inbox message is marked sent.
// When the message belongs to a broadcast, it atomically increments the
// broadcast's sent counter and transitions the broadcast to completed once
// all messages have been dispatched.
func (h *InboxHandler) completeBroadcastIfDone(ctx context.Context, message *subjects.InboxMessage) error {
	if message.BroadcastID == nil {
		return nil
	}

	broadcastID := *message.BroadcastID
	logger := h.logger.With(
		zap.Stringer("project_id", message.ProjectID),
		zap.Stringer("broadcast_id", broadcastID),
		zap.Stringer("message_id", message.ID),
	)

	sent, total, err := h.mgmt.BroadcastsStore.IncrementBroadcastSent(ctx, broadcastID)
	if err != nil {
		logger.Error("failed to increment broadcast sent counter", zap.Error(err))
		return err
	}

	logger.Debug("broadcast progress", zap.Int("sent", sent), zap.Int("total", total))

	if total > 0 && sent >= total {
		transitioned, err := h.mgmt.BroadcastsStore.TransitionBroadcastState(
			ctx, message.ProjectID, broadcastID,
			management.BroadcastStateSending, management.BroadcastStateCompleted,
			total, nil,
		)
		if err != nil {
			logger.Error("failed to transition broadcast to completed", zap.Error(err))
			return err
		}
		if transitioned {
			logger.Info("broadcast completed", zap.Int("sent", sent), zap.Int("total", total))
		}
	}

	return nil
}
