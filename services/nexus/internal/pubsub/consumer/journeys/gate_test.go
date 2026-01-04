package journeys

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/rules"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleGate(t *testing.T) {
	ctx := context.Background()

	projectID := uuid.New()
	userID := uuid.New()

	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db := sqlx.NewDb(mockDB, "sqlmock")

	type test struct {
		step           store.JourneyVersionStep
		state          *store.JourneyUserState
		data           map[string]any
		matchesRule    bool
		expectedPath   string
		expectedErr    bool
		expectedResult bool
	}

	tests := map[string]test{
		"nil state with rule match follows yes path": {
			step: store.JourneyVersionStep{
				Type: GateStepType,
				Data: json.RawMessage(`{"rule":{"type":"wrapper","group":"user","operator":"and","children":[]}}`),
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "child1", Path: ptr("yes")},
					{ChildExternalID: "child2", Path: ptr("no")},
				},
			},
			state: nil,
			data: map[string]any{
				"user_id":    userID.String(),
				"project_id": projectID.String(),
			},
			matchesRule:    true,
			expectedPath:   "yes",
			expectedErr:    false,
			expectedResult: true,
		},
		"nil state with rule mismatch follows no path": {
			step: store.JourneyVersionStep{
				Type: GateStepType,
				Data: json.RawMessage(`{"rule":{"type":"wrapper","group":"user","operator":"and","children":[]}}`),
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "child1", Path: ptr("yes")},
					{ChildExternalID: "child2", Path: ptr("no")},
				},
			},
			state: nil,
			data: map[string]any{
				"user_id":    userID.String(),
				"project_id": projectID.String(),
			},
			matchesRule:    false,
			expectedPath:   "no",
			expectedErr:    false,
			expectedResult: true,
		},
		"existing state returns selected child": {
			step: store.JourneyVersionStep{
				Type: GateStepType,
				Data: json.RawMessage(`{"rule":{"type":"wrapper","group":"user","operator":"and","children":[]}}`),
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "child1", Path: ptr("yes")},
					{ChildExternalID: "child2", Path: ptr("no")},
				},
			},
			state: &store.JourneyUserState{
				CompletedAt: nil, // Not completed yet
				Data:        json.RawMessage(`{"selected_child":"child1"}`),
			},
			data: map[string]any{
				"user_id":    userID.String(),
				"project_id": projectID.String(),
			},
			matchesRule:    true, // Rule matches = yes path
			expectedPath:   "yes",
			expectedErr:    false,
			expectedResult: true,
		},
		"no children returns no result": {
			step: store.JourneyVersionStep{
				Type:     GateStepType,
				Data:     json.RawMessage(`{"rule":{"type":"wrapper","group":"user","operator":"and","children":[]}}`),
				Children: []store.JourneyVersionStepChild{},
			},
			state: nil,
			data: map[string]any{
				"user_id":    userID.String(),
				"project_id": projectID.String(),
			},
			matchesRule:    true,
			expectedPath:   "",
			expectedErr:    false,
			expectedResult: false, // No children so no result
		},
		"no matching path returns nil": {
			step: store.JourneyVersionStep{
				Type: GateStepType,
				Data: json.RawMessage(`{"rule":{"type":"wrapper","group":"user","operator":"and","children":[]}}`),
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "child1", Path: ptr("other")},
					{ChildExternalID: "child2", Path: ptr("another")},
				},
			},
			state: nil,
			data: map[string]any{
				"user_id":    userID.String(),
				"project_id": projectID.String(),
			},
			matchesRule:    true,
			expectedPath:   "",
			expectedErr:    false,
			expectedResult: false, // No yes/no paths so returns nil
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup mock expectations for new states (not existing state test)
			// Don't set up mocks if there are no children (early return) or if state is completed
			if tc.state == nil || (tc.state != nil && tc.state.CompletedAt == nil) {
				if len(tc.step.Children) > 0 {
					if tc.matchesRule {
						mock.ExpectQuery("SELECT u.id FROM users u").
							WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID.String()))
					} else {
						mock.ExpectQuery("SELECT u.id FROM users u").
							WillReturnError(sqlmock.ErrCancelled)
					}
				}
			}

			hctx := HandlerContext{
				Context:   ctx,
				DB:        db,
				ProjectID: projectID,
				UserID:    userID,
				Data:      tc.data,
			}
			var state store.JourneyUserState
			if tc.state != nil {
				state = *tc.state
			}
			gotState, gotChildren, err := HandleGate(hctx, tc.step, state)

			if tc.expectedErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tc.expectedResult {
				assert.NotNil(t, gotState.CompletedAt)
				assert.NotNil(t, gotState.Data)

				require.Len(t, gotChildren, 1)

				if tc.state != nil {
					// Verify we got the stored child back
					var gateData GateData
					err := json.Unmarshal(tc.state.Data, &gateData)
					require.NoError(t, err)
					assert.Equal(t, gateData.SelectedExternalID, gotChildren[0].ChildExternalID)
				} else if tc.expectedPath != "" {
					// Verify we got the correct path
					assert.Equal(t, tc.expectedPath, *gotChildren[0].Path)
				}

				// Verify gate data structure
				var gateData GateData
				err = json.Unmarshal(gotState.Data, &gateData)
				require.NoError(t, err)
				assert.Equal(t, gotChildren[0].ChildExternalID, gateData.SelectedExternalID)
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSelectGateBranch(t *testing.T) {
	ctx := context.Background()

	projectID := uuid.New()
	userID := uuid.New()

	ruleSet := rules.RuleSet{
		Rule: rules.Rule{
			Type:     rules.RuleTypeWrapper,
			Group:    rules.RuleGroupUser,
			Operator: rules.OperatorAnd,
			Children: []rules.Rule{},
		},
	}

	type test struct {
		children        store.JourneyVersionStepChildren
		matchesRule     bool
		expectedPath    *string
		expectedErr     bool
		expectedChildID string
	}

	tests := map[string]test{
		"empty children returns nil": {
			children:    []store.JourneyVersionStepChild{},
			matchesRule: true,
			expectedErr: false,
		},
		"single yes child when rule matches": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child1", Path: ptr("yes")},
			},
			matchesRule:     true,
			expectedPath:    ptr("yes"),
			expectedErr:     false,
			expectedChildID: "child1",
		},
		"single no child when rule mismatches": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child2", Path: ptr("no")},
			},
			matchesRule:     false,
			expectedPath:    ptr("no"),
			expectedErr:     false,
			expectedChildID: "child2",
		},
		"yes path selected when rule matches with both paths": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child1", Path: ptr("yes")},
				{ChildExternalID: "child2", Path: ptr("no")},
			},
			matchesRule:     true,
			expectedPath:    ptr("yes"),
			expectedErr:     false,
			expectedChildID: "child1",
		},
		"no path selected when rule mismatches with both paths": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child1", Path: ptr("yes")},
				{ChildExternalID: "child2", Path: ptr("no")},
			},
			matchesRule:     false,
			expectedPath:    ptr("no"),
			expectedErr:     false,
			expectedChildID: "child2",
		},
		"no yes/no paths returns nil": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child1", Path: ptr("other")},
				{ChildExternalID: "child2", Path: ptr("another")},
			},
			matchesRule:     true,
			expectedPath:    nil,
			expectedErr:     false,
			expectedChildID: "", // Expect nil result
		},
		"yes path selected with nil path sibling": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child1", Path: nil},
				{ChildExternalID: "child2", Path: ptr("yes")},
			},
			matchesRule:     true, // Rule matches, has "yes" path
			expectedPath:    ptr("yes"),
			expectedErr:     false,
			expectedChildID: "child2", // Selects yes path
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// Create mock DB for each subtest
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()
			db := sqlx.NewDb(mockDB, "sqlmock")

			if !tc.expectedErr {
				// Use ExpectQuery with regex to match any WHERE clause
				if tc.matchesRule {
					mock.ExpectQuery("^SELECT u\\.id FROM users u WHERE").
						WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID.String()))
				} else {
					mock.ExpectQuery("^SELECT u\\.id FROM users u WHERE").
						WillReturnError(sqlmock.ErrCancelled)
				}
			}

			hctx := HandlerContext{
				Context:   ctx,
				DB:        db,
				ProjectID: projectID,
				UserID:    userID,
			}

			step := store.JourneyVersionStep{
				Type:     GateStepType,
				Children: tc.children,
				Data:     json.RawMessage(`{"rule":{"type":"wrapper","group":"user","operator":"and","children":[]}}`),
			}

			gateData := oapi.GateStepData{
				Rule: ruleSet,
			}

			child, err := selectGateBranch(hctx, step, gateData)

			if tc.expectedErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tc.expectedChildID == "" {
				// Expecting nil child
				assert.Nil(t, child)
				return
			}

			require.NotNil(t, child, "expected child, got nil")
			assert.Equal(t, tc.expectedChildID, child.ChildExternalID)
			if tc.expectedPath != nil {
				require.NotNil(t, child.Path)
				assert.Equal(t, *tc.expectedPath, *child.Path)
			}

			// Verify all expectations were met
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
