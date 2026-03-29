package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func NewController(ctx graceful.Context, logger *zap.Logger, config config.Node, jrny *journey.State, usrs *subjects.State, mgmt *management.State, pub pubsub.Publisher) *Controller {
	return &Controller{
		logger:     logger,
		config:     config,
		journeys:   jrny.JourneysStore,
		scheduled:  usrs.ScheduledStore,
		broadcasts: mgmt.BroadcastsStore,
		pub:        pub,
	}
}

type Controller struct {
	logger     *zap.Logger
	config     config.Node
	journeys   *journey.JourneysStore
	scheduled  *subjects.ScheduledStore
	broadcasts *management.BroadcastsStore
	pub        pubsub.Publisher
}

func (controller *Controller) Schedule(ctx context.Context) {
	logger := controller.logger.With(zap.String("component", "scheduler"))
	logger.Info("starting scheduler")

	for {
		// NOTE: align to the start of the minute
		now := time.Now()
		next := now.Truncate(time.Minute).Add(time.Minute)
		timer := time.NewTimer(time.Until(next))

		select {
		case <-ctx.Done():
			logger.Info("stopping scheduler")
			return
		case <-timer.C:
			logger.Debug("reconciling cluster state")
			var wg sync.WaitGroup
			wg.Go(controller.ReconcileJourneyResumptions(ctx))
			wg.Go(controller.ReconcileUserSchedules(ctx))
			wg.Go(controller.ReconcileOrganizationSchedules(ctx))
			wg.Go(controller.ReconcileUserScheduledEvents(ctx))
			wg.Go(controller.ReconcileOrganizationScheduledEvents(ctx))
			wg.Go(controller.ReconcileScheduledBroadcasts(ctx))
			wg.Wait() // nolint:errcheck
			logger.Debug("reconciliation complete")
		}
	}
}

// recover recovers from panics so that a single failing reconciliation task
// does not crash the entire scheduler loop. It is intended to be deferred at
// the top of each Reconcile* closure.
func (controller *Controller) recover(name string) {
	if r := recover(); r != nil {
		metrics.ReconciliationPanicsTotal.WithLabelValues(name).Inc()
		controller.logger.Error("panic during reconciliation",
			zap.String("task", name),
			zap.String("error", fmt.Sprint(r)),
			zap.Stack("stacktrace"),
		)
	}
}
