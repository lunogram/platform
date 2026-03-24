package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/actions"
	"github.com/lunogram/platform/internal/journeys"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

func JourneyStepHandler(logger *zap.Logger, db *sqlx.DB, jrny *journey.State, mgmt *management.State, pub pubsub.Publisher, actionRegistry *actions.Registry) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		event := schemas.JourneyStep{}
		err := json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal journey state message", zap.Error(err))
			return Permanent(err)
		}

		// Check if the journey has been cancelled before processing
		if event.StateID != nil {
			state, err := jrny.GetJourneyStateByID(ctx, *event.StateID)
			if err != nil {
				logger.Error("failed to check cancellation state", zap.Error(err))
				return err
			}
			if state.CompletedAt != nil {
				logger.Info("journey state already completed/cancelled, skipping",
					zap.Stringer("state_id", *event.StateID),
					zap.Stringer("user_id", event.UserID),
				)
				return nil
			}
		}

		step, err := jrny.GetJourneyStep(ctx, event.JourneyID, event.ExternalStepID, event.VersionID)
		if err != nil {
			logger.Error("failed to get journey step", zap.Error(err))
			return err
		}

		data, err := jrny.GetJourneyEntryData(ctx, db, event.JourneyEntryID, event.UserID)
		if err != nil {
			logger.Error("failed to get journey entry data", zap.Error(err))
			return err
		}

		state := &journey.JourneyUserState{
			JourneyID:       event.JourneyID,
			JourneyEntryID:  event.JourneyEntryID,
			UserID:          event.UserID,
			ExternalStepID:  step.ExternalID,
			PinnedVersionID: event.VersionID,
		}

		if event.StateID != nil {
			state, err = jrny.GetJourneyStateByID(ctx, *event.StateID)
			if err != nil {
				logger.Error("failed to get journey state by ID", zap.Error(err), zap.Stringer("state_id", *event.StateID))
				return err
			}
		}

		logger := logger.With(zap.String("step_type", step.Type), zap.String("step_id", step.ID.String()), zap.String("user_id", event.UserID.String()))
		logger.Info("processing journey step")

		start := time.Now()
		next, children, err := journeys.Handle(ctx, db, pub, event.ProjectID, event.UserID, step, state, data, mgmt, actionRegistry)
		duration := time.Since(start).Seconds()
		projectID := event.ProjectID.String()
		metrics.JourneyStepDurationSeconds.WithLabelValues(step.Type, projectID).Observe(duration)

		if err != nil {
			metrics.JourneyStepsErrorsTotal.WithLabelValues(step.Type, projectID).Inc()
			logger.Error("failed to handle journey step", zap.Error(err))
			return err
		}

		metrics.JourneyStepsProcessedTotal.WithLabelValues(step.Type, projectID).Inc()

		if next.ResumeAt != nil && next.CompletedAt == nil {
			metrics.JourneyStepsPausedTotal.WithLabelValues(step.Type, projectID).Inc()
			logger.Info("journey step processing paused, waiting for resume")
		}

		if next.CompletedAt != nil {
			metrics.JourneyStepsCompletedTotal.WithLabelValues(step.Type, projectID).Inc()
			logger.Info("journey completed")

			executed := schemas.JourneyStepExecuted{
				ProjectID:      event.ProjectID,
				JourneyID:      event.JourneyID,
				JourneyEntryID: event.JourneyEntryID,
				UserID:         event.UserID,
				ExternalStepID: step.ExternalID,
				StepType:       step.Type,
			}

			err = pub.Publish(ctx, schemas.JourneysStepExecuted(event.ProjectID, event.JourneyID, event.UserID), executed)
			if err != nil {
				logger.Error("failed to publish journey step executed event", zap.Error(err))
				return err
			}
		}

		next.ID, err = jrny.CreateUserJourneyState(ctx, next)
		if err != nil {
			logger.Error("failed to create journey user state", zap.Error(err))
			return err
		}

		for _, child := range children {
			stepType, err := jrny.GetStepType(ctx, *event.VersionID, child.ChildExternalID)
			if err != nil {
				logger.Error("failed to get step type for child", zap.Error(err))
				continue
			}

			next := schemas.JourneyStep{
				ProjectID:      event.ProjectID,
				JourneyID:      event.JourneyID,
				JourneyEntryID: event.JourneyEntryID,
				VersionID:      event.VersionID,
				UserID:         event.UserID,
				ExternalStepID: child.ChildExternalID,
				StepType:       stepType,
			}

			err = pub.Publish(ctx, schemas.JourneysAdvance(event.ProjectID, event.JourneyID, event.UserID), next)
			if err != nil {
				logger.Error("failed to publish next journey step", zap.Error(err))
				return err
			}

			logger.Info("published next journey step", zap.String("step_id", child.ChildExternalID), zap.String("step_type", stepType))
		}

		logger.Info("journey step processed successfully!")
		return nil
	}
}
