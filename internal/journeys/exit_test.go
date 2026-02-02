package journeys

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/require"
)

func TestHandleExit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	projectID := uuid.New()
	journeyID := uuid.New()
	journeyEntryID := uuid.New()
	userID := uuid.New()
	stepID := uuid.New()
	externalStepID := "exit-step"
	entranceUUID := "entrance-step"

	type test struct {
		state   store.JourneyUserState
		wantErr bool
	}

	tests := map[string]test{
		"first time exit": {
			state: store.JourneyUserState{
				ID:             uuid.New(),
				ProjectID:      projectID,
				JourneyID:      journeyID,
				JourneyEntryID: journeyEntryID,
				UserID:         userID,
				ExternalStepID: externalStepID,
				CompletedAt:    nil,
			},
			wantErr: false,
		},
		"already completed": {
			state: store.JourneyUserState{
				ID:             uuid.New(),
				ProjectID:      projectID,
				JourneyID:      journeyID,
				JourneyEntryID: journeyEntryID,
				UserID:         userID,
				ExternalStepID: externalStepID,
				CompletedAt:    func() *time.Time { t := time.Now(); return &t }(),
			},
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()

			sqlxDB := sqlx.NewDb(db, "sqlmock")

			// Setup expectations only if state is not already completed
			if tc.state.CompletedAt == nil {
				mock.ExpectExec("UPDATE journey_user_state").
					WithArgs(sqlmock.AnyArg(), entranceUUID, journeyID).
					WillReturnResult(sqlmock.NewResult(0, 1))
			}

			step := store.JourneyVersionStep{
				ID:         stepID,
				Type:       "exit",
				ExternalID: externalStepID,
				Data:       []byte(`{"entrance_uuid":"` + entranceUUID + `"}`),
			}

			handlerCtx := HandlerContext{
				Context:   ctx,
				DB:        sqlxDB,
				Publisher: nil, // Exit doesn't need publisher
				ProjectID: projectID,
				UserID:    userID,
				Step:      step,
				Data:      nil,
			}

			result, children, err := HandleExit(handlerCtx, step, tc.state)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Nil(t, children)

				// Verify completed_at is set
				require.NotNil(t, result.CompletedAt)
			}

			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
