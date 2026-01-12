package journeys

import (
	"github.com/lunogram/platform/services/nexus/internal/store"
)

func HandleAction(ctx HandlerContext, step store.JourneyVersionStep, state store.JourneyUserState) (store.JourneyUserState, store.JourneyVersionStepChildren, error) {
	state.CompletedAt = Now()
	return state, step.Children, nil
}
