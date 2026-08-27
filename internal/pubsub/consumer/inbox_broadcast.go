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

// parseInboxLifecycleMessageID unmarshals an inbox lifecycle event and extracts
// the message ID from the data payload. All errors are permanent: a malformed
// event will never become valid on retry.
func parseInboxLifecycleMessageID(msg jetstream.Msg) (uuid.UUID, error) {
	var event struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(msg.Data(), &event); err != nil {
		return uuid.Nil, fmt.Errorf("unmarshal inbox lifecycle event: %w", err)
	}

	messageIDStr, _ := event.Data["message_id"].(string)
	if messageIDStr == "" {
		return uuid.Nil, fmt.Errorf("inbox lifecycle event missing message_id")
	}

	messageID, err := uuid.Parse(messageIDStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("inbox lifecycle event invalid message_id %q: %w", messageIDStr, err)
	}

	return messageID, nil
}

// completeBroadcastIfDone is called after an inbox message reaches a terminal
// outcome. When the message belongs to a broadcast it atomically advances the
// matching counter and transitions the broadcast to completed once every
// message it published has settled.
//
// Sent and failed are counted separately: a suppressed recipient never received
// anything, and `sent` is reported to the customer as messages that went out.
// Completion therefore reads sent+failed rather than sent alone, or a broadcast
// whose tail was entirely suppressed would sit in "sending" forever.
func (h *InboxHandler) completeBroadcastIfDone(ctx context.Context, message *subjects.InboxMessage, outcome broadcastOutcome) error {
	if message.BroadcastID == nil {
		return nil
	}

	broadcastID := *message.BroadcastID
	logger := h.logger.With(
		zap.Stringer("project_id", message.ProjectID),
		zap.Stringer("broadcast_id", broadcastID),
		zap.Stringer("message_id", message.ID),
		zap.String("outcome", string(outcome)),
	)

	progress, err := h.incrementBroadcast(ctx, broadcastID, outcome)
	if err != nil {
		logger.Error("failed to increment broadcast counter", zap.Error(err))
		return err
	}

	logger.Debug("broadcast progress",
		zap.Int("sent", progress.Sent),
		zap.Int("failed", progress.Failed),
		zap.Int("total", progress.Total),
	)

	if !progress.Settled() {
		return nil
	}

	transitioned, err := h.mgmt.BroadcastsStore.TransitionBroadcastState(
		ctx, message.ProjectID, broadcastID,
		management.BroadcastStateSending, management.BroadcastStateCompleted,
		progress.Total, nil,
	)
	if err != nil {
		logger.Error("failed to transition broadcast to completed", zap.Error(err))
		return err
	}
	if transitioned {
		logger.Info("broadcast completed",
			zap.Int("sent", progress.Sent),
			zap.Int("failed", progress.Failed),
			zap.Int("total", progress.Total),
		)
	}

	return nil
}

// broadcastOutcome selects which of a broadcast's terminal counters a settled
// message advances.
type broadcastOutcome string

const (
	broadcastOutcomeSent   broadcastOutcome = "sent"
	broadcastOutcomeFailed broadcastOutcome = "failed"
)

func (h *InboxHandler) incrementBroadcast(ctx context.Context, broadcastID uuid.UUID, outcome broadcastOutcome) (management.BroadcastProgress, error) {
	if outcome == broadcastOutcomeFailed {
		return h.mgmt.BroadcastsStore.IncrementBroadcastFailed(ctx, broadcastID)
	}
	return h.mgmt.BroadcastsStore.IncrementBroadcastSent(ctx, broadcastID)
}
