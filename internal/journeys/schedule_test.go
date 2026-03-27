package journeys

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleSchedule(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	projectID := uuid.New()
	userID := uuid.New()
	scheduleID := uuid.New()
	userScheduleID := uuid.New()

	type test struct {
		step    journey.JourneyVersionStep
		state   journey.JourneyUserState
		data    map[string]any
		wantErr bool
	}

	now := time.Now()

	tests := map[string]test{
		"simple schedule assignment": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: ScheduleStepType,
				Data: json.RawMessage(`{"schedule_name":"renewal"}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state:   journey.JourneyUserState{},
			data:    map[string]any{},
			wantErr: false,
		},
		"schedule with scheduled_at": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: ScheduleStepType,
				Data: json.RawMessage(`{"schedule_name":"renewal","scheduled_at":"2025-06-01T00:00:00Z"}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state:   journey.JourneyUserState{},
			data:    map[string]any{},
			wantErr: false,
		},
		"schedule with liquid template data": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: ScheduleStepType,
				Data: json.RawMessage(`{"schedule_name":"renewal","template":"{\"plan\":\"{{ journey.entrance.plan }}\"}"}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state: journey.JourneyUserState{},
			data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"plan": "premium",
					},
				},
			},
			wantErr: false,
		},
		"schedule with liquid schedule_name": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: ScheduleStepType,
				Data: json.RawMessage(`{"schedule_name":"{{ journey.entrance.schedule_type }}"}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state: journey.JourneyUserState{},
			data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"schedule_type": "renewal",
					},
				},
			},
			wantErr: false,
		},
		"missing schedule_name": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: ScheduleStepType,
				Data: json.RawMessage(`{"template":"{\"key\":\"value\"}"}`),
			},
			state:   journey.JourneyUserState{},
			data:    map[string]any{},
			wantErr: true,
		},
		"invalid template JSON": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: ScheduleStepType,
				Data: json.RawMessage(`{"schedule_name":"renewal","template":"not valid json"}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state:   journey.JourneyUserState{},
			data:    map[string]any{},
			wantErr: true,
		},
		"invalid scheduled_at format": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: ScheduleStepType,
				Data: json.RawMessage(`{"schedule_name":"renewal","scheduled_at":"not-a-date"}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state:   journey.JourneyUserState{},
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

			mockPub := &mockPublisher{}

			if !tc.wantErr {
				// Expect GetScheduleByName query
				scheduleRows := sqlmock.NewRows([]string{
					"id", "project_id", "name", "type", "created_at", "updated_at",
				}).AddRow(
					scheduleID, projectID, "renewal", "single", now, now,
				)
				mock.ExpectQuery(`SELECT (.+) FROM schedules`).
					WithArgs(projectID, "renewal").
					WillReturnRows(scheduleRows)

				// Expect UpsertUserSchedule INSERT
				userScheduleRows := sqlmock.NewRows([]string{
					"id", "user_id", "schedule_id", "scheduled_at", "start_at", "anchor_at",
					"interval", "occurrence", "data", "paused_at", "created_at", "updated_at",
				}).AddRow(
					userScheduleID, userID, scheduleID, nil, nil, nil,
					nil, 0, []byte("{}"), nil, now, now,
				)
				mock.ExpectQuery(`INSERT INTO user_schedules`).
					WillReturnRows(userScheduleRows)

				// Expect delete of unfired events (from UpsertUserSchedule)
				mock.ExpectExec(`DELETE FROM user_scheduled_events`).
					WillReturnResult(sqlmock.NewResult(0, 0))

				// Expect generateScheduledEvents INSERT (from UpsertUserSchedule)
				// This may not match if scheduledAt is nil (generateScheduledEvents returns early)
				// For cases with scheduled_at, expect the INSERT
				mock.ExpectExec(`INSERT INTO user_scheduled_events`).
					WillReturnResult(sqlmock.NewResult(0, 0))
			}

			hctx := HandlerContext{
				Context:   ctx,
				DB:        db,
				Publisher: mockPub,
				ProjectID: projectID,
				UserID:    userID,
				Data:      tc.data,
			}

			gotState, gotChildren, err := HandleSchedule(hctx, tc.step, tc.state)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, gotState.CompletedAt)
			require.Len(t, gotChildren, 1)
			assert.Equal(t, "next-step", gotChildren[0].ChildExternalID)

			// Verify state data contains the user schedule
			assert.NotNil(t, gotState.Data)
		})
	}
}
