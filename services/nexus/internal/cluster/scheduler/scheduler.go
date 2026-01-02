package scheduler

import (
	"context"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/pubsub"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"go.uber.org/zap"
)

func NewController(ctx graceful.Context, logger *zap.Logger, config config.Node, db *sqlx.DB, pub pubsub.Publisher) *Controller {
	return &Controller{
		logger:   logger,
		config:   config,
		journeys: store.NewJourneysStore(db),
		pub:      pub,
	}
}

type Controller struct {
	logger   *zap.Logger
	config   config.Node
	journeys *store.JourneysStore
	pub      pubsub.Publisher
}

func (controller *Controller) ReconcileJourneyResumptions(ctx context.Context) {
	states, err := controller.journeys.ListResumeableUserJourneys(ctx)
	if err != nil {
		controller.logger.Error("failed to list resumeable user journeys", zap.Error(err))
		return
	}

	for _, state := range states {
		step := pubsub.JourneyStep{
			JourneyID:      state.JourneyID,
			JourneyEntryID: state.JourneyEntryID,
			VersionID:      state.PinnedVersionID,
			UserID:         state.UserID,
			ExternalStepID: state.ExternalStepID,
		}

		err = controller.pub.Publish(ctx, pubsub.JourneyStepSubject(state.ProjectID), step)
		if err != nil {
			controller.logger.Error("failed to publish journey step", zap.Error(err), zap.Stringer("journey_id", state.JourneyID), zap.Stringer("journey_entry_id", state.JourneyEntryID))
		}
	}
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
			controller.ReconcileJourneyResumptions(ctx)
		}
	}
}
