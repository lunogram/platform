package journeys

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHandleGate(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	projectID := uuid.New()
	userID := uuid.New()

	// Create mock DB
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer mockDB.Close()
	db := sqlx.NewDb(mockDB, "sqlmock")

	type test struct {
		step           journey.JourneyVersionStep
		state          *journey.JourneyUserState
		data           map[string]any
		matchesRule    bool
		expectedPath   string
		expectedErr    bool
		expectedResult bool
	}

	tests := map[string]test{
		"nil state with rule match follows yes path": {
			step: journey.JourneyVersionStep{
				Type: GateStepType,
				Data: json.RawMessage(`{"rule":{"type":"wrapper","group":"user","operator":"and","children":[]}}`),
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "child1", Path: ptr.To("yes")},
					{ChildExternalID: "child2", Path: ptr.To("no")},
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
			step: journey.JourneyVersionStep{
				Type: GateStepType,
				Data: json.RawMessage(`{"rule":{"type":"wrapper","group":"user","operator":"and","children":[]}}`),
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "child1", Path: ptr.To("yes")},
					{ChildExternalID: "child2", Path: ptr.To("no")},
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
			step: journey.JourneyVersionStep{
				Type: GateStepType,
				Data: json.RawMessage(`{"rule":{"type":"wrapper","group":"user","operator":"and","children":[]}}`),
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "child1", Path: ptr.To("yes")},
					{ChildExternalID: "child2", Path: ptr.To("no")},
				},
			},
			state: &journey.JourneyUserState{
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
			step: journey.JourneyVersionStep{
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
			step: journey.JourneyVersionStep{
				Type: GateStepType,
				Data: json.RawMessage(`{"rule":{"type":"wrapper","group":"user","operator":"and","children":[]}}`),
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "child1", Path: ptr.To("other")},
					{ChildExternalID: "child2", Path: ptr.To("another")},
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
				logger:    logger,
			}
			var state journey.JourneyUserState
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
	logger := zap.NewNop()
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
				{ChildExternalID: "child1", Path: ptr.To("yes")},
			},
			matchesRule:     true,
			expectedPath:    ptr.To("yes"),
			expectedErr:     false,
			expectedChildID: "child1",
		},
		"single no child when rule mismatches": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child2", Path: ptr.To("no")},
			},
			matchesRule:     false,
			expectedPath:    ptr.To("no"),
			expectedErr:     false,
			expectedChildID: "child2",
		},
		"yes path selected when rule matches with both paths": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child1", Path: ptr.To("yes")},
				{ChildExternalID: "child2", Path: ptr.To("no")},
			},
			matchesRule:     true,
			expectedPath:    ptr.To("yes"),
			expectedErr:     false,
			expectedChildID: "child1",
		},
		"no path selected when rule mismatches with both paths": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child1", Path: ptr.To("yes")},
				{ChildExternalID: "child2", Path: ptr.To("no")},
			},
			matchesRule:     false,
			expectedPath:    ptr.To("no"),
			expectedErr:     false,
			expectedChildID: "child2",
		},
		"no yes/no paths returns nil": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child1", Path: ptr.To("other")},
				{ChildExternalID: "child2", Path: ptr.To("another")},
			},
			matchesRule:     true,
			expectedPath:    nil,
			expectedErr:     false,
			expectedChildID: "", // Expect nil result
		},
		"yes path selected with nil path sibling": {
			children: []store.JourneyVersionStepChild{
				{ChildExternalID: "child1", Path: nil},
				{ChildExternalID: "child2", Path: ptr.To("yes")},
			},
			matchesRule:     true, // Rule matches, has "yes" path
			expectedPath:    ptr.To("yes"),
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
				logger:    logger,
			}

			step := journey.JourneyVersionStep{
				Type:     GateStepType,
				Children: tc.children,
				Data:     json.RawMessage(`{"rule":{"type":"wrapper","group":"user","operator":"and","children":[]}}`),
			}

			gateData := oapi.GateStepData{
				Rule: ruleSet,
			}

			child, err := selectGateBranch(hctx, step, gateData, journey.JourneyUserState{})

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

func TestEvaluateGateRules(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	projectID := uuid.New()
	userID := uuid.New()

	t.Run("journey-only rules: match", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		rs := rules.RuleSet{
			Rule: rules.Rule{
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupParent,
				Operator: rules.OperatorAnd,
				Children: []rules.Rule{
					{
						Type:     rules.RuleTypeNumber,
						Group:    rules.RuleGroupJourney,
						Path:     "journey.entrance.amount",
						Operator: rules.OperatorGreaterThan,
						Value:    "50",
					},
				},
			},
		}

		hctx := HandlerContext{
			Context:   ctx,
			DB:        db,
			ProjectID: projectID,
			UserID:    userID,
			logger:    logger,
			Data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"amount": float64(100),
					},
				},
			},
		}

		result, err := evaluateGateRules(hctx, rs, journey.JourneyUserState{})
		require.NoError(t, err)
		assert.True(t, result)

		// No DB expectations should have been set
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("journey-only rules: no match", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		rs := rules.RuleSet{
			Rule: rules.Rule{
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupParent,
				Operator: rules.OperatorAnd,
				Children: []rules.Rule{
					{
						Type:     rules.RuleTypeNumber,
						Group:    rules.RuleGroupJourney,
						Path:     "journey.entrance.amount",
						Operator: rules.OperatorGreaterThan,
						Value:    "200",
					},
				},
			},
		}

		hctx := HandlerContext{
			Context:   ctx,
			DB:        db,
			ProjectID: projectID,
			UserID:    userID,
			logger:    logger,
			Data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"amount": float64(100),
					},
				},
			},
		}

		result, err := evaluateGateRules(hctx, rs, journey.JourneyUserState{})
		require.NoError(t, err)
		assert.False(t, result)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("db-only rules: passes through to SQL", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		rs := rules.RuleSet{
			Rule: rules.Rule{
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupUser,
				Operator: rules.OperatorAnd,
				Children: []rules.Rule{},
			},
		}

		mock.ExpectQuery("^SELECT u\\.id FROM users u WHERE").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID.String()))

		hctx := HandlerContext{
			Context:   ctx,
			DB:        db,
			ProjectID: projectID,
			UserID:    userID,
			logger:    logger,
			Data:      map[string]any{},
		}

		result, err := evaluateGateRules(hctx, rs, journey.JourneyUserState{})
		require.NoError(t, err)
		assert.True(t, result)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("mixed AND: journey matches, DB matches", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		rs := rules.RuleSet{
			Rule: rules.Rule{
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupParent,
				Operator: rules.OperatorAnd,
				Children: []rules.Rule{
					{
						Type:     rules.RuleTypeNumber,
						Group:    rules.RuleGroupJourney,
						Path:     "journey.entrance.amount",
						Operator: rules.OperatorGreaterThan,
						Value:    "50",
					},
					{
						Type:     rules.RuleTypeString,
						Group:    rules.RuleGroupUser,
						Path:     ".email",
						Operator: rules.OperatorContains,
						Value:    "test",
					},
				},
			},
		}

		mock.ExpectQuery("^SELECT u\\.id FROM users u WHERE").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID.String()))

		hctx := HandlerContext{
			Context:   ctx,
			DB:        db,
			ProjectID: projectID,
			UserID:    userID,
			logger:    logger,
			Data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"amount": float64(100),
					},
				},
			},
		}

		result, err := evaluateGateRules(hctx, rs, journey.JourneyUserState{})
		require.NoError(t, err)
		assert.True(t, result)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("mixed AND: journey fails, short-circuits DB", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		rs := rules.RuleSet{
			Rule: rules.Rule{
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupParent,
				Operator: rules.OperatorAnd,
				Children: []rules.Rule{
					{
						Type:     rules.RuleTypeNumber,
						Group:    rules.RuleGroupJourney,
						Path:     "journey.entrance.amount",
						Operator: rules.OperatorGreaterThan,
						Value:    "200",
					},
					{
						Type:     rules.RuleTypeString,
						Group:    rules.RuleGroupUser,
						Path:     ".email",
						Operator: rules.OperatorContains,
						Value:    "test",
					},
				},
			},
		}

		// No DB expectation — query should be short-circuited

		hctx := HandlerContext{
			Context:   ctx,
			DB:        db,
			ProjectID: projectID,
			UserID:    userID,
			logger:    logger,
			Data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"amount": float64(10),
					},
				},
			},
		}

		result, err := evaluateGateRules(hctx, rs, journey.JourneyUserState{})
		require.NoError(t, err)
		assert.False(t, result)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("mixed OR: journey matches, short-circuits DB", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		rs := rules.RuleSet{
			Rule: rules.Rule{
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupParent,
				Operator: rules.OperatorOr,
				Children: []rules.Rule{
					{
						Type:     rules.RuleTypeNumber,
						Group:    rules.RuleGroupJourney,
						Path:     "journey.entrance.amount",
						Operator: rules.OperatorGreaterThan,
						Value:    "50",
					},
					{
						Type:     rules.RuleTypeString,
						Group:    rules.RuleGroupUser,
						Path:     ".email",
						Operator: rules.OperatorContains,
						Value:    "test",
					},
				},
			},
		}

		// No DB expectation — short-circuited because journey matches

		hctx := HandlerContext{
			Context:   ctx,
			DB:        db,
			ProjectID: projectID,
			UserID:    userID,
			logger:    logger,
			Data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"amount": float64(100),
					},
				},
			},
		}

		result, err := evaluateGateRules(hctx, rs, journey.JourneyUserState{})
		require.NoError(t, err)
		assert.True(t, result)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("mixed OR: journey fails, DB matches", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		rs := rules.RuleSet{
			Rule: rules.Rule{
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupParent,
				Operator: rules.OperatorOr,
				Children: []rules.Rule{
					{
						Type:     rules.RuleTypeNumber,
						Group:    rules.RuleGroupJourney,
						Path:     "journey.entrance.amount",
						Operator: rules.OperatorGreaterThan,
						Value:    "200",
					},
					{
						Type:     rules.RuleTypeString,
						Group:    rules.RuleGroupUser,
						Path:     ".email",
						Operator: rules.OperatorContains,
						Value:    "test",
					},
				},
			},
		}

		mock.ExpectQuery("^SELECT u\\.id FROM users u WHERE").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID.String()))

		hctx := HandlerContext{
			Context:   ctx,
			DB:        db,
			ProjectID: projectID,
			UserID:    userID,
			logger:    logger,
			Data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"amount": float64(10),
					},
				},
			},
		}

		result, err := evaluateGateRules(hctx, rs, journey.JourneyUserState{})
		require.NoError(t, err)
		assert.True(t, result)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("journey string equals condition", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		rs := rules.RuleSet{
			Rule: rules.Rule{
				Type:     rules.RuleTypeWrapper,
				Group:    rules.RuleGroupParent,
				Operator: rules.OperatorAnd,
				Children: []rules.Rule{
					{
						Type:     rules.RuleTypeString,
						Group:    rules.RuleGroupJourney,
						Path:     "journey.entrance.plan",
						Operator: rules.OperatorEquals,
						Value:    "enterprise",
					},
				},
			},
		}

		hctx := HandlerContext{
			Context:   ctx,
			DB:        db,
			ProjectID: projectID,
			UserID:    userID,
			logger:    logger,
			Data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"plan": "enterprise",
					},
				},
			},
		}

		result, err := evaluateGateRules(hctx, rs, journey.JourneyUserState{})
		require.NoError(t, err)
		assert.True(t, result)

		assert.NoError(t, mock.ExpectationsWereMet())
	})

	// This test simulates the exact production scenario:
	// 1. Rule JSON from frontend (as stored in DB)
	// 2. Data map from GetJourneyEntryData (JSON round-tripped through DB)
	// 3. Full HandleGate flow: DecodeStepData → RenderRuleSet → evaluateGateRules
	t.Run("JSON round-trip: journey.entrance.data.email contains jeroen", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		// This is the exact JSON structure the frontend produces when the user
		// adds a journey data condition with:
		//   path: journey.entrance.data.email
		//   type: string
		//   operator: contains
		//   value: jeroen
		stepDataJSON := json.RawMessage(`{
			"rule": {
				"uuid": "00000000-0000-0000-0000-000000000001",
				"type": "wrapper",
				"group": "parent",
				"operator": "and",
				"path": "",
				"children": [
					{
						"uuid": "00000000-0000-0000-0000-000000000002",
						"root_uuid": "00000000-0000-0000-0000-000000000001",
						"parent_uuid": "00000000-0000-0000-0000-000000000001",
						"type": "string",
						"group": "journey",
						"path": "journey.entrance.data.email",
						"operator": "contains",
						"value": "jeroen"
					}
				]
			}
		}`)

		// Simulate the data map as reconstructed by GetJourneyEntryData:
		// The entrance state stored {"data": {"email": "jeroen@example.com", ...}}
		// GetJourneyEntryData aggregates by data_key, producing:
		//   {"user": {...}, "journey": {"entrance": {"data": {"email": "jeroen@example.com"}}}}
		//
		// Simulate JSON round-trip through the DB (unmarshal → marshal → unmarshal)
		dataJSON := []byte(`{
			"user": {"id": "` + userID.String() + `", "email": "jeroen@example.com"},
			"journey": {
				"entrance": {
					"data": {
						"email": "jeroen@example.com",
						"amount": 100
					}
				}
			}
		}`)
		var data map[string]any
		require.NoError(t, json.Unmarshal(dataJSON, &data))

		// Decode step data exactly like HandleGate does
		config, err := DecodeStepData[oapi.GateStepData](stepDataJSON)
		require.NoError(t, err)

		// Verify the rule was deserialized correctly
		require.Len(t, config.Rule.Children, 1, "expected 1 child rule")
		child := config.Rule.Children[0]
		assert.Equal(t, rules.RuleGroupJourney, child.Group, "child group should be 'journey'")
		assert.Equal(t, "journey.entrance.data.email", child.Path)
		assert.Equal(t, rules.OperatorContains, child.Operator)
		assert.Equal(t, "jeroen", child.Value)

		hctx := HandlerContext{
			Context:   ctx,
			DB:        db,
			ProjectID: projectID,
			UserID:    userID,
			logger:    logger,
			Data:      data,
		}

		result, err := evaluateGateRules(hctx, config.Rule, journey.JourneyUserState{})
		require.NoError(t, err)
		assert.True(t, result, "gate should match: journey.entrance.data.email contains 'jeroen'")

		// No DB queries should have been made (journey-only rule)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	// Test with RenderRuleSet in the loop (full selectGateBranch flow)
	t.Run("full HandleGate flow: journey condition selects yes path", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		stepDataJSON := json.RawMessage(`{
			"rule": {
				"uuid": "00000000-0000-0000-0000-000000000001",
				"type": "wrapper",
				"group": "parent",
				"operator": "and",
				"path": "",
				"children": [
					{
						"uuid": "00000000-0000-0000-0000-000000000002",
						"root_uuid": "00000000-0000-0000-0000-000000000001",
						"parent_uuid": "00000000-0000-0000-0000-000000000001",
						"type": "string",
						"group": "journey",
						"path": "journey.entrance.data.email",
						"operator": "contains",
						"value": "jeroen"
					}
				]
			}
		}`)

		dataJSON := []byte(`{
			"user": {"id": "` + userID.String() + `"},
			"journey": {
				"entrance": {
					"data": {
						"email": "jeroen@example.com"
					}
				}
			}
		}`)
		var data map[string]any
		require.NoError(t, json.Unmarshal(dataJSON, &data))

		step := journey.JourneyVersionStep{
			Type: GateStepType,
			Data: stepDataJSON,
			Children: []store.JourneyVersionStepChild{
				{ChildExternalID: "yes-child", Path: ptr.To("yes")},
				{ChildExternalID: "no-child", Path: ptr.To("no")},
			},
		}

		hctx := HandlerContext{
			Context:   ctx,
			DB:        db,
			ProjectID: projectID,
			UserID:    userID,
			logger:    logger,
			Data:      data,
		}

		state, children, err := HandleGate(hctx, step, journey.JourneyUserState{})
		require.NoError(t, err)
		require.NotNil(t, state.CompletedAt, "gate should be completed")
		require.Len(t, children, 1, "should have 1 child selected")
		assert.Equal(t, "yes-child", children[0].ChildExternalID)
		assert.Equal(t, "yes", *children[0].Path)

		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
