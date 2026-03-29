//go:build enterprise

package scheduler

import (
	"context"
	"time"

	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

// ReconcileScheduledBroadcasts scans for broadcasts in 'scheduled' state whose
// scheduled_at time has arrived. For each due broadcast it:
//  1. Atomically transitions the broadcast from 'scheduled' to 'sending'
//  2. Publishes a ProcessBroadcast message to NATS to trigger processing
//
// NOTE: There is an intentional gap between the state transition and the NATS
// publish. If the process crashes after the transition but before publishing,
// the broadcast will be in 'sending' state without a NATS message. A separate
// reconciliation or manual send can handle this edge case. This is preferred
// over the reverse (publish then transition) which could cause duplicate sends.
func (controller *Controller) ReconcileScheduledBroadcasts(ctx context.Context) func() {
	return func() {
		defer controller.recover("scheduled_broadcasts")
		start := time.Now()
		var published, failed int

		scanner := func(broadcast management.Broadcast) error {
			// Atomically transition from scheduled -> sending to prevent
			// duplicate processing by concurrent scheduler runs.
			err := controller.broadcasts.TransitionScheduledBroadcastToSending(ctx, broadcast.ID)
			if err != nil {
				failed++
				controller.logger.Warn("failed to transition scheduled broadcast to sending",
					zap.Error(err),
					zap.Stringer("broadcast_id", broadcast.ID),
					zap.Stringer("project_id", broadcast.ProjectID),
				)
				return nil
			}

			err = controller.pub.Publish(ctx, schemas.BroadcastsProcess(broadcast.ProjectID, broadcast.ID), schemas.ProcessBroadcast{
				ProjectID:   broadcast.ProjectID,
				BroadcastID: broadcast.ID,
			})
			if err != nil {
				failed++
				controller.logger.Error("failed to publish scheduled broadcast",
					zap.Error(err),
					zap.Stringer("broadcast_id", broadcast.ID),
					zap.Stringer("project_id", broadcast.ProjectID),
				)
				return nil
			}

			published++
			controller.logger.Info("scheduled broadcast triggered",
				zap.Stringer("broadcast_id", broadcast.ID),
				zap.Stringer("project_id", broadcast.ProjectID),
				zap.Timep("scheduled_at", broadcast.ScheduledAt),
			)

			return nil
		}

		processed, err := controller.broadcasts.ScanScheduledBroadcasts(ctx, scanner)
		if err != nil {
			controller.logger.Error("failed to scan scheduled broadcasts", zap.Error(err))
		}

		controller.logger.Debug("reconciled scheduled broadcasts",
			zap.Int("processed", processed),
			zap.Int("published", published),
			zap.Int("failed", failed),
		)

		metrics.ReconciliationRunsTotal.WithLabelValues("scheduled_broadcasts").Inc()
		metrics.ReconciliationItemsProcessedTotal.WithLabelValues("scheduled_broadcasts").Add(float64(processed))
		metrics.ReconciliationItemsPublishedTotal.WithLabelValues("scheduled_broadcasts").Add(float64(published))
		metrics.ReconciliationItemsFailedTotal.WithLabelValues("scheduled_broadcasts").Add(float64(failed))
		metrics.ReconciliationDurationSeconds.WithLabelValues("scheduled_broadcasts").Observe(time.Since(start).Seconds())
	}
}
