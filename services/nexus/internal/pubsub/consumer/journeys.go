package consumer

import (
	"context"
	"encoding/json"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/pubsub"
	"github.com/lunogram/platform/services/nexus/internal/pubsub/consumer/journeys"
	"github.com/lunogram/platform/services/nexus/internal/pubsub/schemas"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

func JourneyStepHandler(logger *zap.Logger, db *sqlx.DB, pub pubsub.Publisher) HandlerFunc {
	journeyStore := store.NewJourneysStore(db)

	return func(ctx context.Context, msg jetstream.Msg) error {
		event := schemas.JourneyStep{}
		err := json.Unmarshal(msg.Data(), &event)
		if err != nil {
			logger.Error("failed to unmarshal journey state message", zap.Error(err))
			return err
		}

		step, err := journeyStore.GetJourneyStep(ctx, event.JourneyID, event.ExternalStepID, event.VersionID)
		if err != nil {
			logger.Error("failed to get journey step", zap.Error(err))
			return err
		}

		data, err := journeyStore.GetJourneyEntryData(ctx, event.JourneyEntryID, event.UserID)
		if err != nil {
			logger.Error("failed to get journey entry data", zap.Error(err))
			return err
		}

		state := &store.JourneyUserState{
			JourneyID:       event.JourneyID,
			JourneyEntryID:  event.JourneyEntryID,
			UserID:          event.UserID,
			ExternalStepID:  step.ExternalID,
			PinnedVersionID: event.VersionID,
		}

		if event.StateID != nil {
			state, err = journeyStore.GetJourneyStateByID(ctx, *event.StateID)
			if err != nil {
				logger.Error("failed to get journey state by ID", zap.Error(err), zap.Stringer("state_id", *event.StateID))
				return err
			}
		}

		logger := logger.With(zap.String("step_type", step.Type), zap.String("step_id", step.ID.String()), zap.String("user_id", event.UserID.String()))
		logger.Info("processing journey step")

		next, children, err := journeys.Handle(ctx, db, pub, event.ProjectID, event.UserID, step, state, data)
		if err != nil {
			logger.Error("failed to handle journey step", zap.Error(err))
			return err
		}

		if next.ResumeAt != nil && next.CompletedAt == nil {
			logger.Info("journey step processing paused, waiting for resume")
		}

		if next.CompletedAt != nil {
			logger.Info("journey completed")
		}

		next.ID, err = journeyStore.CreateUserJourneyState(ctx, next)
		if err != nil {
			logger.Error("failed to create journey user state", zap.Error(err))
			return err
		}

		for _, child := range children {
			next := schemas.JourneyStep{
				ProjectID:      event.ProjectID,
				JourneyID:      event.JourneyID,
				JourneyEntryID: event.JourneyEntryID,
				VersionID:      event.VersionID,
				UserID:         event.UserID,
				ExternalStepID: child.ChildExternalID,
			}

			err = pub.Publish(ctx, schemas.JourneyStepSubject(event.ProjectID, event.JourneyID), next)
			if err != nil {
				logger.Error("failed to publish next journey step", zap.Error(err))
				return err
			}

			logger.Info("published next journey step", zap.String("step_id", child.ChildExternalID))
		}

		logger.Info("journey step processed successfully!")
		return nil
	}
}
