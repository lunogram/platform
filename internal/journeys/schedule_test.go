package journeys

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHandleSchedule(t *testing.T) {
	t.Parallel()

	_, usersState, dbConn := setupStore(t)
	pub := pubsub.NewNoopPublisher()
	ctx := context.Background()
	projectID := uuid.New()

	userID, err := usersState.CreateUser(ctx, projectID, nil, nil, json.RawMessage(`{}`), nil, nil, []subjects.ExternalIDParam{
		{Source: "anonymous", ExternalID: "anon_" + uuid.New().String()},
	})
	require.NoError(t, err)

	type test struct {
		step      journey.JourneyVersionStep
		state     journey.JourneyUserState
		data      map[string]any
		wantErr   bool
		recurring bool // when true, assert start_at is set instead of scheduled_at
	}

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
				Data: json.RawMessage(`{"schedule_name":"appointment","scheduled_at":"2025-06-01T00:00:00Z"}`),
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
				Data: json.RawMessage(`{"schedule_name":"plan_renewal","template":"{\"plan\":\"{{ journey.entrance.plan }}\"}"}`),
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
						"schedule_type": "dynamic_renewal",
					},
				},
			},
			wantErr: false,
		},
		"recurring schedule with interval": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: ScheduleStepType,
				Data: json.RawMessage(`{"schedule_name":"monthly_check","interval":"1 month","start_at":"2025-01-01T00:00:00Z"}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state:     journey.JourneyUserState{},
			data:      map[string]any{},
			wantErr:   false,
			recurring: true,
		},
		"recurring schedule defaults start_at to now": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: ScheduleStepType,
				Data: json.RawMessage(`{"schedule_name":"weekly_digest","interval":"7 days"}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state:     journey.JourneyUserState{},
			data:      map[string]any{},
			wantErr:   false,
			recurring: true,
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
				Data: json.RawMessage(`{"schedule_name":"bad_template","template":"not valid json"}`),
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
				Data: json.RawMessage(`{"schedule_name":"bad_date","scheduled_at":"not-a-date"}`),
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
			hctx := HandlerContext{
				Context:   ctx,
				DB:        dbConn,
				Publisher: pub,
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

			// Verify state data contains the user schedule with valid times
			assert.NotNil(t, gotState.Data)

			var userSchedule subjects.UserSchedule
			require.NoError(t, json.Unmarshal(gotState.Data, &userSchedule))

			if tc.recurring {
				require.NotNil(t, userSchedule.StartAt, "recurring schedule must have start_at set")
				assert.False(t, userSchedule.StartAt.IsZero(), "start_at must not be zero time")
			} else {
				require.NotNil(t, userSchedule.ScheduledAt, "single schedule must have scheduled_at set")
				assert.False(t, userSchedule.ScheduledAt.IsZero(), "scheduled_at must not be zero time")
				assert.True(t, userSchedule.ScheduledAt.After(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)),
					"scheduled_at should be a reasonable date, got %v", userSchedule.ScheduledAt)
			}

			// Verify the schedule was actually created in the database
			scheduledStore := subjects.NewScheduledStore(dbConn, zap.NewNop())

			var scheduleName string
			switch tc.step.Type {
			default:
				var stepData struct {
					ScheduleName string `json:"schedule_name"`
				}
				require.NoError(t, json.Unmarshal(tc.step.Data, &stepData))
				scheduleName = stepData.ScheduleName
			}

			// If the schedule name is a liquid expression, resolve what it would have become
			if scheduleName == "{{ journey.entrance.schedule_type }}" {
				scheduleName = "dynamic_renewal"
			}

			schedule, err := scheduledStore.GetScheduleByName(ctx, projectID, scheduleName)
			require.NoError(t, err)
			assert.Equal(t, scheduleName, schedule.Name)
			assert.Equal(t, projectID, schedule.ProjectID)
		})
	}
}

func TestHandleScheduleMultipleEntries(t *testing.T) {
	t.Parallel()

	_, usersState, dbConn := setupStore(t)
	pub := pubsub.NewNoopPublisher()
	ctx := context.Background()
	projectID := uuid.New()

	userID, err := usersState.CreateUser(ctx, projectID, nil, nil, json.RawMessage(`{}`), nil, nil, []subjects.ExternalIDParam{
		{Source: "anonymous", ExternalID: "anon_" + uuid.New().String()},
	})
	require.NoError(t, err)

	step := journey.JourneyVersionStep{
		ID:   uuid.New(),
		Type: ScheduleStepType,
		Data: json.RawMessage(`{"schedule_name":"reentry_reminder","scheduled_at":"2030-01-01T00:00:00Z"}`),
		Children: []journey.JourneyVersionStepChild{
			{ChildExternalID: "next-step"},
		},
	}

	run := func(entryID uuid.UUID) {
		hctx := HandlerContext{
			Context:   ctx,
			DB:        dbConn,
			Publisher: pub,
			ProjectID: projectID,
			UserID:    userID,
			Data:      map[string]any{},
		}
		_, _, runErr := HandleSchedule(hctx, step, journey.JourneyUserState{JourneyEntryID: entryID})
		require.NoError(t, runErr)
	}

	scheduledStore := subjects.NewScheduledStore(dbConn, zap.NewNop())

	entry1 := uuid.New()
	run(entry1)
	// A redelivered/retried step message within the same journey entry must not
	// duplicate the assignment.
	run(entry1)

	schedule, err := scheduledStore.GetScheduleByName(ctx, projectID, "reentry_reminder")
	require.NoError(t, err)

	afterFirst, err := scheduledStore.ListUserSchedulesByScheduleID(ctx, userID, schedule.ID)
	require.NoError(t, err)
	require.Len(t, afterFirst, 1, "same journey entry must not duplicate the assignment")

	// A second journey entry schedules an additional instance with the same name.
	run(uuid.New())

	afterSecond, err := scheduledStore.ListUserSchedulesByScheduleID(ctx, userID, schedule.ID)
	require.NoError(t, err)
	require.Len(t, afterSecond, 2, "a new journey entry must create an additional assignment")
}
