package journeys

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleBalancer(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Now()

	type test struct {
		step    journey.JourneyVersionStep
		state   *journey.JourneyUserState
		wantErr bool
	}

	tests := map[string]test{
		"nil state selects one child": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: "balancer",
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "child1"},
					{ChildExternalID: "child2"},
				},
			},
			state:   nil,
			wantErr: false,
		},
		"existing state returns selected child": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: "balancer",
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "child1"},
					{ChildExternalID: "child2"},
				},
			},
			state: func() *journey.JourneyUserState {
				selectedChild := journey.JourneyVersionStepChild{ChildExternalID: "child1"}
				balancerData := BalancerData{Selected: selectedChild}
				balancerDataJSON, _ := json.Marshal(balancerData)
				return &journey.JourneyUserState{
					CompletedAt: &now,
					Data:        json.RawMessage(balancerDataJSON),
				}
			}(),
			wantErr: false,
		},
		"no children returns empty": {
			step: journey.JourneyVersionStep{
				ID:       uuid.New(),
				Type:     "balancer",
				Children: []journey.JourneyVersionStepChild{},
			},
			state:   nil,
			wantErr: false,
		},
		"single child always selected": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: "balancer",
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "child1"},
				},
			},
			state:   nil,
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			hctx := HandlerContext{
				Context: ctx,
			}
			var state journey.JourneyUserState
			if tc.state != nil {
				state = *tc.state
			}
			gotState, gotChildren, err := HandleBalancer(hctx, tc.step, state)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, gotState.CompletedAt)

			if tc.state != nil {
				// When state exists, should return only the selected child
				assert.Len(t, gotChildren, 1)
				// The selected child should match what was stored in state Data
				var stateData BalancerData
				if tc.state.Data != nil {
					err := json.Unmarshal(tc.state.Data, &stateData)
					require.NoError(t, err)
					assert.Equal(t, stateData.Selected.ChildExternalID, gotChildren[0].ChildExternalID)
				}
			}

			if tc.state == nil && len(tc.step.Children) > 1 {
				// When no state, should select exactly one child
				assert.Len(t, gotChildren, 1)
				assert.Contains(t, []string{"child1", "child2"}, gotChildren[0].ChildExternalID)
				// State should have the selected child
				assert.NotNil(t, gotState.Data)
			}

			if tc.state == nil && len(tc.step.Children) == 1 {
				assert.Len(t, gotChildren, 1)
				assert.Equal(t, "child1", gotChildren[0].ChildExternalID)
				// State should have the selected child
				assert.NotNil(t, gotState.Data)
			}

			if tc.state == nil && len(tc.step.Children) == 0 {
				assert.Len(t, gotChildren, 1)
				assert.Equal(t, "", gotChildren[0].ChildExternalID)
			}
		})
	}
}

func TestSelectBalancerBranch(t *testing.T) {
	t.Parallel()

	type test struct {
		children  []journey.JourneyVersionStepChild
		checkFunc func(t *testing.T, selected journey.JourneyVersionStepChild)
		wantErr   bool
	}

	tests := map[string]test{
		"empty children returns empty": {
			children: []journey.JourneyVersionStepChild{},
			checkFunc: func(t *testing.T, selected journey.JourneyVersionStepChild) {
				assert.Equal(t, "", selected.ChildExternalID)
			},
			wantErr: false,
		},
		"single child always selected": {
			children: []journey.JourneyVersionStepChild{
				{ChildExternalID: "only-child"},
			},
			checkFunc: func(t *testing.T, selected journey.JourneyVersionStepChild) {
				assert.Equal(t, "only-child", selected.ChildExternalID)
			},
			wantErr: false,
		},
		"multiple children equal distribution": {
			children: []journey.JourneyVersionStepChild{
				{ChildExternalID: "child1"},
				{ChildExternalID: "child2"},
				{ChildExternalID: "child3"},
			},
			checkFunc: func(t *testing.T, selected journey.JourneyVersionStepChild) {
				assert.Contains(t, []string{"child1", "child2", "child3"}, selected.ChildExternalID)
			},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			selected, err := selectBalancerBranch(tc.children)

			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tc.checkFunc != nil {
				tc.checkFunc(t, selected)
			}
		})
	}
}

func TestBalancerDistribution(t *testing.T) {
	t.Parallel()

	children := []journey.JourneyVersionStepChild{
		{ChildExternalID: "child1"},
		{ChildExternalID: "child2"},
		{ChildExternalID: "child3"},
	}

	counts := make(map[string]int)
	iterations := 3000

	for i := 0; i < iterations; i++ {
		selected, err := selectBalancerBranch(children)
		require.NoError(t, err)
		counts[selected.ChildExternalID]++
	}

	// With equal weights (1.0 for all), each should get roughly 33%
	// Allow 25% to 42% range (should be very unlikely to fail with 3000 iterations)
	minExpected := iterations * 25 / 100
	maxExpected := iterations * 42 / 100

	for _, child := range children {
		count := counts[child.ChildExternalID]
		assert.GreaterOrEqual(t, count, minExpected, "child %s got %d selections, expected at least %d", child.ChildExternalID, count, minExpected)
		assert.LessOrEqual(t, count, maxExpected, "child %s got %d selections, expected at most %d", child.ChildExternalID, count, maxExpected)
	}
}
