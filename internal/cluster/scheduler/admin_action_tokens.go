package scheduler

import (
	"context"
	"time"

	"github.com/lunogram/platform/internal/node/metrics"
	"go.uber.org/zap"
)

// actionTokenRetention is how long a token that can no longer be redeemed is
// kept. The row holds a hash of a dead secret, so retention buys nothing beyond
// a short window for answering "was this link ever used" while an incident is
// still being looked at.
const actionTokenRetention = 7 * 24 * time.Hour

// ReconcileAdminActionTokens deletes verification and reset tokens that can no
// longer be redeemed.
//
// Every registration and every reset request writes one, and nothing deleted
// them: the rows and their entries in the unique hash index accumulated for the
// life of the deployment. Purging is not security-critical -- an expired or
// consumed token is already refused -- which is exactly why it needed a caller
// rather than a note that somebody should add one.
func (controller *Controller) ReconcileAdminActionTokens(ctx context.Context) func() {
	return func() {
		defer controller.recover("admin_action_tokens")
		start := time.Now()

		purged, err := controller.actionTokens.PurgeExpiredAdminActionTokens(ctx, actionTokenRetention)
		if err != nil {
			controller.logger.Error("failed to purge spent admin action tokens", zap.Error(err))
			return
		}

		if purged > 0 {
			controller.logger.Debug("purged spent admin action tokens",
				zap.Int64("purged", purged),
				zap.Duration("took", time.Since(start)),
			)
		}

		metrics.ReconciliationRunsTotal.WithLabelValues("admin_action_tokens").Inc()
	}
}
