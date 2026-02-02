package journeys

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleLink(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	projectID := uuid.New()
	userID := uuid.New()
	targetJourneyID := uuid.New()
	versionID := uuid.New()
	entranceStepID := uuid.New()

	type test struct {
		step            store.JourneyVersionStep
		state           store.JourneyUserState
		data            map[string]any
		expectedPublish int
		wantErr         bool
		mockSetup       func(sqlmock.Sqlmock)
	}

	tests := map[string]test{
		"successfully links to target journey": {
			step: store.JourneyVersionStep{
				ID:         uuid.New(),
				Type:       LinkStepType,
				ExternalID: "link-1",
				Data:       json.RawMessage(`{"target_id":"` + targetJourneyID.String() + `"}`),
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state: store.JourneyUserState{
				ExternalStepID: "link-1",
			},
			data: map[string]any{
				"user": map[string]any{
					"name": "Test User",
				},
			},
			expectedPublish: 1,
			wantErr:         false,
			mockSetup: func(mock sqlmock.Sqlmock) {
				now := time.Now()
				versionRows := sqlmock.NewRows([]string{"id", "journey_id", "version_number", "status", "created_at", "published_at"}).
					AddRow(versionID, targetJourneyID, 1, "published", now, nil)
				mock.ExpectQuery("SELECT jv.id, jv.journey_id, jv.version_number, jv.status, jv.created_at, jv.published_at").
					WithArgs(targetJourneyID).
					WillReturnRows(versionRows)

				childrenJSON := `[{"version_id":"` + versionID.String() + `","parent_external_id":"entrance-1","child_external_id":"step-1"}]`
				stepRows := sqlmock.NewRows([]string{"id", "version_id", "external_id", "type", "name", "data", "data_key", "x", "y", "created_at", "children"}).
					AddRow(entranceStepID, versionID, "entrance-1", "entrance", nil, json.RawMessage("{}"), nil, 0, 0, time.Now(), childrenJSON)
				mock.ExpectQuery("SELECT (.+) FROM journey_version_steps").
					WithArgs(versionID).
					WillReturnRows(stepRows)

				mock.ExpectQuery("INSERT INTO journey_user_state").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
			},
		},
		"target journey with no entrance step": {
			step: store.JourneyVersionStep{
				ID:         uuid.New(),
				Type:       LinkStepType,
				ExternalID: "link-1",
				Data:       json.RawMessage(`{"target_id":"` + targetJourneyID.String() + `"}`),
			},
			state:           store.JourneyUserState{},
			data:            map[string]any{},
			expectedPublish: 0,
			wantErr:         true,
			mockSetup: func(mock sqlmock.Sqlmock) {
				now := time.Now()
				versionRows := sqlmock.NewRows([]string{"id", "journey_id", "version_number", "status", "created_at", "published_at"}).
					AddRow(versionID, targetJourneyID, 1, "published", now, nil)
				mock.ExpectQuery("SELECT jv.id, jv.journey_id, jv.version_number, jv.status, jv.created_at, jv.published_at").
					WithArgs(targetJourneyID).
					WillReturnRows(versionRows)

				stepRows := sqlmock.NewRows([]string{"id", "version_id", "external_id", "type", "name", "data", "data_key", "x", "y", "created_at", "children"}).
					AddRow(uuid.New(), versionID, "step-1", "action", nil, json.RawMessage("{}"), nil, 0, 0, time.Now(), "[]")
				mock.ExpectQuery("SELECT (.+) FROM journey_version_steps").
					WithArgs(versionID).
					WillReturnRows(stepRows)
			},
		},
		"invalid target_id format": {
			step: store.JourneyVersionStep{
				ID:         uuid.New(),
				Type:       LinkStepType,
				ExternalID: "link-1",
				Data:       json.RawMessage(`{"target_id":"invalid-uuid"}`),
			},
			state:           store.JourneyUserState{},
			data:            map[string]any{},
			expectedPublish: 0,
			wantErr:         true,
			mockSetup:       func(mock sqlmock.Sqlmock) {},
		},
		"missing target_id": {
			step: store.JourneyVersionStep{
				ID:         uuid.New(),
				Type:       LinkStepType,
				ExternalID: "link-1",
				Data:       json.RawMessage(`{}`),
			},
			state:           store.JourneyUserState{},
			data:            map[string]any{},
			expectedPublish: 0,
			wantErr:         true,
			mockSetup:       func(mock sqlmock.Sqlmock) {},
		},
		"entrance with multiple children": {
			step: store.JourneyVersionStep{
				ID:         uuid.New(),
				Type:       LinkStepType,
				ExternalID: "link-1",
				Data:       json.RawMessage(`{"target_id":"` + targetJourneyID.String() + `"}`),
				Children: []store.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state:           store.JourneyUserState{},
			data:            map[string]any{},
			expectedPublish: 3,
			wantErr:         false,
			mockSetup: func(mock sqlmock.Sqlmock) {
				now := time.Now()
				versionRows := sqlmock.NewRows([]string{"id", "journey_id", "version_number", "status", "created_at", "published_at"}).
					AddRow(versionID, targetJourneyID, 1, "published", now, nil)
				mock.ExpectQuery("SELECT jv.id, jv.journey_id, jv.version_number, jv.status, jv.created_at, jv.published_at").
					WithArgs(targetJourneyID).
					WillReturnRows(versionRows)

				childrenJSON := `[{"version_id":"` + versionID.String() + `","parent_external_id":"entrance-1","child_external_id":"step-1"},{"version_id":"` + versionID.String() + `","parent_external_id":"entrance-1","child_external_id":"step-2"},{"version_id":"` + versionID.String() + `","parent_external_id":"entrance-1","child_external_id":"step-3"}]`
				stepRows := sqlmock.NewRows([]string{"id", "version_id", "external_id", "type", "name", "data", "data_key", "x", "y", "created_at", "children"}).
					AddRow(entranceStepID, versionID, "entrance-1", "entrance", nil, json.RawMessage("{}"), nil, 0, 0, time.Now(), childrenJSON)
				mock.ExpectQuery("SELECT (.+) FROM journey_version_steps").
					WithArgs(versionID).
					WillReturnRows(stepRows)

				mock.ExpectQuery("INSERT INTO journey_user_state").
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()
			db := sqlx.NewDb(mockDB, "sqlmock")

			mockPub := &mockPublisher{}

			tc.mockSetup(mock)

			handlerCtx := HandlerContext{
				Context:   ctx,
				DB:        db,
				Publisher: mockPub,
				ProjectID: projectID,
				UserID:    userID,
				Step:      tc.step,
				Data:      tc.data,
			}

			result, children, err := HandleLink(handlerCtx, tc.step, tc.state)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, result.CompletedAt)
			assert.Equal(t, tc.step.Children, children)
			assert.Len(t, mockPub.publishedEvents, tc.expectedPublish)

			if tc.expectedPublish > 0 {
				for _, event := range mockPub.publishedEvents {
					assert.Equal(t, schemas.JourneysAdvance(projectID, targetJourneyID), event.subject)
					step, ok := event.data.(schemas.JourneyStep)
					require.True(t, ok)
					assert.Equal(t, projectID, step.ProjectID)
					assert.Equal(t, targetJourneyID, step.JourneyID)
					assert.Equal(t, userID, step.UserID)
					assert.NotEqual(t, uuid.Nil, step.JourneyEntryID)
				}
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
