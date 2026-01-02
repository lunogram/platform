package journeys

import (
	"context"

	"github.com/lunogram/platform/services/nexus/internal/store"
)

func HandleAction(ctx context.Context, step store.JourneyVersionStep, state *store.JourneyUserState, data map[string]any) (store.JourneyUserState, store.JourneyVersionStepChildren, error) {
	return store.JourneyUserState{}, nil, nil
}
