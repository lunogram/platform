package journeys

import (
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store/journey"
)

func HandleExit(ctx HandlerContext, step journey.JourneyVersionStep, state journey.JourneyUserState) (journey.JourneyUserState, journey.JourneyVersionStepChildren, error) {
	if state.CompletedAt != nil {
		return state, nil, nil
	}

	config, err := DecodeStepData[oapi.ExitStepData](step.Data)
	if err != nil {
		return state, nil, err
	}

	now := Now()

	journeys := journey.NewJourneysStore(ctx.DB)
	err = journeys.CompleteJourneyEntryStates(ctx, state.JourneyID, config.EntranceUuid, *now)
	if err != nil {
		return state, nil, err
	}

	state.CompletedAt = now
	return state, nil, nil
}
