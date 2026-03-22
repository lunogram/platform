package scheduler

import (
	"context"

	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/journey"
	"go.uber.org/zap"
)

func (controller *Controller) ReconcileJourneyResumptions(ctx context.Context) func() {
	return func() {
		defer controller.recover("journey_resumptions")
		var processed, published, failed int

		scanner := func(state journey.JourneyUserState) error {
			processed++
			stepType, err := controller.journeys.GetStepType(ctx, *state.PinnedVersionID, state.ExternalStepID)
			if err != nil {
				controller.logger.Error("failed to get step type", zap.Error(err), zap.Stringer("journey_id", state.JourneyID), zap.String("step_id", state.ExternalStepID))
				return nil
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
				failed++
				controller.logger.Error("failed to publish journey step", zap.Error(err), zap.Stringer("journey_id", state.JourneyID), zap.Stringer("journey_entry_id", state.JourneyEntryID))
				return nil
			}

			published++
			controller.logger.Debug("published journey resumption",
				zap.Stringer("journey_id", state.JourneyID),
				zap.Stringer("journey_entry_id", state.JourneyEntryID),
				zap.Stringer("user_id", state.UserID),
				zap.Stringer("project_id", state.ProjectID),
				zap.String("step_id", state.ExternalStepID),
				zap.String("step_type", string(stepType)),
			)

			return nil
		}

		err := controller.journeys.ScanResumeableUserJourneys(ctx, scanner)
		if err != nil {
			controller.logger.Error("failed to scan resumeable user journeys", zap.Error(err))
		}

		controller.logger.Debug("reconciled journey resumptions",
			zap.Int("processed", processed),
			zap.Int("published", published),
			zap.Int("failed", failed),
		)
	}
}
