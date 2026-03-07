package journeys

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/journey"
)

func HandleLink(ctx HandlerContext, step journey.JourneyVersionStep, state journey.JourneyUserState) (journey.JourneyUserState, journey.JourneyVersionStepChildren, error) {
	config, err := DecodeStepData[oapi.LinkStepData](step.Data)
	if err != nil {
		return state, nil, fmt.Errorf("failed to decode link step data: %w", err)
	}

	targetJourneyID, err := uuid.Parse(config.TargetId)
	if err != nil {
		return state, nil, fmt.Errorf("invalid target_id: %w", err)
	}

	journeysStore := journey.NewJourneysStore(ctx.DB)
	version, err := journeysStore.GetCurrentVersion(ctx, targetJourneyID)
	if err != nil {
		return state, nil, fmt.Errorf("failed to get current version for target journey: %w", err)
	}

	steps, err := journeysStore.GetJourneyVersionSteps(ctx, version.ID)
	if err != nil {
		return state, nil, fmt.Errorf("failed to get version steps: %w", err)
	}

	var entrance *journey.JourneyVersionStep
	for _, s := range steps {
		if s.Type == "entrance" {
			entrance = &s
			break
		}
	}

	if entrance == nil {
		return state, nil, fmt.Errorf("target journey has no entrance step")
	}

	// Check entry eligibility based on entrance step settings
	entranceData := oapi.EntranceStepData{}
	if len(entrance.Data) > 0 {
		if err := json.Unmarshal(entrance.Data, &entranceData); err != nil {
			return state, nil, fmt.Errorf("failed to decode entrance step data: %w", err)
		}
	}

	multiple := entranceData.Multiple != nil && *entranceData.Multiple
	concurrent := entranceData.Concurrent != nil && *entranceData.Concurrent

	eligible, err := journeysStore.CheckEntryEligibility(ctx, targetJourneyID, ctx.UserID, entrance.ExternalID, multiple, concurrent)
	if err != nil {
		return state, nil, fmt.Errorf("failed to check journey entry eligibility: %w", err)
	}

	if !eligible {
		// User is not eligible; complete this link step without entering the target journey
		state.CompletedAt = Now()
		return state, step.Children, nil
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
	entryState := journey.JourneyUserState{
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
		journeys := journey.NewJourneysStore(ctx.DB)
		stepType, err := journeys.GetStepType(ctx, version.ID, child.ChildExternalID)
		if err != nil {
			return state, nil, fmt.Errorf("failed to get step type: %w", err)
		}

		journeyStep := schemas.JourneyStep{
			ProjectID:      ctx.ProjectID,
			JourneyID:      targetJourneyID,
			JourneyEntryID: entry,
			VersionID:      &version.ID,
			UserID:         ctx.UserID,
			ExternalStepID: child.ChildExternalID,
			StepType:       stepType,
		}

		err = ctx.Publisher.Publish(ctx, schemas.JourneysAdvance(ctx.ProjectID, targetJourneyID, ctx.UserID), journeyStep)
		if err != nil {
			return state, nil, fmt.Errorf("failed to publish journey step: %w", err)
		}
	}

	state.CompletedAt = Now()
	return state, step.Children, nil
}
