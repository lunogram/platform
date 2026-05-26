package scheduler

import (
	"context"
	"time"

	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"go.uber.org/zap"
)

func (controller *Controller) ReconcileListRecomputation(ctx context.Context) func() {
	return func() {
		defer controller.recover("list_recomputation")
		start := time.Now()
		var published, failed int

		lists, err := controller.lists.SelectListsDueForTimeReconciliation(ctx, controller.reconciliationBatchSize)
		if err != nil {
			controller.logger.Error("failed to select lists due for time reconciliation", zap.Error(err))
			return
		}

		for _, list := range lists {
			msg := consumer.RecomputeList{
				ID:        list.ID,
				ProjectID: list.ProjectID,
			}

			err := controller.pub.Publish(ctx, schemas.ListsRecompute(list.ProjectID, list.ID), msg)
			if err != nil {
				failed++
				controller.logger.Error("failed to publish list recomputation",
					zap.Error(err),
					zap.Stringer("list_id", list.ID),
					zap.Stringer("project_id", list.ProjectID),
				)
				continue
			}

			published++
			controller.logger.Debug("published list recomputation",
				zap.Stringer("list_id", list.ID),
				zap.Stringer("project_id", list.ProjectID),
			)
		}

		controller.logger.Debug("reconciled list recomputation",
			zap.Int("processed", len(lists)),
			zap.Int("published", published),
			zap.Int("failed", failed),
		)

		metrics.ReconciliationRunsTotal.WithLabelValues("list_recomputation").Inc()
		metrics.ReconciliationItemsProcessedTotal.WithLabelValues("list_recomputation").Add(float64(len(lists)))
		metrics.ReconciliationItemsPublishedTotal.WithLabelValues("list_recomputation").Add(float64(published))
		metrics.ReconciliationItemsFailedTotal.WithLabelValues("list_recomputation").Add(float64(failed))
		metrics.ReconciliationDurationSeconds.WithLabelValues("list_recomputation").Observe(time.Since(start).Seconds())
	}
}
