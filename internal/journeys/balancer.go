package journeys

import (
	"encoding/json"

	"github.com/lunogram/platform/internal/store"
)

type BalancerData struct {
	Selected store.JourneyVersionStepChild `json:"selected"`
}

func HandleBalancer(ctx HandlerContext, step store.JourneyVersionStep, state store.JourneyUserState) (store.JourneyUserState, store.JourneyVersionStepChildren, error) {
	if state.CompletedAt != nil {
		var balancer BalancerData
		err := json.Unmarshal(state.Data, &balancer)
		if err != nil {
			return state, nil, err
		}

		return state, []store.JourneyVersionStepChild{balancer.Selected}, nil
	}

	selected, err := selectBalancerBranch(step.Children)
	if err != nil {
		return state, nil, err
	}

	state.CompletedAt = Now()
	state, err = WithStateData(state, BalancerData{
		Selected: selected,
	})
	if err != nil {
		return state, nil, err
	}

	return state, []store.JourneyVersionStepChild{selected}, nil
}

func selectBalancerBranch(children []store.JourneyVersionStepChild) (store.JourneyVersionStepChild, error) {
	// NOTE: balancer uses equal weights for all branches (1.0)
	return selectWeightedBranch(children, func(child store.JourneyVersionStepChild) (float32, error) {
		return 1.0, nil
	})
}
