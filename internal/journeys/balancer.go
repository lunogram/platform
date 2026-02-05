package journeys

import (
	"encoding/json"

	"github.com/lunogram/platform/internal/store/journey"
)

type BalancerData struct {
	Selected journey.JourneyVersionStepChild `json:"selected"`
}

func HandleBalancer(ctx HandlerContext, step journey.JourneyVersionStep, state journey.JourneyUserState) (journey.JourneyUserState, journey.JourneyVersionStepChildren, error) {
	if state.CompletedAt != nil {
		var balancer BalancerData
		err := json.Unmarshal(state.Data, &balancer)
		if err != nil {
			return state, nil, err
		}

		return state, []journey.JourneyVersionStepChild{balancer.Selected}, nil
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

	return state, []journey.JourneyVersionStepChild{selected}, nil
}

func selectBalancerBranch(children []journey.JourneyVersionStepChild) (journey.JourneyVersionStepChild, error) {
	// NOTE: balancer uses equal weights for all branches (1.0)
	return selectWeightedBranch(children, func(child journey.JourneyVersionStepChild) (float32, error) {
		return 1.0, nil
	})
}
