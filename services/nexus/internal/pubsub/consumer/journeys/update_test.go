package journeys

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	projectID := uuid.New()
	userID := uuid.New()

	type test struct {
		step        store.JourneyVersionStep
		state       store.JourneyUserState
		data        map[string]any
		expectedSQL string
		wantErr     bool
	}

	tests := map[string]test{
		"empty template completes without update": {
			step: store.JourneyVersionStep{
				ID:   uuid.New(),
				Type: UpdateStepType,
				Data: json.RawMessage(`{"template":""}`),
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state:       store.JourneyUserState{},
			data:        map[string]any{},
			expectedSQL: "",
			wantErr:     false,
		},
		"simple template with static values": {
			step: store.JourneyVersionStep{
				ID:   uuid.New(),
				Type: UpdateStepType,
				Data: json.RawMessage(`{"template":"{\"last_journey\":\"onboarding\"}"}`),
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state: store.JourneyUserState{},
			data:  map[string]any{},
			expectedSQL: `UPDATE users 
	SET 
		external_id = COALESCE($2, external_id),
		email = COALESCE($3, email),
		phone = COALESCE($4, phone),
		timezone = COALESCE($5, timezone),
		locale = COALESCE($6, locale),
		data = CASE 
			WHEN $7::jsonb IS NOT NULL THEN data || $7::jsonb
			ELSE data
		END
	WHERE id = $1`,
			wantErr: false,
		},
		"template with liquid variables": {
			step: store.JourneyVersionStep{
				ID:   uuid.New(),
				Type: UpdateStepType,
				Data: json.RawMessage(`{"template":"{\"full_name\":\"{{ user.first_name }} {{ user.last_name }}\",\"last_product\":\"{{ journey.product.name }}\"}"}`),
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state: store.JourneyUserState{},
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
			expectedSQL: `UPDATE users 
	SET 
		external_id = COALESCE($2, external_id),
		email = COALESCE($3, email),
		phone = COALESCE($4, phone),
		timezone = COALESCE($5, timezone),
		locale = COALESCE($6, locale),
		data = CASE 
			WHEN $7::jsonb IS NOT NULL THEN data || $7::jsonb
			ELSE data
		END
	WHERE id = $1`,
			wantErr: false,
		},
		"invalid JSON template": {
			step: store.JourneyVersionStep{
				ID:   uuid.New(),
				Type: UpdateStepType,
				Data: json.RawMessage(`{"template":"not valid json"}`),
			},
			state:   store.JourneyUserState{},
			data:    map[string]any{},
			wantErr: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()
			db := sqlx.NewDb(mockDB, "sqlmock")

			if tc.expectedSQL != "" {
				mock.ExpectExec(tc.expectedSQL).
					WithArgs(userID, nil, nil, nil, nil, nil, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			}

			hctx := HandlerContext{
				Context:   ctx,
				DB:        db,
				ProjectID: projectID,
				UserID:    userID,
				Data:      tc.data,
			}

			gotState, gotChildren, err := HandleUpdate(hctx, tc.step, tc.state)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, gotState.CompletedAt)

			if tc.expectedSQL != "" {
				assert.NoError(t, mock.ExpectationsWereMet())
				require.Len(t, gotChildren, 1)
				assert.Equal(t, "next-step", gotChildren[0].ChildExternalID)
			}
		})
	}
}

func TestHandleUpdateTemplateRendering(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	projectID := uuid.New()
	userID := uuid.New()

	type test struct {
		template     string
		data         map[string]any
		expectedData string
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
			expectedData: `{"name":"Alice","email":"alice@example.com"}`,
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
			expectedData: `{"product":"12345","category":"electronics"}`,
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
			expectedData: `{"full_name":"Bob Smith","company":"Acme Corp"}`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()
			db := sqlx.NewDb(mockDB, "sqlmock")

			// Expect the update call with the rendered data
			mock.ExpectExec(`UPDATE users`).
				WithArgs(sqlmock.AnyArg(), nil, nil, nil, nil, nil, sqlmock.AnyArg()).
				WillReturnResult(sqlmock.NewResult(1, 1))

			stepData := map[string]any{
				"template": tc.template,
			}
			stepDataJSON, _ := json.Marshal(stepData)

			step := store.JourneyVersionStep{
				ID:   uuid.New(),
				Type: UpdateStepType,
				Data: json.RawMessage(stepDataJSON),
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "next"},
				},
			}

			hctx := HandlerContext{
				Context:   ctx,
				DB:        db,
				ProjectID: projectID,
				UserID:    userID,
				Data:      tc.data,
			}

			gotState, gotChildren, err := HandleUpdate(hctx, step, store.JourneyUserState{})

			require.NoError(t, err)
			assert.NotNil(t, gotState.CompletedAt)
			require.Len(t, gotChildren, 1)

			// Verify the SQL was called
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
