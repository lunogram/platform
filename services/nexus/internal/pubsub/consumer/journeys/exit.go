package journeys

import (
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/store"
)

func HandleExit(ctx HandlerContext, step store.JourneyVersionStep, state store.JourneyUserState) (store.JourneyUserState, store.JourneyVersionStepChildren, error) {
	if state.CompletedAt != nil {
		return state, nil, nil
	}

	config, err := DecodeStepData[oapi.ExitStepData](step.Data)
	if err != nil {
		return state, nil, err
	}

	now := Now()

	journeys := store.NewJourneysStore(ctx.DB)
	err = journeys.CompleteJourneyEntryStates(ctx, state.JourneyID, config.EntranceUuid, *now)
	if err != nil {
		return state, nil, err
	}

	state.CompletedAt = now
	return state, nil, nil
}
