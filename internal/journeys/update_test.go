package journeys

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupStore(t *testing.T) (*management.State, *subjects.State, *sqlx.DB) {
	t.Helper()

	mgmt, usrs, _ := teststore.RunPostgreSQL(t)

	return management.NewState(mgmt), subjects.NewState(usrs, zap.NewNop()), usrs
}

func TestHandleUpdate(t *testing.T) {
	t.Parallel()

	mgmt, usersStore, dbConn := setupStore(t)
	pub := pubsub.NewNoopPublisher()
	ctx := context.Background()

	organizationID, err := mgmt.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	project, err := mgmt.CreateProject(ctx, management.Project{
		OrganizationID: &organizationID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	type test struct {
		step            journey.JourneyVersionStep
		state           journey.JourneyUserState
		data            map[string]any
		initialUserData map[string]any
		expectedData    map[string]any
		wantErr         bool
	}

	tests := map[string]test{
		"empty template completes without update": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: UpdateStepType,
				Data: json.RawMessage(`{"template":""}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state:           journey.JourneyUserState{},
			data:            map[string]any{},
			initialUserData: map[string]any{},
			expectedData:    nil,
			wantErr:         false,
		},
		"simple template with static values": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: UpdateStepType,
				Data: json.RawMessage(`{"template":"{\"last_journey\":\"onboarding\"}"}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state:           journey.JourneyUserState{},
			data:            map[string]any{},
			initialUserData: map[string]any{},
			expectedData: map[string]any{
				"last_journey": "onboarding",
			},
			wantErr: false,
		},
		"template with liquid variables": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: UpdateStepType,
				Data: json.RawMessage(`{"template":"{\"full_name\":\"{{ user.first_name }} {{ user.last_name }}\",\"last_product\":\"{{ journey.product.name }}\"}"}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state: journey.JourneyUserState{},
			data: map[string]any{
				"user": map[string]any{
					"first_name": "John",
					"last_name":  "Doe",
				},
				"journey": map[string]any{
					"product": map[string]any{
						"name": "Premium Plan",
					},
				},
			},
			initialUserData: map[string]any{},
			expectedData: map[string]any{
				"full_name":    "John Doe",
				"last_product": "Premium Plan",
			},
			wantErr: false,
		},
		"invalid JSON template": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: UpdateStepType,
				Data: json.RawMessage(`{"template":"not valid json"}`),
			},
			state:           journey.JourneyUserState{},
			data:            map[string]any{},
			initialUserData: map[string]any{},
			wantErr:         true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			initialData, _ := json.Marshal(tc.initialUserData)
			userID, err := usersStore.CreateUser(ctx, subjects.User{
				ProjectID:   project,
				Data:        json.RawMessage(initialData),
				AnonymousID: ptr("anon_" + uuid.New().String()),
			})
			require.NoError(t, err)

			hctx := HandlerContext{
				Context:   ctx,
				DB:        dbConn,
				ProjectID: project,
				UserID:    userID,
				Data:      tc.data,
				Publisher: pub,
			}

			gotState, gotChildren, err := HandleUpdate(hctx, tc.step, tc.state)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, gotState.CompletedAt)

			if tc.expectedData != nil {
				user, err := usersStore.GetUser(ctx, project, userID)
				require.NoError(t, err)

				var actualData map[string]any
				err = json.Unmarshal(user.Data, &actualData)
				require.NoError(t, err)

				for k, v := range tc.expectedData {
					assert.Equal(t, v, actualData[k])
				}

				require.Len(t, gotChildren, 1)
				assert.Equal(t, "next-step", gotChildren[0].ChildExternalID)
			}
		})
	}
}

func TestHandleUpdateTemplateRendering(t *testing.T) {
	t.Parallel()

	mgmt, usersStore, dbConn := setupStore(t)
	pub := pubsub.NewNoopPublisher()
	ctx := context.Background()

	organizationID, err := mgmt.CreateOrganization(ctx, "Test Org")
	require.NoError(t, err)

	project, err := mgmt.CreateProject(ctx, management.Project{
		OrganizationID: &organizationID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	type test struct {
		template     string
		data         map[string]any
		expectedData map[string]any
	}

	tests := map[string]test{
		"renders user fields": {
			template: `{"name":"{{ user.name }}","email":"{{ user.email }}"}`,
			data: map[string]any{
				"user": map[string]any{
					"name":  "Alice",
					"email": "alice@example.com",
				},
			},
			expectedData: map[string]any{
				"name":  "Alice",
				"email": "alice@example.com",
			},
		},
		"renders journey step data": {
			template: `{"product":"{{ journey.entrance.product_id }}","category":"{{ journey.entrance.category }}"}`,
			data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"product_id": "12345",
						"category":   "electronics",
					},
				},
			},
			expectedData: map[string]any{
				"product":  "12345",
				"category": "electronics",
			},
		},
		"renders nested data structures": {
			template: `{"full_name":"{{ user.first_name }} {{ user.last_name }}","company":"{{ user.company.name }}"}`,
			data: map[string]any{
				"user": map[string]any{
					"first_name": "Bob",
					"last_name":  "Smith",
					"company": map[string]any{
						"name": "Acme Corp",
					},
				},
			},
			expectedData: map[string]any{
				"full_name": "Bob Smith",
				"company":   "Acme Corp",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			userID, err := usersStore.CreateUser(ctx, subjects.User{
				ProjectID:   project,
				Data:        json.RawMessage(`{}`),
				AnonymousID: ptr("anon_" + uuid.New().String()),
			})
			require.NoError(t, err)

			stepData := map[string]any{
				"template": tc.template,
			}
			stepDataJSON, _ := json.Marshal(stepData)

			step := journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: UpdateStepType,
				Data: json.RawMessage(stepDataJSON),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next"},
				},
			}

			hctx := HandlerContext{
				Context:   ctx,
				DB:        dbConn,
				ProjectID: project,
				UserID:    userID,
				Data:      tc.data,
				Publisher: pub,
			}

			gotState, gotChildren, err := HandleUpdate(hctx, step, journey.JourneyUserState{})

			require.NoError(t, err)
			assert.NotNil(t, gotState.CompletedAt)
			require.Len(t, gotChildren, 1)

			user, err := usersStore.GetUser(ctx, project, userID)
			require.NoError(t, err)

			var actualData map[string]any
			err = json.Unmarshal(user.Data, &actualData)
			require.NoError(t, err)

			for k, v := range tc.expectedData {
				assert.Equal(t, v, actualData[k])
			}
		})
	}
}
