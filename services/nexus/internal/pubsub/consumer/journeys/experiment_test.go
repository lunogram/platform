package journeys

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleExperiment(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Now()

	type test struct {
		step    store.JourneyVersionStep
		state   *store.JourneyUserState
		wantErr bool
	}

	childData1 := oapi.ExperimentChildData{Ratio: 0.5}
	childData1JSON, _ := json.Marshal(childData1)
	childData1Raw := json.RawMessage(childData1JSON)

	childData2 := oapi.ExperimentChildData{Ratio: 0.5}
	childData2JSON, _ := json.Marshal(childData2)
	childData2Raw := json.RawMessage(childData2JSON)

	tests := map[string]test{
		"nil state selects one child": {
			step: store.JourneyVersionStep{
				ID:   uuid.New(),
				Type: "experiment",
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "child1", Data: &childData1Raw},
					{ChildExternalID: "child2", Data: &childData2Raw},
				},
			},
			state:   nil,
			wantErr: false,
		},
		"existing state returns selected child": {
			step: store.JourneyVersionStep{
				ID:   uuid.New(),
				Type: "experiment",
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "child1", Data: &childData1Raw},
					{ChildExternalID: "child2", Data: &childData2Raw},
				},
			},
			state: func() *store.JourneyUserState {
				selectedChild := store.JourneyVersionStepChild{ChildExternalID: "child1", Data: &childData1Raw}
				expData := ExperimentData{Selected: selectedChild}
				expDataJSON, _ := json.Marshal(expData)
				return &store.JourneyUserState{
					CompletedAt: &now,
					Data:        json.RawMessage(expDataJSON),
				}
			}(),
			wantErr: false,
		},
		"no children returns empty": {
			step: store.JourneyVersionStep{
				ID:       uuid.New(),
				Type:     "experiment",
				Children: []store.JourneyVersionStepChild{},
			},
			state:   nil,
			wantErr: false,
		},
		"single child always selected": {
			step: store.JourneyVersionStep{
				ID:   uuid.New(),
				Type: "experiment",
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "child1", Data: &childData1Raw},
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
			var state store.JourneyUserState
			if tc.state != nil {
				state = *tc.state
			}
			gotState, gotChildren, err := HandleExperiment(hctx, tc.step, state)

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
				var stateData ExperimentData
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
				// State should have the selected child ID
				assert.NotNil(t, gotState.Data)
			}

			if tc.state == nil && len(tc.step.Children) == 1 {
				assert.Len(t, gotChildren, 1)
				assert.Equal(t, "child1", gotChildren[0].ChildExternalID)
				// State should have the selected child ID
				assert.NotNil(t, gotState.Data)
			}

			if tc.state == nil && len(tc.step.Children) == 0 {
				assert.Len(t, gotChildren, 1)
				assert.Equal(t, "", gotChildren[0].ChildExternalID)
			}
		})
	}
}

func TestSelectExperimentBranch(t *testing.T) {
	t.Parallel()

	type test struct {
		children  []store.JourneyVersionStepChild
		checkFunc func(t *testing.T, selected store.JourneyVersionStepChild)
		wantErr   bool
	}

	childData1 := oapi.ExperimentChildData{Ratio: 0.7}
	childData1JSON, _ := json.Marshal(childData1)
	childData1Raw := json.RawMessage(childData1JSON)

	childData2 := oapi.ExperimentChildData{Ratio: 0.3}
	childData2JSON, _ := json.Marshal(childData2)
	childData2Raw := json.RawMessage(childData2JSON)

	equalRatio := oapi.ExperimentChildData{Ratio: 1.0}
	equalRatioJSON, _ := json.Marshal(equalRatio)
	equalRatioRaw := json.RawMessage(equalRatioJSON)

	negativeRatio := oapi.ExperimentChildData{Ratio: -1.0}
	negativeRatioJSON, _ := json.Marshal(negativeRatio)
	negativeRatioRaw := json.RawMessage(negativeRatioJSON)

	tests := map[string]test{
		"empty children returns empty": {
			children: []store.JourneyVersionStepChild{},
			checkFunc: func(t *testing.T, selected store.JourneyVersionStepChild) {
				assert.Equal(t, "", selected.ChildExternalID)
			},
			wantErr: false,
		},
		"single child always selected": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "only-child", Data: &childData1Raw},
			},
			checkFunc: func(t *testing.T, selected store.JourneyVersionStepChild) {
				assert.Equal(t, "only-child", selected.ChildExternalID)
			},
			wantErr: false,
		},
		"multiple children with ratios": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child1", Data: &childData1Raw},
				{ChildExternalID: "child2", Data: &childData2Raw},
			},
			checkFunc: func(t *testing.T, selected store.JourneyVersionStepChild) {
				assert.Contains(t, []string{"child1", "child2"}, selected.ChildExternalID)
			},
			wantErr: false,
		},
		"equal ratios distribution": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child1", Data: &equalRatioRaw},
				{ChildExternalID: "child2", Data: &equalRatioRaw},
				{ChildExternalID: "child3", Data: &equalRatioRaw},
			},
			checkFunc: func(t *testing.T, selected store.JourneyVersionStepChild) {
				assert.Contains(t, []string{"child1", "child2", "child3"}, selected.ChildExternalID)
			},
			wantErr: false,
		},
		"missing data defaults to ratio 1": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child1"},
				{ChildExternalID: "child2"},
			},
			checkFunc: func(t *testing.T, selected store.JourneyVersionStepChild) {
				assert.Contains(t, []string{"child1", "child2"}, selected.ChildExternalID)
			},
			wantErr: false,
		},
		"negative ratio returns error": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child1", Data: &negativeRatioRaw},
			},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			selected, err := selectExperimentBranch(tc.children)

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

func TestSelectExperimentBranchDistribution(t *testing.T) {
	t.Parallel()

	childData1 := oapi.ExperimentChildData{Ratio: 0.7}
	childData1JSON, _ := json.Marshal(childData1)
	childData1Raw := json.RawMessage(childData1JSON)

	childData2 := oapi.ExperimentChildData{Ratio: 0.3}
	childData2JSON, _ := json.Marshal(childData2)
	childData2Raw := json.RawMessage(childData2JSON)

	children := []store.JourneyVersionStepChild{
		{ChildExternalID: "child1", Data: &childData1Raw},
		{ChildExternalID: "child2", Data: &childData2Raw},
	}

	iterations := 1000
	child1Count := 0
	child2Count := 0

	for i := 0; i < iterations; i++ {
		selected, err := selectExperimentBranch(children)
		require.NoError(t, err)

		if selected.ChildExternalID == "child1" {
			child1Count++
		} else if selected.ChildExternalID == "child2" {
			child2Count++
		}
	}

	child1Ratio := float64(child1Count) / float64(iterations)
	child2Ratio := float64(child2Count) / float64(iterations)

	assert.InDelta(t, 0.7, child1Ratio, 0.1, "child1 should be selected ~70%% of the time")
	assert.InDelta(t, 0.3, child2Ratio, 0.1, "child2 should be selected ~30%% of the time")
}

func TestCryptoRandFloat64(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		val, err := cryptoRandFloat64()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, val, 0.0)
		assert.Less(t, val, 1.0)
	}
}
