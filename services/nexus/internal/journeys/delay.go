package journeys

import (
	"context"
	"fmt"
	"time"

	"github.com/lunogram/platform/services/nexus/internal/store"
)

func HandleDelay(ctx context.Context, step store.JourneyVersionStep, state *store.JourneyUserState, data map[string]any) (store.JourneyUserState, store.JourneyVersionStepChildren, error) {
	now := time.Now()

	if state == nil {
		delay := store.JourneyUserState{
			ResumeAt: &now,
		}

		return delay, nil, nil
	}

	fmt.Println("------------", step.Children)
	state.CompletedAt = &now
	return *state, step.Children, nil
}
