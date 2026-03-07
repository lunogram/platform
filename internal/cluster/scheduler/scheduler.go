package scheduler

import (
	"context"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/journey"
	"go.uber.org/zap"
)

func NewController(ctx graceful.Context, logger *zap.Logger, config config.Node, jrny *journey.State, pub pubsub.Publisher) *Controller {
	return &Controller{
		logger:   logger,
		config:   config,
		journeys: jrny.JourneysStore,
		pub:      pub,
	}
}

type Controller struct {
	logger   *zap.Logger
	config   config.Node
	journeys *journey.JourneysStore
	pub      pubsub.Publisher
}

func (controller *Controller) ReconcileJourneyResumptions(ctx context.Context) {
	states, err := controller.journeys.ListResumeableUserJourneys(ctx)
	if err != nil {
		controller.logger.Error("failed to list resumeable user journeys", zap.Error(err))
		return
	}

	for _, state := range states {
		stepType, err := controller.journeys.GetStepType(ctx, *state.PinnedVersionID, state.ExternalStepID)
		if err != nil {
			controller.logger.Error("failed to get step type", zap.Error(err), zap.Stringer("journey_id", state.JourneyID), zap.String("step_id", state.ExternalStepID))
			continue
		}

		step := schemas.JourneyStep{
			ProjectID:      state.ProjectID,
			JourneyID:      state.JourneyID,
			JourneyEntryID: state.JourneyEntryID,
			VersionID:      state.PinnedVersionID,
			UserID:         state.UserID,
			ExternalStepID: state.ExternalStepID,
			StepType:       stepType,
			StateID:        &state.ID,
		}

		err = controller.pub.Publish(ctx, schemas.JourneysAdvance(state.ProjectID, state.JourneyID, state.UserID), step)
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
