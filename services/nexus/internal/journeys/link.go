package journeys

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/pubsub/schemas"
	"github.com/lunogram/platform/services/nexus/internal/store"
)

func HandleLink(ctx HandlerContext, step store.JourneyVersionStep, state store.JourneyUserState) (store.JourneyUserState, store.JourneyVersionStepChildren, error) {
	config, err := DecodeStepData[oapi.LinkStepData](step.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to decode link step data: %w", err)
	}

	targetJourneyID, err := uuid.Parse(config.TargetId)
	if err != nil {
		return state, nil, fmt.Errorf("invalid target_id: %w", err)
	}

	journeysStore := store.NewJourneysStore(ctx.DB)
	version, err := journeysStore.GetCurrentVersion(ctx, targetJourneyID)
	if err != nil {
		return state, nil, fmt.Errorf("failed to get current version for target journey: %w", err)
	}

	steps, err := journeysStore.GetJourneyVersionSteps(ctx, version.ID)
	if err != nil {
		return state, nil, fmt.Errorf("failed to get version steps: %w", err)
	}

	var entrance *store.JourneyVersionStep
	for _, s := range steps {
		if s.Type == "entrance" {
			entrance = &s
			break
		}
	}

	if entrance == nil {
		return state, nil, fmt.Errorf("target journey has no entrance step")
	}

	entry, err := uuid.NewRandom()
	if err != nil {
		return state, nil, fmt.Errorf("failed to generate journey entry ID: %w", err)
	}

	data, err := json.Marshal(ctx.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to marshal journey entry data: %w", err)
	}

	completedAt := Now()
	entryState := store.JourneyUserState{
		JourneyID:       targetJourneyID,
		JourneyEntryID:  entry,
		PinnedVersionID: &version.ID,
		UserID:          ctx.UserID,
		ExternalStepID:  entrance.ExternalID,
		Data:            json.RawMessage(data),
		CompletedAt:     completedAt,
	}

	_, err = journeysStore.CreateUserJourneyState(ctx, entryState)
	if err != nil {
		return state, nil, fmt.Errorf("failed to create journey user state: %w", err)
	}

	for _, child := range entrance.Children {
		journeyStep := schemas.JourneyStep{
			ProjectID:      ctx.ProjectID,
			JourneyID:      targetJourneyID,
			JourneyEntryID: entry,
			VersionID:      &version.ID,
			UserID:         ctx.UserID,
			ExternalStepID: child.ChildExternalID,
		}

		err = ctx.Publisher.Publish(ctx, schemas.JourneysAdvance(ctx.ProjectID, targetJourneyID), journeyStep)
		if err != nil {
			return state, nil, fmt.Errorf("failed to publish journey step: %w", err)
		}
	}

	state.CompletedAt = Now()
	return state, step.Children, nil
}
