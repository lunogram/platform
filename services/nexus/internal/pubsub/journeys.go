package pubsub

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/journeys"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

type JourneyStep struct {
	JourneyID      uuid.UUID  `json:"journey_id"`
	JourneyEntryID uuid.UUID  `json:"journey_entry_id"`
	VersionID      *uuid.UUID `json:"version_id,omitempty"`
	UserID         uuid.UUID  `json:"user_id"`
	ExternalStepID string     `json:"external_step_id"`
}

// JourneyStepSubject returns the NATS subject for journey step requests.
func JourneyStepSubject(projectID uuid.UUID) Subject {
	return Subject(fmt.Sprintf("state.journeys.step.%s", projectID))
}

func JourneyStepHandler(logger *zap.Logger, db *sqlx.DB, pub Publisher) HandlerFunc {
	journeyStore := store.NewJourneysStore(db)

	return func(ctx context.Context, msg jetstream.Msg) error {
		event := JourneyStep{}
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

		fmt.Println("---------", event.JourneyID, event.ExternalStepID, event.VersionID)

		data, err := journeyStore.GetJourneyEntryData(ctx, event.JourneyEntryID, event.UserID)
		if err != nil {
			logger.Error("failed to get journey entry data", zap.Error(err))
			return err
		}

		state, err := journeyStore.GetUserJourneyState(ctx, event.JourneyEntryID, event.ExternalStepID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			logger.Error("failed to get user journey state", zap.Error(err))
			return err
		}

		logger := logger.With(zap.String("step_type", step.Type), zap.String("step_id", step.ID.String()), zap.String("user_id", event.UserID.String()))
		logger.Info("processing journey step")

		result, children, err := journeys.Handle(ctx, step, state, data)
		if err != nil {
			logger.Error("failed to handle journey step", zap.Error(err))
			return err
		}

		result.JourneyID = event.JourneyID
		result.JourneyEntryID = event.JourneyEntryID
		result.UserID = event.UserID
		result.ExternalStepID = step.ExternalID
		result.PinnedVersionID = event.VersionID

		if result.ResumeAt != nil && result.CompletedAt == nil {
			logger.Info("journey step processing paused, waiting for resume")
		}

		if result.CompletedAt == nil && result.ResumeAt == nil {
			logger.Info("journey completed")

			now := time.Now()
			result.CompletedAt = &now
		}

		result.ID, err = journeyStore.CreateUserJourneyState(ctx, result)
		if err != nil {
			logger.Error("failed to create journey user state", zap.Error(err))
			return err
		}

		for _, child := range children {
			next := JourneyStep{
				JourneyID:      event.JourneyID,
				JourneyEntryID: event.JourneyEntryID,
				VersionID:      event.VersionID,
				UserID:         event.UserID,
				ExternalStepID: child.ChildExternalID,
			}

			err = pub.Publish(ctx, JourneyStepSubject(state.ProjectID), next)
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
