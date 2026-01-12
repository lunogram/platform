package journeys

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"math"
	"math/big"

	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/store"
)

type ExperimentData struct {
	Selected store.JourneyVersionStepChild `json:"selected_child_id"`
}

func HandleExperiment(ctx HandlerContext, step store.JourneyVersionStep, state store.JourneyUserState) (store.JourneyUserState, store.JourneyVersionStepChildren, error) {
	if state.CompletedAt != nil {
		var experiment ExperimentData
		err := json.Unmarshal(state.Data, &experiment)
		if err != nil {
			return state, nil, err
		}

		return state, []store.JourneyVersionStepChild{experiment.Selected}, nil
	}

	selected, err := selectExperimentBranch(step.Children)
	if err != nil {
		return state, nil, err
	}

	state.CompletedAt = Now()
	state, err = WithStateData(state, ExperimentData{
		Selected: selected,
	})
	if err != nil {
		return state, nil, err
	}

	return state, []store.JourneyVersionStepChild{selected}, nil
}

func selectExperimentBranch(children []store.JourneyVersionStepChild) (store.JourneyVersionStepChild, error) {
	return selectWeightedBranch(children, func(child store.JourneyVersionStepChild) (float32, error) {
		if child.Data == nil {
			return 1.0, nil
		}

		experiment, err := DecodeStepData[oapi.ExperimentChildData](*child.Data)
		if err != nil {
			return 0, err
		}

		return experiment.Ratio, nil
	})
}

// selectWeightedBranch selects a child branch based on weighted ratios
// The ratioExtractor function extracts the weight from each child's data
func selectWeightedBranch(children []store.JourneyVersionStepChild, ratioExtractor func(store.JourneyVersionStepChild) (float32, error)) (store.JourneyVersionStepChild, error) {
	if len(children) == 0 {
		return store.JourneyVersionStepChild{}, nil
	}

	ratios := make([]float32, len(children))
	totalRatio := float32(0)

	for i, child := range children {
		ratio, err := ratioExtractor(child)
		if err != nil {
			return store.JourneyVersionStepChild{}, err
		}

		if ratio < 0 || math.IsNaN(float64(ratio)) || math.IsInf(float64(ratio), 0) {
			return store.JourneyVersionStepChild{}, errors.New("invalid ratio: must be non-negative finite number")
		}

		ratios[i] = ratio
		totalRatio += ratio
	}

	if totalRatio == 0 {
		return store.JourneyVersionStepChild{}, errors.New("total ratio cannot be zero")
	}

	randomValue, err := cryptoRandFloat64()
	if err != nil {
		return store.JourneyVersionStepChild{}, err
	}

	randomValue *= float64(totalRatio)
	cumulativeRatio := float64(0)

	for i, ratio := range ratios {
		cumulativeRatio += float64(ratio)
		if randomValue < cumulativeRatio {
			return children[i], nil
		}
	}

	return children[len(children)-1], nil
}

// cryptoRandFloat64 generates a cryptographically secure random float64 in [0, 1)
// using unbiased bit sampling
func cryptoRandFloat64() (float64, error) {
	// Generate 53 random bits (mantissa precision of float64)
	max := big.NewInt(1 << 53)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return 0, err
	}

	// Scale to [0, 1) with full precision
	return float64(n.Uint64()) / float64(uint64(1)<<53), nil
}
