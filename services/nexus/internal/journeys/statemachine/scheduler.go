package statemachine

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// DBScheduler implements DelayScheduler using a database
type DBScheduler struct {
	db *sqlx.DB
}

// NewDBScheduler creates a new database-backed delay scheduler
func NewDBScheduler(db *sqlx.DB) *DBScheduler {
	return &DBScheduler{
		db: db,
	}
}

// ScheduleDelay creates a journey_user_step record with a delay_until timestamp
func (s *DBScheduler) ScheduleDelay(ctx context.Context, userID, journeyID, stepID, entranceID uuid.UUID, delayUntil time.Time, data map[string]interface{}) error {
	var dataJSON []byte
	var err error
	if data != nil {
		dataJSON, err = json.Marshal(data)
		if err != nil {
			return err
		}
	}

	var stepIDPtr *uuid.UUID
	if stepID != uuid.Nil {
		var exists bool
		checkStmt := `SELECT EXISTS(SELECT 1 FROM journey_steps WHERE id = $1)`
		err = s.db.GetContext(ctx, &exists, checkStmt, stepID)
		if err != nil {
			return err
		}
		if exists {
			stepIDPtr = &stepID
		}
	}

	stmt := `
		INSERT INTO journey_user_step (user_id, journey_id, step_id, entrance_id, delay_until, data, type)
		VALUES ($1, $2, $3, $4, $5, $6, 'scheduled')
	`

	_, err = s.db.ExecContext(ctx, stmt, userID, journeyID, stepIDPtr, entranceID, delayUntil, dataJSON)
	return err
}
