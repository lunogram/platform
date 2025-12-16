package statemachine

import (
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/stretchr/testify/require"
)

func TestDBScheduler_ScheduleDelay(t *testing.T) {
	t.Parallel()

	ctx := graceful.NewContext(t.Context())
	config := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
	}

	err := store.Migrate(config.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, config.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, store.Project{
		Name: "Test Project",
	})
	require.NoError(t, err)

	usersStore := store.NewUsersStore(db)
	userID, err := usersStore.CreateUser(ctx, store.User{
		ProjectID:  projectID,
		ExternalID: stringPtr("test-user"),
		Data:       []byte("{}"),
	})
	require.NoError(t, err)

	journeysStore := store.NewJourneysStore(db)
	status := "live"
	journeyID, err := journeysStore.CreateJourney(ctx, store.Journey{
		ProjectID: projectID,
		Name:      "Test Journey",
		Status:    &status,
	})
	require.NoError(t, err)

	type test struct {
		data        map[string]interface{}
		expectError bool
	}

	tests := map[string]test{
		"schedule_without_data": {
			data:        nil,
			expectError: false,
		},
		"schedule_with_data": {
			data: map[string]interface{}{
				"key1": "value1",
				"key2": 123,
			},
			expectError: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			scheduler := NewDBScheduler(db)

			entranceID, err := journeysStore.CreateUserJourneyStep(ctx, userID, journeyID, "entrance")
			require.NoError(t, err)

			stepID := uuid.New()
			delayUntil := time.Now().Add(10 * time.Minute)

			err = scheduler.ScheduleDelay(ctx, userID, journeyID, stepID, entranceID, delayUntil, tc.data)

			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				var result struct {
					ID         uuid.UUID  `db:"id"`
					UserID     uuid.UUID  `db:"user_id"`
					JourneyID  uuid.UUID  `db:"journey_id"`
					StepID     uuid.UUID  `db:"step_id"`
					EntranceID uuid.UUID  `db:"entrance_id"`
					DelayUntil *time.Time `db:"delay_until"`
					Type       string     `db:"type"`
				}

				query := `
					SELECT id, user_id, journey_id, step_id, entrance_id, delay_until, type
					FROM journey_user_step
					WHERE user_id = $1 AND journey_id = $2 AND entrance_id = $3
					ORDER BY created_at DESC
					LIMIT 1
				`

				err = db.GetContext(ctx, &result, query, userID, journeyID, entranceID)
				require.NoError(t, err)
				require.Equal(t, userID, result.UserID)
				require.Equal(t, journeyID, result.JourneyID)
				require.Equal(t, entranceID, result.EntranceID)
				require.NotNil(t, result.DelayUntil)
				require.Equal(t, "scheduled", result.Type)
				require.WithinDuration(t, delayUntil, *result.DelayUntil, time.Second)
			}
		})
	}
}

func stringPtr(s string) *string {
	return &s
}
