package consumer

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/actions"
	"github.com/lunogram/platform/internal/journeys"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

func JourneyStepHandler(logger *zap.Logger, db *sqlx.DB, jrny *journey.State, mgmt *management.State, pub pubsub.Publisher, actionRegistry *actions.Registry, registry *providers.Registry) HandlerFunc {
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
		next, children, err := journeys.Handle(ctx, logger, db, pub, event.ProjectID, event.UserID, step, state, data, mgmt, actionRegistry, registry)
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

// JourneyEntranceHandler creates a handler that processes journey entrance
// requests. It performs the eligibility check, creates the initial journey
// state, and publishes advancement messages for the entrance step's children.
// This offloads per-user entrance work from the event handlers so they only
// need to evaluate the entrance rule once and publish lightweight messages.
func JourneyEntranceHandler(logger *zap.Logger, jrny *journey.State, pub pubsub.Publisher) HandlerFunc {
	return func(ctx context.Context, msg jetstream.Msg) error {
		var entrance schemas.JourneyEntrance
		if err := json.Unmarshal(msg.Data(), &entrance); err != nil {
			logger.Error("failed to unmarshal journey entrance message", zap.Error(err))
			return Permanent(err)
		}

		logger := logger.With(
			zap.Stringer("journey_id", entrance.JourneyID),
			zap.Stringer("user_id", entrance.UserID),
			zap.String("external_step_id", entrance.ExternalStepID),
		)

		eligible, err := jrny.CheckEntryEligibility(ctx, entrance.JourneyID, entrance.UserID, entrance.ExternalStepID, entrance.Multiple, entrance.Concurrent)
		if err != nil {
			logger.Error("failed to check journey entry eligibility", zap.Error(err))
			return err
		}

		if !eligible {
			logger.Info("user not eligible to enter journey",
				zap.Bool("multiple", entrance.Multiple),
				zap.Bool("concurrent", entrance.Concurrent),
			)
			metrics.JourneyEntranceRejectionsTotal.WithLabelValues(entrance.ProjectID.String(), "not_eligible").Inc()
			return nil
		}

		data, err := json.Marshal(map[string]any{"data": entrance.Data})
		if err != nil {
			logger.Error("failed to marshal journey entry data", zap.Error(err))
			return err
		}

		now := time.Now()
		versionID := entrance.VersionID
		result := journey.JourneyUserState{
			JourneyID:       entrance.JourneyID,
			JourneyEntryID:  uuid.New(),
			UserID:          entrance.UserID,
			ExternalStepID:  entrance.ExternalStepID,
			PinnedVersionID: &versionID,
			Data:            json.RawMessage(data),
			CompletedAt:     &now,
		}

		_, err = jrny.CreateUserJourneyState(ctx, result)
		if err != nil {
			logger.Error("failed to create journey user state", zap.Error(err))
			return err
		}

		metrics.JourneyEntrancesTotal.WithLabelValues(entrance.ProjectID.String()).Inc()

		var children store.JourneyVersionStepChildren
		if err := json.Unmarshal(entrance.Children, &children); err != nil {
			logger.Error("failed to unmarshal entrance children", zap.Error(err))
			return err
		}

		for _, child := range children {
			stepType, err := jrny.GetStepType(ctx, entrance.VersionID, child.ChildExternalID)
			if err != nil {
				logger.Error("failed to get step type for child", zap.Error(err))
				continue
			}

			step := schemas.JourneyStep{
				ProjectID:      entrance.ProjectID,
				JourneyID:      entrance.JourneyID,
				JourneyEntryID: result.JourneyEntryID,
				VersionID:      &versionID,
				ExternalStepID: child.ChildExternalID,
				UserID:         entrance.UserID,
				StepType:       stepType,
			}

			err = pub.Publish(ctx, schemas.JourneysAdvance(entrance.ProjectID, entrance.JourneyID, entrance.UserID), step)
			if err != nil {
				logger.Error("failed to publish journey advancement", zap.Error(err))
				return err
			}
		}

		logger.Info("journey entrance processed")
		return nil
	}
}
