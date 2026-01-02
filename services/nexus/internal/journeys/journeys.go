package journeys

import (
	"context"
	"errors"

	"github.com/lunogram/platform/services/nexus/internal/store"
)

const (
	ActionStepType = "action"
	DelayStepType  = "delay"
)

func Handle(ctx context.Context, step store.JourneyVersionStep, state *store.JourneyUserState, data map[string]any) (store.JourneyUserState, store.JourneyVersionStepChildren, error) {
	switch step.Type {
	case ActionStepType:
		return HandleAction(ctx, step, state, data)
	case DelayStepType:
		return HandleDelay(ctx, step, state, data)
	}

	return store.JourneyUserState{}, nil, errors.New("unsupported step type")
}
