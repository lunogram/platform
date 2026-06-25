package subjects

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store"
	"go.uber.org/zap"
)

func (s *ScheduledStore) ValidateInterval(ctx context.Context, interval string) bool {
	var positive bool
	err := s.db.GetContext(ctx, &positive, `SELECT $1::interval > '0 seconds'::interval`, interval)
	if err != nil {
		return false
	}

	return positive
}

// Schedule represents a schedule definition.
type Schedule struct {
	ID        uuid.UUID         `db:"id" json:"id"`
	ProjectID uuid.UUID         `db:"project_id" json:"project_id"`
	Name      string            `db:"name" json:"name"`
	Type      string            `db:"type" json:"type"` // "single" or "recurring"
	Schema    []EventSchemaPath `json:"schema,omitempty"`
	CreatedAt time.Time         `db:"created_at" json:"created_at"`
	UpdatedAt time.Time         `db:"updated_at" json:"updated_at"`
	DeletedAt *time.Time        `db:"deleted_at" json:"-"`
}

// ScheduleOffset represents an offset for a schedule.
// Offsets are first-class entities (like rules). Journeys reference them by ID.
// A default offset with offset='0 minutes', direction='after' is auto-created with each schedule.
type ScheduleOffset struct {
	ID         uuid.UUID `db:"id" json:"id"`
	ScheduleID uuid.UUID `db:"schedule_id" json:"schedule_id"`
	Offset     string    `db:"offset" json:"offset"`       // PG INTERVAL, always positive (e.g. "00:30:00", "1 mon", "1 year")
	Direction  string    `db:"direction" json:"direction"` // "before" or "after"
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`
}

// UserSchedule represents a user's assignment to a schedule.
type UserSchedule struct {
	ID               uuid.UUID       `db:"id" json:"id"`
	UserID           uuid.UUID       `db:"user_id" json:"user_id"`
	ScheduleID       uuid.UUID       `db:"schedule_id" json:"schedule_id"`
	ScheduledAt      *time.Time      `db:"scheduled_at" json:"scheduled_at,omitempty"` // fire time for single
	StartAt          *time.Time      `db:"start_at" json:"start_at,omitempty"`         // start of interval for recurring (historical, never mutated after creation)
	AnchorAt         *time.Time      `db:"anchor_at" json:"anchor_at,omitempty"`       // computation base for occurrence (scheduled_at = anchor_at + occurrence * interval)
	Interval         *string         `db:"interval" json:"interval,omitempty"`         // duration string for recurring (e.g. "1 month", "30 days")
	Occurrence       int             `db:"occurrence" json:"occurrence"`               // number of intervals advanced from anchor_at
	Data             json.RawMessage `db:"data" json:"data"`
	PausedAt         *time.Time      `db:"paused_at" json:"paused_at,omitempty"`
	HasPendingEvents bool            `db:"has_pending_events" json:"has_pending_events"`
	CreatedAt        time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time       `db:"updated_at" json:"updated_at"`
}

// ScheduledEvent represents a pre-computed event to be fired by the scheduler.
// Created when a user_schedule is created/updated. Stored in user_scheduled_events.
type ScheduledEvent struct {
	ID               uuid.UUID       `db:"id" json:"id"`
	UserScheduleID   uuid.UUID       `db:"user_schedule_id" json:"user_schedule_id"`
	ScheduleOffsetID uuid.UUID       `db:"schedule_offset_id" json:"schedule_offset_id"`
	UserID           uuid.UUID       `db:"user_id" json:"user_id"`
	ScheduleID       uuid.UUID       `db:"schedule_id" json:"schedule_id"`
	FireAt           time.Time       `db:"fire_at" json:"fire_at"`
	FiredAt          *time.Time      `db:"fired_at" json:"fired_at,omitempty"`
	Data             json.RawMessage `db:"data" json:"data"`
	CreatedAt        time.Time       `db:"created_at" json:"created_at"`
}

// DueScheduledEvent is returned by ListDueScheduledEvents and includes
// the schedule name for constructing the event name.
type DueScheduledEvent struct {
	ScheduledEvent
	ScheduleName string    `db:"schedule_name"`
	Offset       string    `db:"offset"`
	Direction    string    `db:"direction"`
	ProjectID    uuid.UUID `db:"project_id"`
}

// OrganizationSchedule represents an organization's assignment to a schedule.
type OrganizationSchedule struct {
	ID               uuid.UUID       `db:"id" json:"id"`
	OrganizationID   uuid.UUID       `db:"organization_id" json:"organization_id"`
	ScheduleID       uuid.UUID       `db:"schedule_id" json:"schedule_id"`
	ScheduledAt      *time.Time      `db:"scheduled_at" json:"scheduled_at,omitempty"`
	StartAt          *time.Time      `db:"start_at" json:"start_at,omitempty"`
	AnchorAt         *time.Time      `db:"anchor_at" json:"anchor_at,omitempty"` // computation base for occurrence (scheduled_at = anchor_at + occurrence * interval)
	Interval         *string         `db:"interval" json:"interval,omitempty"`
	Occurrence       int             `db:"occurrence" json:"occurrence"` // number of intervals advanced from anchor_at
	Data             json.RawMessage `db:"data" json:"data"`
	PausedAt         *time.Time      `db:"paused_at" json:"paused_at,omitempty"`
	HasPendingEvents bool            `db:"has_pending_events" json:"has_pending_events"`
	CreatedAt        time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt        time.Time       `db:"updated_at" json:"updated_at"`
}

// OrgScheduledEvent represents a pre-computed event for an organization schedule.
type OrgScheduledEvent struct {
	ID                     uuid.UUID       `db:"id" json:"id"`
	OrganizationScheduleID uuid.UUID       `db:"organization_schedule_id" json:"organization_schedule_id"`
	ScheduleOffsetID       uuid.UUID       `db:"schedule_offset_id" json:"schedule_offset_id"`
	OrganizationID         uuid.UUID       `db:"organization_id" json:"organization_id"`
	ScheduleID             uuid.UUID       `db:"schedule_id" json:"schedule_id"`
	FireAt                 time.Time       `db:"fire_at" json:"fire_at"`
	FiredAt                *time.Time      `db:"fired_at" json:"fired_at,omitempty"`
	Data                   json.RawMessage `db:"data" json:"data"`
	CreatedAt              time.Time       `db:"created_at" json:"created_at"`
}

// DueOrgScheduledEvent is returned by ListDueOrgScheduledEvents.
type DueOrgScheduledEvent struct {
	OrgScheduledEvent
	ScheduleName string    `db:"schedule_name"`
	Offset       string    `db:"offset"`
	Direction    string    `db:"direction"`
	ProjectID    uuid.UUID `db:"project_id"`
}

func NewScheduledStore(db store.DB, logger *zap.Logger) *ScheduledStore {
	return &ScheduledStore{db: db, logger: logger}
}

type ScheduledStore struct {
	db     store.DB
	logger *zap.Logger
}

// UpsertSchedule creates or updates a schedule definition, returning its ID.
// When creating a new schedule, a default offset with offset_minutes=0 is also created.
func (s *ScheduledStore) UpsertSchedule(ctx context.Context, projectID uuid.UUID, name string, scheduleType string) (uuid.UUID, error) {
	stmt := `
	INSERT INTO schedules (project_id, name, type)
	VALUES ($1, $2, $3)
	ON CONFLICT (project_id, name) WHERE deleted_at IS NULL
	DO UPDATE SET type = EXCLUDED.type, deleted_at = NULL
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, projectID, name, scheduleType)
	if err != nil {
		return uuid.Nil, err
	}

	// Ensure a default offset (offset='0 minutes', direction='after') exists.
	offsetStmt := `
	INSERT INTO schedule_offsets (schedule_id, "offset", direction)
	VALUES ($1, '0 minutes'::interval, 'after')
	ON CONFLICT (schedule_id, "offset", direction) DO NOTHING`

	_, err = s.db.ExecContext(ctx, offsetStmt, id)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

// DeleteSchedule soft-deletes a schedule definition by project and name.
func (s *ScheduledStore) DeleteSchedule(ctx context.Context, projectID uuid.UUID, name string) error {
	stmt := `
	UPDATE schedules
	SET deleted_at = NOW()
	WHERE project_id = $1 AND name = $2 AND deleted_at IS NULL`

	result, err := s.db.ExecContext(ctx, stmt, projectID, name)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return store.ErrNoRows
	}

	return nil
}

// DeleteScheduleByID soft-deletes a schedule definition by project and ID.
func (s *ScheduledStore) DeleteScheduleByID(ctx context.Context, projectID, scheduleID uuid.UUID) error {
	stmt := `UPDATE schedules SET deleted_at = NOW() WHERE project_id = $1 AND id = $2 AND deleted_at IS NULL`
	result, err := s.db.ExecContext(ctx, stmt, projectID, scheduleID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return store.ErrNoRows
	}

	return nil
}

// GetScheduleByName looks up a schedule definition by project and name.
func (s *ScheduledStore) GetScheduleByName(ctx context.Context, projectID uuid.UUID, name string) (*Schedule, error) {
	stmt := `
	SELECT id, project_id, name, type, created_at, updated_at
	FROM schedules
	WHERE project_id = $1 AND name = $2 AND deleted_at IS NULL`

	var schedule Schedule
	err := s.db.GetContext(ctx, &schedule, stmt, projectID, name)
	if err != nil {
		return nil, err
	}

	return &schedule, nil
}

// GetScheduleByID looks up a schedule definition by ID.
func (s *ScheduledStore) GetScheduleByID(ctx context.Context, scheduleID uuid.UUID) (*Schedule, error) {
	stmt := `
	SELECT id, project_id, name, type, created_at, updated_at
	FROM schedules
	WHERE id = $1 AND deleted_at IS NULL`

	var schedule Schedule
	err := s.db.GetContext(ctx, &schedule, stmt, scheduleID)
	if err != nil {
		return nil, err
	}

	return &schedule, nil
}

// scheduleSchemaRow is used for scanning joined schedule + schema rows.
type scheduleSchemaRow struct {
	ID        uuid.UUID      `db:"id"`
	ProjectID uuid.UUID      `db:"project_id"`
	Name      string         `db:"name"`
	Type      string         `db:"type"`
	Path      *string        `db:"path"`
	Types     pq.StringArray `db:"types"`
}

type scheduleSchemaRows []scheduleSchemaRow

func (rows scheduleSchemaRows) ToSchedules() []Schedule {
	lookup := make(map[uuid.UUID]int)
	results := make([]Schedule, 0)

	for _, row := range rows {
		index, has := lookup[row.ID]
		if !has {
			lookup[row.ID] = len(results)
			results = append(results, Schedule{
				ID:        row.ID,
				ProjectID: row.ProjectID,
				Name:      row.Name,
				Type:      row.Type,
				Schema:    []EventSchemaPath{},
			})
			index = lookup[row.ID]
		}

		if row.Path != nil {
			results[index].Schema = append(results[index].Schema, EventSchemaPath{
				Path:  *row.Path,
				Types: row.Types,
			})
		}
	}

	return results
}

// ListSchedules returns all schedule definitions for a project, joined with their schema paths.
func (s *ScheduledStore) ListSchedules(ctx context.Context, projectID uuid.UUID) ([]Schedule, error) {
	stmt := `
	SELECT
		sc.id,
		sc.project_id,
		sc.name,
		sc.type,
		ss.path,
		COALESCE(array_agg(DISTINCT ss.data_type ORDER BY ss.data_type) FILTER (WHERE ss.data_type IS NOT NULL), '{}') as types
	FROM schedules sc
	LEFT JOIN scheduled_schemas ss ON sc.id = ss.schedule_id
	WHERE sc.project_id = $1 AND sc.deleted_at IS NULL
	GROUP BY sc.id, sc.project_id, sc.name, sc.type, ss.path
	ORDER BY sc.name, ss.path`

	var rows scheduleSchemaRows
	err := s.db.SelectContext(ctx, &rows, stmt, projectID)
	if err != nil {
		return nil, err
	}

	return rows.ToSchedules(), nil
}

// CreateScheduleOffset creates a new offset for a schedule.
func (s *ScheduledStore) CreateScheduleOffset(ctx context.Context, scheduleID uuid.UUID, offset, direction string) (*ScheduleOffset, error) {
	if direction != "before" && direction != "after" {
		return nil, fmt.Errorf("invalid direction: %q (must be \"before\" or \"after\")", direction)
	}

	stmt := `
	INSERT INTO schedule_offsets (schedule_id, "offset", direction)
	VALUES ($1, $2::interval, $3)
	RETURNING id, schedule_id, "offset", direction, created_at, updated_at`

	var result ScheduleOffset
	err := s.db.GetContext(ctx, &result, stmt, scheduleID, offset, direction)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

// GetScheduleOffset returns an offset by ID.
func (s *ScheduledStore) GetScheduleOffset(ctx context.Context, offsetID uuid.UUID) (*ScheduleOffset, error) {
	stmt := `
	SELECT id, schedule_id, "offset", direction, created_at, updated_at
	FROM schedule_offsets
	WHERE id = $1`

	var offset ScheduleOffset
	err := s.db.GetContext(ctx, &offset, stmt, offsetID)
	if err != nil {
		return nil, err
	}

	return &offset, nil
}

// ListScheduleOffsets returns all offsets for a schedule.
func (s *ScheduledStore) ListScheduleOffsets(ctx context.Context, scheduleID uuid.UUID) ([]ScheduleOffset, error) {
	stmt := `
	SELECT id, schedule_id, "offset", direction, created_at, updated_at
	FROM schedule_offsets
	WHERE schedule_id = $1
	ORDER BY direction, "offset"`

	var offsets []ScheduleOffset
	err := s.db.SelectContext(ctx, &offsets, stmt, scheduleID)
	return offsets, err
}

// DeleteScheduleOffset deletes an offset. Cannot delete the default (offset='0 minutes', direction='after') offset.
func (s *ScheduledStore) DeleteScheduleOffset(ctx context.Context, offsetID uuid.UUID) error {
	stmt := `DELETE FROM schedule_offsets WHERE id = $1 AND NOT ("offset" = '0 minutes'::interval AND direction = 'after')`
	result, err := s.db.ExecContext(ctx, stmt, offsetID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return store.ErrNoRows
	}

	return nil
}

// CreateUserSchedule creates a user schedule assignment and generates user_scheduled_events
// for all offsets of the schedule.
func (s *ScheduledStore) CreateUserSchedule(ctx context.Context, userID, scheduleID uuid.UUID, scheduledAt *time.Time, startAt *time.Time, interval *string, data json.RawMessage) (*UserSchedule, error) {
	// For recurring schedules, anchor_at starts equal to start_at.
	anchorAt := startAt

	// For recurring schedules without an explicit scheduled_at, compute the
	// first occurrence from anchor_at + interval using SQL.
	var occurrence int
	if scheduledAt == nil && interval != nil && anchorAt != nil {
		result, err := s.computeNextOccurrence(ctx, *interval, *anchorAt)
		if err != nil {
			return nil, fmt.Errorf("invalid interval: %w", err)
		}
		scheduledAt = &result.NextAt
		occurrence = result.Occurrence
	}

	stmt := `
	INSERT INTO user_schedules (user_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, data)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id, user_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at`

	var us UserSchedule
	err := s.db.GetContext(ctx, &us, stmt, userID, scheduleID, scheduledAt, startAt, anchorAt, interval, occurrence, data)
	if err != nil {
		return nil, err
	}

	// Generate user_scheduled_events for all offsets
	err = s.generateScheduledEvents(ctx, us)
	if err != nil {
		return nil, err
	}

	return &us, nil
}

// UpdateUserSchedule updates a user schedule assignment. Recomputes pending user_scheduled_events.
// Nil fields are preserved from the existing row.
func (s *ScheduledStore) UpdateUserSchedule(ctx context.Context, id uuid.UUID, scheduledAt *time.Time, startAt *time.Time, interval *string, data json.RawMessage) (*UserSchedule, error) {
	existing, err := s.GetUserScheduleByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Track whether the caller explicitly provided scheduled_at.
	callerProvidedScheduledAt := scheduledAt != nil

	// Merge nil fields with existing values.
	if scheduledAt == nil {
		scheduledAt = existing.ScheduledAt
	}
	if startAt == nil {
		startAt = existing.StartAt
	}
	if interval == nil {
		interval = existing.Interval
	}
	if data == nil {
		data = existing.Data
	}

	anchorAt := existing.AnchorAt
	var occurrence int

	if callerProvidedScheduledAt {
		// When the caller explicitly sets scheduled_at (e.g. via PATCH),
		// rebase anchor_at to the new scheduled_at and reset occurrence to 0.
		// This keeps the invariant: scheduled_at = anchor_at + occurrence * interval.
		// start_at is preserved as historical data.
		anchorAt = scheduledAt
		occurrence = 0
	} else if interval != nil && anchorAt != nil {
		// For recurring schedules, recompute scheduled_at from anchor_at + interval.
		result, err := s.computeNextOccurrence(ctx, *interval, *anchorAt)
		if err != nil {
			return nil, fmt.Errorf("invalid interval: %w", err)
		}
		scheduledAt = &result.NextAt
		occurrence = result.Occurrence
	}

	stmt := `
	UPDATE user_schedules
	SET scheduled_at = $2, start_at = $3, anchor_at = $4, interval = $5, occurrence = $6, data = $7
	WHERE id = $1
	RETURNING id, user_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at`

	var us UserSchedule
	err = s.db.GetContext(ctx, &us, stmt, id, scheduledAt, startAt, anchorAt, interval, occurrence, data)
	if err != nil {
		return nil, err
	}

	// Delete unfired events and regenerate
	deleteStmt := `DELETE FROM user_scheduled_events WHERE user_schedule_id = $1 AND fired_at IS NULL`
	_, err = s.db.ExecContext(ctx, deleteStmt, id)
	if err != nil {
		return nil, err
	}

	err = s.generateScheduledEvents(ctx, us)
	if err != nil {
		return nil, err
	}

	return &us, nil
}

// UpsertUserSchedule inserts a new user schedule or updates the existing one
// for the given (userID, scheduleID) pair using ON CONFLICT. After the upsert
// it recomputes pending scheduled events.
func (s *ScheduledStore) UpsertUserSchedule(ctx context.Context, userID, scheduleID uuid.UUID, scheduledAt *time.Time, startAt *time.Time, interval *string, data json.RawMessage) (*UserSchedule, error) {
	// For recurring schedules, anchor_at starts equal to start_at.
	anchorAt := startAt

	// For recurring schedules without an explicit scheduled_at, compute the
	// first occurrence from anchor_at + interval using SQL.
	var occurrence int
	if scheduledAt == nil && interval != nil && anchorAt != nil {
		result, err := s.computeNextOccurrence(ctx, *interval, *anchorAt)
		if err != nil {
			return nil, fmt.Errorf("invalid interval: %w", err)
		}
		scheduledAt = &result.NextAt
		occurrence = result.Occurrence
	}

	stmt := `
	INSERT INTO user_schedules (user_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, data)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	ON CONFLICT (user_id, schedule_id) DO UPDATE SET
		scheduled_at = COALESCE(EXCLUDED.scheduled_at, user_schedules.scheduled_at),
		start_at     = COALESCE(EXCLUDED.start_at, user_schedules.start_at),
		anchor_at    = COALESCE(EXCLUDED.anchor_at, user_schedules.anchor_at),
		interval     = COALESCE(EXCLUDED.interval, user_schedules.interval),
		occurrence   = COALESCE(EXCLUDED.occurrence, user_schedules.occurrence),
		data         = COALESCE(EXCLUDED.data, user_schedules.data)
	RETURNING id, user_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at`

	var us UserSchedule
	err := s.db.GetContext(ctx, &us, stmt, userID, scheduleID, scheduledAt, startAt, anchorAt, interval, occurrence, data)
	if err != nil {
		return nil, err
	}

	// Delete unfired events and regenerate so they reflect the latest values.
	deleteStmt := `DELETE FROM user_scheduled_events WHERE user_schedule_id = $1 AND fired_at IS NULL`
	_, err = s.db.ExecContext(ctx, deleteStmt, us.ID)
	if err != nil {
		return nil, err
	}

	if err := s.generateScheduledEvents(ctx, us); err != nil {
		return nil, err
	}

	return &us, nil
}

// GetUserScheduleByID returns a single user schedule by its ID.
func (s *ScheduledStore) GetUserScheduleByID(ctx context.Context, id uuid.UUID) (*UserSchedule, error) {
	stmt := `
	SELECT id, user_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at
	FROM user_schedules
	WHERE id = $1`

	var us UserSchedule
	err := s.db.GetContext(ctx, &us, stmt, id)
	if err != nil {
		return nil, err
	}

	return &us, nil
}

// DeleteUserSchedule deletes a user schedule assignment.
// Cascading delete removes associated user_scheduled_events.
func (s *ScheduledStore) DeleteUserSchedule(ctx context.Context, id uuid.UUID) error {
	stmt := `DELETE FROM user_schedules WHERE id = $1`
	_, err := s.db.ExecContext(ctx, stmt, id)
	return err
}

// PauseUserSchedule sets paused_at to NOW() on a user schedule.
// If deleteEvents is true, all unfired scheduled events are deleted immediately.
// If false, existing events are left to fire but the scheduler won't advance to the next occurrence.
func (s *ScheduledStore) PauseUserSchedule(ctx context.Context, id uuid.UUID, deleteEvents bool) (*UserSchedule, error) {
	stmt := `
	UPDATE user_schedules
	SET paused_at = NOW()
	WHERE id = $1 AND paused_at IS NULL
	RETURNING id, user_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at`

	var us UserSchedule
	err := s.db.GetContext(ctx, &us, stmt, id)
	if err != nil {
		return nil, err
	}

	if deleteEvents {
		deleteStmt := `DELETE FROM user_scheduled_events WHERE user_schedule_id = $1 AND fired_at IS NULL`
		_, err = s.db.ExecContext(ctx, deleteStmt, id)
		if err != nil {
			return nil, err
		}
	}

	return &us, nil
}

// ResumeUserSchedule clears paused_at on a user schedule.
// If recalculate is true, scheduled_at is recomputed to the next future occurrence
// from NOW() (anchor_at is rebased to NOW(), occurrence reset to 0).
// If false, scheduled_at is recomputed to the next natural interval from the existing anchor_at.
// In both cases unfired events are deleted and regenerated.
func (s *ScheduledStore) ResumeUserSchedule(ctx context.Context, id uuid.UUID, recalculate bool) (*UserSchedule, error) {
	existing, err := s.GetUserScheduleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.PausedAt == nil {
		return existing, nil // already active
	}

	now := time.Now()

	if existing.Interval != nil && existing.AnchorAt != nil {
		// Both resume modes rebase anchor to now and reset occurrence.
		// The difference is whether events fire right away or at the next interval.
		//
		// Resume immediately: scheduled_at = now + 1min so the scheduler
		//   generates events that fire right away.
		// At next interval: scheduled_at = now so the scanner picks it up
		//   (scheduled_at <= NOW()) and AdvanceAndGenerate computes
		//   now + 1*interval as the next fire time.
		scheduledAt := now
		if recalculate {
			scheduledAt = now.Add(time.Minute)
		}

		stmt := `
		UPDATE user_schedules
		SET paused_at = NULL, scheduled_at = $2, anchor_at = $3, occurrence = 0
		WHERE id = $1
		RETURNING id, user_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at`

		var us UserSchedule
		err = s.db.GetContext(ctx, &us, stmt, id, scheduledAt, now)
		if err != nil {
			return nil, err
		}

		// Delete unfired events
		deleteStmt := `DELETE FROM user_scheduled_events WHERE user_schedule_id = $1 AND fired_at IS NULL`
		_, err = s.db.ExecContext(ctx, deleteStmt, id)
		if err != nil {
			return nil, err
		}

		// Only generate events immediately for "resume immediately";
		// "at next interval" lets the scheduler advance and generate.
		if recalculate {
			err = s.generateScheduledEvents(ctx, us)
			if err != nil {
				return nil, err
			}
		}

		return &us, nil
	}

	// Non-recurring schedule: just clear paused_at
	stmt := `
	UPDATE user_schedules
	SET paused_at = NULL
	WHERE id = $1
	RETURNING id, user_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at`

	var us UserSchedule
	err = s.db.GetContext(ctx, &us, stmt, id)
	if err != nil {
		return nil, err
	}

	return &us, nil
}

// DeleteUserScheduleByScheduleID deletes all user schedule assignments for a user and schedule.
func (s *ScheduledStore) DeleteUserScheduleByScheduleID(ctx context.Context, userID, scheduleID uuid.UUID) error {
	stmt := `DELETE FROM user_schedules WHERE user_id = $1 AND schedule_id = $2`
	_, err := s.db.ExecContext(ctx, stmt, userID, scheduleID)
	return err
}

// ListUserSchedules returns a paginated list of user schedules for a user within a project.
func (s *ScheduledStore) ListUserSchedules(ctx context.Context, projectID, userID uuid.UUID, params store.Pagination) ([]UserSchedule, int, error) {
	query := `
	SELECT
		us.id, us.user_id, us.schedule_id, us.scheduled_at, us.start_at, us.anchor_at, us.interval,
		us.occurrence, COALESCE(us.data, '{}'::jsonb) AS data, us.paused_at, us.created_at, us.updated_at,
		EXISTS (
			SELECT 1 FROM user_scheduled_events use
			WHERE use.user_schedule_id = us.id AND use.fired_at IS NULL AND use.fire_at <= NOW()
		) AS has_pending_events,
		COUNT(*) OVER () AS total_count
	FROM user_schedules us
	INNER JOIN schedules sc ON us.schedule_id = sc.id
	WHERE sc.project_id = $1 AND us.user_id = $2 AND sc.deleted_at IS NULL
	ORDER BY us.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		UserSchedule
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, userID, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []UserSchedule{}, 0, nil
	}

	total := results[0].TotalCount
	items := make([]UserSchedule, len(results))

	for i, r := range results {
		items[i] = r.UserSchedule
	}

	return items, total, nil
}

// ListUserSchedulesByScheduleID returns all user schedules for a user and schedule.
func (s *ScheduledStore) ListUserSchedulesByScheduleID(ctx context.Context, userID, scheduleID uuid.UUID) ([]UserSchedule, error) {
	stmt := `
	SELECT id, user_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at
	FROM user_schedules
	WHERE user_id = $1 AND schedule_id = $2`

	var results []UserSchedule
	err := s.db.SelectContext(ctx, &results, stmt, userID, scheduleID)
	return results, err
}

// ListDueScheduledEvents returns scheduled events that are due to fire
// (fire_at <= now and not yet fired), joined with schedule name and offset info.
func (s *ScheduledStore) ListDueScheduledEvents(ctx context.Context, limit int) ([]DueScheduledEvent, error) {
	stmt := `
	SELECT
		se.id, se.user_schedule_id, se.schedule_offset_id, se.user_id, se.schedule_id,
		se.fire_at, se.fired_at, COALESCE(se.data, '{}'::jsonb) AS data, se.created_at,
		sc.name AS schedule_name,
		so."offset" AS "offset",
		so.direction AS direction,
		sc.project_id
	FROM user_scheduled_events se
	JOIN schedules sc ON se.schedule_id = sc.id
	JOIN schedule_offsets so ON se.schedule_offset_id = so.id
	WHERE se.fired_at IS NULL AND se.fire_at <= NOW()
	ORDER BY se.fire_at ASC
	LIMIT $1`

	var events []DueScheduledEvent
	err := s.db.SelectContext(ctx, &events, stmt, limit)
	return events, err
}

// ScanDueScheduledEvents iterates over at most limit due user scheduled events
// and calls fn for each row. The query uses FOR UPDATE OF se SKIP LOCKED so that
// rows already locked by a concurrent transaction are silently skipped instead of
// blocking. This prevents double-processing during cluster leader transitions
// where two nodes may briefly both act as leader.
func (s *ScheduledStore) ScanDueScheduledEvents(ctx context.Context, limit int, fn func(DueScheduledEvent) error) (int, error) {
	stmt := `
	SELECT
		se.id, se.user_schedule_id, se.schedule_offset_id, se.user_id, se.schedule_id,
		se.fire_at, se.fired_at, COALESCE(se.data, '{}'::jsonb) AS data, se.created_at,
		sc.name AS schedule_name,
		so."offset" AS "offset",
		so.direction AS direction,
		sc.project_id
	FROM user_scheduled_events se
	JOIN schedules sc ON se.schedule_id = sc.id
	JOIN schedule_offsets so ON se.schedule_offset_id = so.id
	WHERE se.fired_at IS NULL AND se.fire_at <= NOW()
	ORDER BY se.fire_at ASC
	LIMIT $1
	FOR UPDATE OF se SKIP LOCKED`

	var events []DueScheduledEvent
	err := s.db.SelectContext(ctx, &events, stmt, limit)
	if err != nil {
		return 0, err
	}

	for n, event := range events {
		if err := fn(event); err != nil {
			return n, err
		}
	}

	return len(events), nil
}

// MarkScheduledEventFired marks a scheduled event as fired.
func (s *ScheduledStore) MarkScheduledEventFired(ctx context.Context, id uuid.UUID) error {
	stmt := `UPDATE user_scheduled_events SET fired_at = NOW() WHERE id = $1`
	_, err := s.db.ExecContext(ctx, stmt, id)
	return err
}

// ListPendingScheduledEventsForUser returns pending (unfired) scheduled events
// for a user and schedule.
func (s *ScheduledStore) ListPendingScheduledEventsForUser(ctx context.Context, userID, scheduleID uuid.UUID) ([]ScheduledEvent, error) {
	stmt := `
	SELECT id, user_schedule_id, schedule_offset_id, user_id, schedule_id,
		fire_at, fired_at, data, created_at
	FROM user_scheduled_events
	WHERE user_id = $1 AND schedule_id = $2 AND fired_at IS NULL
	ORDER BY fire_at ASC`

	var events []ScheduledEvent
	err := s.db.SelectContext(ctx, &events, stmt, userID, scheduleID)
	return events, err
}

// nextOccurrenceResult holds the result of computeNextOccurrence.
type nextOccurrenceResult struct {
	NextAt     time.Time `db:"next_at"`
	Occurrence int       `db:"occurrence"`
}

// computeNextOccurrence computes the next future occurrence of a recurring
// schedule using PostgreSQL interval arithmetic. It finds the smallest N >= 1
// such that anchor + N * interval > NOW(), and returns both the timestamp and
// the occurrence number N so it can be persisted to avoid drift.
func (s *ScheduledStore) computeNextOccurrence(ctx context.Context, interval string, anchor time.Time) (nextOccurrenceResult, error) {
	stmt := `
	SELECT
		$1::timestamptz + n * $2::interval AS next_at,
		n AS occurrence
	FROM generate_series(1, 10000) AS n
	WHERE $1::timestamptz + n * $2::interval > NOW()
	ORDER BY n
	LIMIT 1`

	var result nextOccurrenceResult
	err := s.db.GetContext(ctx, &result, stmt, anchor, interval)
	if err != nil {
		return nextOccurrenceResult{}, fmt.Errorf("failed to compute next occurrence for interval %q from anchor %v: %w", interval, anchor, err)
	}
	return result, nil
}

// generateScheduledEvents creates user_scheduled_events rows for all offsets
// of a user schedule using a single INSERT...SELECT with SQL interval arithmetic.
// fire_at is computed in PostgreSQL. Only future events are created.
func (s *ScheduledStore) generateScheduledEvents(ctx context.Context, us UserSchedule) error {
	if us.ScheduledAt == nil {
		return nil
	}

	stmt := `
	INSERT INTO user_scheduled_events (user_schedule_id, schedule_offset_id, user_id, schedule_id, fire_at, data)
	SELECT
		$1,
		so.id,
		$2,
		$3,
		CASE so.direction
			WHEN 'before' THEN $4::timestamptz - so."offset"
			WHEN 'after'  THEN $4::timestamptz + so."offset"
		END,
		$5
	FROM schedule_offsets so
	WHERE so.schedule_id = $3
	  AND CASE so.direction
			WHEN 'before' THEN $4::timestamptz - so."offset"
			WHEN 'after'  THEN $4::timestamptz + so."offset"
		  END > NOW()
	ON CONFLICT (user_schedule_id, schedule_offset_id, user_id) WHERE fired_at IS NULL DO NOTHING`

	_, err := s.db.ExecContext(ctx, stmt, us.ID, us.UserID, us.ScheduleID, *us.ScheduledAt, us.Data)
	return err
}

// ScanRecurringUserSchedulesWithoutPendingEvents finds recurring user_schedules whose
// scheduled_at is in the past and that have no pending (unfired) user_scheduled_events.
// For each match it advances scheduled_at by the interval until it is in the future,
// updates the row, and generates the next batch of user_scheduled_events. The query
// uses FOR UPDATE OF us SKIP LOCKED so that rows already locked by a concurrent
// transaction are silently skipped instead of blocking. This prevents
// double-processing during cluster leader transitions where two nodes may
// briefly both act as leader.
func (s *ScheduledStore) ScanRecurringUserSchedulesWithoutPendingEvents(ctx context.Context, limit int, fn func(UserSchedule) error) (int, error) {
	stmt := `
	SELECT us.id, us.user_id, us.schedule_id, us.scheduled_at, us.start_at, us.anchor_at, us.interval, us.occurrence, COALESCE(us.data, '{}'::jsonb) AS data, us.paused_at, us.created_at, us.updated_at
	FROM user_schedules us
	WHERE us.interval IS NOT NULL
	  AND us.start_at IS NOT NULL
	  AND us.paused_at IS NULL
	  AND us.scheduled_at <= NOW()
	  AND NOT EXISTS (
	    SELECT 1 FROM user_scheduled_events use
	    WHERE use.user_schedule_id = us.id AND use.fired_at IS NULL
	  )
	ORDER BY us.scheduled_at ASC
	LIMIT $1
	FOR UPDATE OF us SKIP LOCKED`

	var schedules []UserSchedule
	err := s.db.SelectContext(ctx, &schedules, stmt, limit)
	if err != nil {
		return 0, err
	}

	for n, us := range schedules {
		if err := fn(us); err != nil {
			return n, err
		}
	}

	return len(schedules), nil
}

// AdvanceAndGenerateUserScheduleEvents advances a recurring user schedule by
// incrementing the occurrence counter and computing the next scheduled_at
// as anchor_at + (occurrence * interval) entirely in SQL. This avoids drift
// by always multiplying from the anchor rather than chaining additions.
// After advancing it generates new user_scheduled_events for all offsets.
func (s *ScheduledStore) AdvanceAndGenerateUserScheduleEvents(ctx context.Context, us UserSchedule) error {
	if us.Interval == nil || us.AnchorAt == nil {
		return nil
	}

	// Atomically increment occurrence and compute scheduled_at = anchor_at + occurrence * interval.
	// The new occurrence is the smallest N > current occurrence where anchor_at + N * interval > NOW().
	advanceStmt := `
	UPDATE user_schedules
	SET occurrence = sub.n,
	    scheduled_at = sub.next_at
	FROM (
		SELECT n, $2::timestamptz + n * $3::interval AS next_at
		FROM generate_series($4::int + 1, $4::int + 10000) AS n
		WHERE $2::timestamptz + n * $3::interval > NOW()
		ORDER BY n
		LIMIT 1
	) sub
	WHERE id = $1
	RETURNING user_schedules.scheduled_at, user_schedules.occurrence`

	var nextAt time.Time
	var newOccurrence int
	err := s.db.QueryRowxContext(ctx, advanceStmt, us.ID, *us.AnchorAt, *us.Interval, us.Occurrence).Scan(&nextAt, &newOccurrence)
	if err != nil {
		return fmt.Errorf("failed to advance user_schedule %s with interval %q: %w", us.ID, *us.Interval, err)
	}

	us.ScheduledAt = &nextAt
	us.Occurrence = newOccurrence
	return s.generateScheduledEvents(ctx, us)
}

// UpsertScheduledSchema inserts schema paths for a schedule's data.
func (s *ScheduledStore) UpsertScheduledSchema(ctx context.Context, projectID, scheduleID uuid.UUID, paths rules.Paths) error {
	stmt := `
	INSERT INTO scheduled_schemas (schedule_id, path, data_type)
	VALUES ($1, $2, $3)
	ON CONFLICT (schedule_id, path, data_type) DO NOTHING`

	for _, path := range paths {
		_, err := s.db.ExecContext(ctx, stmt, scheduleID, path.Path, path.Type)
		if err != nil {
			return err
		}
	}

	return nil
}

// ListScheduledSchemas returns schema paths for a specific schedule.
func (s *ScheduledStore) ListScheduledSchemas(ctx context.Context, projectID uuid.UUID, name string) ([]EventSchemaPath, error) {
	stmt := `
	SELECT
		ss.path,
		COALESCE(array_agg(DISTINCT ss.data_type ORDER BY ss.data_type) FILTER (WHERE ss.data_type IS NOT NULL), '{}') as types
	FROM schedules sc
	JOIN scheduled_schemas ss ON sc.id = ss.schedule_id
	WHERE sc.project_id = $1 AND sc.name = $2 AND sc.deleted_at IS NULL
	GROUP BY ss.path
	ORDER BY ss.path`

	var schemas []EventSchemaPath
	err := s.db.SelectContext(ctx, &schemas, stmt, projectID, name)
	if err != nil {
		return nil, err
	}

	return schemas, nil
}

// CreateOrganizationSchedule creates an organization schedule assignment and generates
// organization_scheduled_events for all offsets of the schedule.
func (s *ScheduledStore) CreateOrganizationSchedule(ctx context.Context, organizationID, scheduleID uuid.UUID, scheduledAt *time.Time, startAt *time.Time, interval *string, data json.RawMessage) (*OrganizationSchedule, error) {
	// For recurring schedules, anchor_at starts equal to start_at.
	anchorAt := startAt

	// For recurring schedules without an explicit scheduled_at, compute the
	// first occurrence from anchor_at + interval using SQL.
	var occurrence int
	if scheduledAt == nil && interval != nil && anchorAt != nil {
		result, err := s.computeNextOccurrence(ctx, *interval, *anchorAt)
		if err != nil {
			return nil, fmt.Errorf("invalid interval: %w", err)
		}
		scheduledAt = &result.NextAt
		occurrence = result.Occurrence
	}

	stmt := `
	INSERT INTO organization_schedules (organization_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, data)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id, organization_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at`

	var os OrganizationSchedule
	err := s.db.GetContext(ctx, &os, stmt, organizationID, scheduleID, scheduledAt, startAt, anchorAt, interval, occurrence, data)
	if err != nil {
		return nil, err
	}

	// Generate organization_scheduled_events for all offsets
	err = s.generateOrgScheduledEvents(ctx, os)
	if err != nil {
		return nil, err
	}

	return &os, nil
}

// UpdateOrganizationSchedule updates an organization schedule assignment. Recomputes pending organization_scheduled_events.
// Nil fields are preserved from the existing row.
func (s *ScheduledStore) UpdateOrganizationSchedule(ctx context.Context, id uuid.UUID, scheduledAt *time.Time, startAt *time.Time, interval *string, data json.RawMessage) (*OrganizationSchedule, error) {
	existing, err := s.GetOrganizationScheduleByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Track whether the caller explicitly provided scheduled_at.
	callerProvidedScheduledAt := scheduledAt != nil

	// Merge nil fields with existing values.
	if scheduledAt == nil {
		scheduledAt = existing.ScheduledAt
	}
	if startAt == nil {
		startAt = existing.StartAt
	}
	if interval == nil {
		interval = existing.Interval
	}
	if data == nil {
		data = existing.Data
	}

	anchorAt := existing.AnchorAt
	var occurrence int

	if callerProvidedScheduledAt {
		// When the caller explicitly sets scheduled_at (e.g. via PATCH),
		// rebase anchor_at to the new scheduled_at and reset occurrence to 0.
		// This keeps the invariant: scheduled_at = anchor_at + occurrence * interval.
		// start_at is preserved as historical data.
		anchorAt = scheduledAt
		occurrence = 0
	} else if interval != nil && anchorAt != nil {
		// For recurring schedules, recompute scheduled_at from anchor_at + interval.
		result, err := s.computeNextOccurrence(ctx, *interval, *anchorAt)
		if err != nil {
			return nil, fmt.Errorf("invalid interval: %w", err)
		}
		scheduledAt = &result.NextAt
		occurrence = result.Occurrence
	}

	stmt := `
	UPDATE organization_schedules
	SET scheduled_at = $2, start_at = $3, anchor_at = $4, interval = $5, occurrence = $6, data = $7
	WHERE id = $1
	RETURNING id, organization_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at`

	var os OrganizationSchedule
	err = s.db.GetContext(ctx, &os, stmt, id, scheduledAt, startAt, anchorAt, interval, occurrence, data)
	if err != nil {
		return nil, err
	}

	// Delete unfired events and regenerate
	deleteStmt := `DELETE FROM organization_scheduled_events WHERE organization_schedule_id = $1 AND fired_at IS NULL`
	_, err = s.db.ExecContext(ctx, deleteStmt, id)
	if err != nil {
		return nil, err
	}

	err = s.generateOrgScheduledEvents(ctx, os)
	if err != nil {
		return nil, err
	}

	return &os, nil
}

// UpsertOrganizationSchedule inserts a new organization schedule or updates the
// existing one for the given (organizationID, scheduleID) pair using ON CONFLICT.
// After the upsert it recomputes pending scheduled events.
func (s *ScheduledStore) UpsertOrganizationSchedule(ctx context.Context, organizationID, scheduleID uuid.UUID, scheduledAt *time.Time, startAt *time.Time, interval *string, data json.RawMessage) (*OrganizationSchedule, error) {
	// For recurring schedules, anchor_at starts equal to start_at.
	anchorAt := startAt

	// For recurring schedules without an explicit scheduled_at, compute the
	// first occurrence from anchor_at + interval using SQL.
	var occurrence int
	if scheduledAt == nil && interval != nil && anchorAt != nil {
		result, err := s.computeNextOccurrence(ctx, *interval, *anchorAt)
		if err != nil {
			return nil, fmt.Errorf("invalid interval: %w", err)
		}
		scheduledAt = &result.NextAt
		occurrence = result.Occurrence
	}

	stmt := `
	INSERT INTO organization_schedules (organization_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, data)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	ON CONFLICT (organization_id, schedule_id) DO UPDATE SET
		scheduled_at = COALESCE(EXCLUDED.scheduled_at, organization_schedules.scheduled_at),
		start_at     = COALESCE(EXCLUDED.start_at, organization_schedules.start_at),
		anchor_at    = COALESCE(EXCLUDED.anchor_at, organization_schedules.anchor_at),
		interval     = COALESCE(EXCLUDED.interval, organization_schedules.interval),
		occurrence   = COALESCE(EXCLUDED.occurrence, organization_schedules.occurrence),
		data         = COALESCE(EXCLUDED.data, organization_schedules.data)
	RETURNING id, organization_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at`

	var os OrganizationSchedule
	err := s.db.GetContext(ctx, &os, stmt, organizationID, scheduleID, scheduledAt, startAt, anchorAt, interval, occurrence, data)
	if err != nil {
		return nil, err
	}

	// Delete unfired events and regenerate so they reflect the latest values.
	deleteStmt := `DELETE FROM organization_scheduled_events WHERE organization_schedule_id = $1 AND fired_at IS NULL`
	_, err = s.db.ExecContext(ctx, deleteStmt, os.ID)
	if err != nil {
		return nil, err
	}

	if err := s.generateOrgScheduledEvents(ctx, os); err != nil {
		return nil, err
	}

	return &os, nil
}

// GetOrganizationScheduleByID returns a single organization schedule by its ID.
func (s *ScheduledStore) GetOrganizationScheduleByID(ctx context.Context, id uuid.UUID) (*OrganizationSchedule, error) {
	stmt := `
	SELECT id, organization_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at
	FROM organization_schedules
	WHERE id = $1`

	var os OrganizationSchedule
	err := s.db.GetContext(ctx, &os, stmt, id)
	if err != nil {
		return nil, err
	}

	return &os, nil
}

// DeleteOrganizationSchedule deletes an organization schedule assignment.
// Cascading delete removes associated organization_scheduled_events.
func (s *ScheduledStore) DeleteOrganizationSchedule(ctx context.Context, id uuid.UUID) error {
	stmt := `DELETE FROM organization_schedules WHERE id = $1`
	_, err := s.db.ExecContext(ctx, stmt, id)
	return err
}

// PauseOrganizationSchedule sets paused_at to NOW() on an organization schedule.
// If deleteEvents is true, all unfired scheduled events are deleted immediately.
// If false, existing events are left to fire but the scheduler won't advance to the next occurrence.
func (s *ScheduledStore) PauseOrganizationSchedule(ctx context.Context, id uuid.UUID, deleteEvents bool) (*OrganizationSchedule, error) {
	stmt := `
	UPDATE organization_schedules
	SET paused_at = NOW()
	WHERE id = $1 AND paused_at IS NULL
	RETURNING id, organization_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at`

	var os OrganizationSchedule
	err := s.db.GetContext(ctx, &os, stmt, id)
	if err != nil {
		return nil, err
	}

	if deleteEvents {
		deleteStmt := `DELETE FROM organization_scheduled_events WHERE organization_schedule_id = $1 AND fired_at IS NULL`
		_, err = s.db.ExecContext(ctx, deleteStmt, id)
		if err != nil {
			return nil, err
		}
	}

	return &os, nil
}

// ResumeOrganizationSchedule clears paused_at on an organization schedule.
// If recalculate is true, scheduled_at is recomputed to the next future occurrence
// from NOW() (anchor_at is rebased to NOW(), occurrence reset to 0).
// If false, scheduled_at is recomputed to the next natural interval from the existing anchor_at.
// In both cases unfired events are deleted and regenerated.
func (s *ScheduledStore) ResumeOrganizationSchedule(ctx context.Context, id uuid.UUID, recalculate bool) (*OrganizationSchedule, error) {
	existing, err := s.GetOrganizationScheduleByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.PausedAt == nil {
		return existing, nil // already active
	}

	now := time.Now()

	if existing.Interval != nil && existing.AnchorAt != nil {
		// Both resume modes rebase anchor to now and reset occurrence.
		// The difference is whether events fire right away or at the next interval.
		//
		// Resume immediately: scheduled_at = now + 1min so the scheduler
		//   generates events that fire right away.
		// At next interval: scheduled_at = now so the scanner picks it up
		//   (scheduled_at <= NOW()) and AdvanceAndGenerate computes
		//   now + 1*interval as the next fire time.
		scheduledAt := now
		if recalculate {
			scheduledAt = now.Add(time.Minute)
		}

		stmt := `
		UPDATE organization_schedules
		SET paused_at = NULL, scheduled_at = $2, anchor_at = $3, occurrence = 0
		WHERE id = $1
		RETURNING id, organization_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at`

		var os OrganizationSchedule
		err = s.db.GetContext(ctx, &os, stmt, id, scheduledAt, now)
		if err != nil {
			return nil, err
		}

		// Delete unfired events
		deleteStmt := `DELETE FROM organization_scheduled_events WHERE organization_schedule_id = $1 AND fired_at IS NULL`
		_, err = s.db.ExecContext(ctx, deleteStmt, id)
		if err != nil {
			return nil, err
		}

		// Only generate events immediately for "resume immediately";
		// "at next interval" lets the scheduler advance and generate.
		if recalculate {
			err = s.generateOrgScheduledEvents(ctx, os)
			if err != nil {
				return nil, err
			}
		}

		return &os, nil
	}

	// Non-recurring schedule: just clear paused_at
	stmt := `
	UPDATE organization_schedules
	SET paused_at = NULL
	WHERE id = $1
	RETURNING id, organization_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at`

	var os OrganizationSchedule
	err = s.db.GetContext(ctx, &os, stmt, id)
	if err != nil {
		return nil, err
	}

	return &os, nil
}

// DeleteOrganizationScheduleByScheduleID deletes all organization schedule assignments for an organization and schedule.
func (s *ScheduledStore) DeleteOrganizationScheduleByScheduleID(ctx context.Context, organizationID, scheduleID uuid.UUID) error {
	stmt := `DELETE FROM organization_schedules WHERE organization_id = $1 AND schedule_id = $2`
	_, err := s.db.ExecContext(ctx, stmt, organizationID, scheduleID)
	return err
}

// ListOrganizationSchedules returns a paginated list of organization schedules for an organization within a project.
func (s *ScheduledStore) ListOrganizationSchedules(ctx context.Context, projectID, organizationID uuid.UUID, params store.Pagination) ([]OrganizationSchedule, int, error) {
	query := `
	SELECT
		os.id, os.organization_id, os.schedule_id, os.scheduled_at, os.start_at, os.anchor_at, os.interval,
		os.occurrence, COALESCE(os.data, '{}'::jsonb) AS data, os.paused_at, os.created_at, os.updated_at,
		EXISTS (
			SELECT 1 FROM organization_scheduled_events ose
			WHERE ose.organization_schedule_id = os.id AND ose.fired_at IS NULL AND ose.fire_at <= NOW()
		) AS has_pending_events,
		COUNT(*) OVER () AS total_count
	FROM organization_schedules os
	INNER JOIN schedules sc ON os.schedule_id = sc.id
	WHERE sc.project_id = $1 AND os.organization_id = $2 AND sc.deleted_at IS NULL
	ORDER BY os.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		OrganizationSchedule
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, organizationID, params.Limit, params.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []OrganizationSchedule{}, 0, nil
	}

	total := results[0].TotalCount
	items := make([]OrganizationSchedule, len(results))

	for i, r := range results {
		items[i] = r.OrganizationSchedule
	}

	return items, total, nil
}

// ListOrganizationSchedulesByScheduleID returns all organization schedules for an organization and schedule.
func (s *ScheduledStore) ListOrganizationSchedulesByScheduleID(ctx context.Context, organizationID, scheduleID uuid.UUID) ([]OrganizationSchedule, error) {
	stmt := `
	SELECT id, organization_id, schedule_id, scheduled_at, start_at, anchor_at, interval, occurrence, COALESCE(data, '{}'::jsonb) AS data, paused_at, created_at, updated_at
	FROM organization_schedules
	WHERE organization_id = $1 AND schedule_id = $2`

	var results []OrganizationSchedule
	err := s.db.SelectContext(ctx, &results, stmt, organizationID, scheduleID)
	return results, err
}

// ListDueOrgScheduledEvents returns org scheduled events that are due to fire
// (fire_at <= now and not yet fired), joined with schedule name and offset info.
func (s *ScheduledStore) ListDueOrgScheduledEvents(ctx context.Context, limit int) ([]DueOrgScheduledEvent, error) {
	stmt := `
	SELECT
		se.id, se.organization_schedule_id, se.schedule_offset_id, se.organization_id, se.schedule_id,
		se.fire_at, se.fired_at, COALESCE(se.data, '{}'::jsonb) AS data, se.created_at,
		sc.name AS schedule_name,
		so."offset" AS "offset",
		so.direction AS direction,
		sc.project_id
	FROM organization_scheduled_events se
	JOIN schedules sc ON se.schedule_id = sc.id
	JOIN schedule_offsets so ON se.schedule_offset_id = so.id
	WHERE se.fired_at IS NULL AND se.fire_at <= NOW()
	ORDER BY se.fire_at ASC
	LIMIT $1`

	var events []DueOrgScheduledEvent
	err := s.db.SelectContext(ctx, &events, stmt, limit)
	return events, err
}

// ScanDueOrgScheduledEvents iterates over at most limit due organization scheduled
// events and calls fn for each row. The query uses FOR UPDATE OF se SKIP LOCKED so
// that rows already locked by a concurrent transaction are silently skipped instead
// of blocking. This prevents double-processing during cluster leader transitions
// where two nodes may briefly both act as leader.
func (s *ScheduledStore) ScanDueOrgScheduledEvents(ctx context.Context, limit int, fn func(DueOrgScheduledEvent) error) (int, error) {
	stmt := `
	SELECT
		se.id, se.organization_schedule_id, se.schedule_offset_id, se.organization_id, se.schedule_id,
		se.fire_at, se.fired_at, COALESCE(se.data, '{}'::jsonb) AS data, se.created_at,
		sc.name AS schedule_name,
		so."offset" AS "offset",
		so.direction AS direction,
		sc.project_id
	FROM organization_scheduled_events se
	JOIN schedules sc ON se.schedule_id = sc.id
	JOIN schedule_offsets so ON se.schedule_offset_id = so.id
	WHERE se.fired_at IS NULL AND se.fire_at <= NOW()
	ORDER BY se.fire_at ASC
	LIMIT $1
	FOR UPDATE OF se SKIP LOCKED`

	var events []DueOrgScheduledEvent
	err := s.db.SelectContext(ctx, &events, stmt, limit)
	if err != nil {
		return 0, err
	}

	for n, event := range events {
		if err := fn(event); err != nil {
			return n, err
		}
	}

	return len(events), nil
}

// MarkOrgScheduledEventFired marks an org scheduled event as fired.
func (s *ScheduledStore) MarkOrgScheduledEventFired(ctx context.Context, id uuid.UUID) error {
	stmt := `UPDATE organization_scheduled_events SET fired_at = NOW() WHERE id = $1`
	_, err := s.db.ExecContext(ctx, stmt, id)
	return err
}

// ListPendingOrgScheduledEventsForOrg returns pending (unfired) org scheduled events
// for an organization and schedule.
func (s *ScheduledStore) ListPendingOrgScheduledEventsForOrg(ctx context.Context, organizationID, scheduleID uuid.UUID) ([]OrgScheduledEvent, error) {
	stmt := `
	SELECT id, organization_schedule_id, schedule_offset_id, organization_id, schedule_id,
		fire_at, fired_at, data, created_at
	FROM organization_scheduled_events
	WHERE organization_id = $1 AND schedule_id = $2 AND fired_at IS NULL
	ORDER BY fire_at ASC`

	var events []OrgScheduledEvent
	err := s.db.SelectContext(ctx, &events, stmt, organizationID, scheduleID)
	return events, err
}

// generateOrgScheduledEvents creates organization_scheduled_events rows for all offsets of an organization schedule.
// For single schedules: one event per offset at scheduled_at + offset_minutes.
// For recurring schedules: one event per offset for the next occurrence.
// generateOrgScheduledEvents creates organization_scheduled_events rows for all offsets
// of an organization schedule using a single INSERT...SELECT with SQL interval arithmetic.
// fire_at is computed in PostgreSQL. Only future events are created.
func (s *ScheduledStore) generateOrgScheduledEvents(ctx context.Context, os OrganizationSchedule) error {
	if os.ScheduledAt == nil {
		return nil
	}

	stmt := `
	INSERT INTO organization_scheduled_events (organization_schedule_id, schedule_offset_id, organization_id, schedule_id, fire_at, data)
	SELECT
		$1,
		so.id,
		$2,
		$3,
		CASE so.direction
			WHEN 'before' THEN $4::timestamptz - so."offset"
			WHEN 'after'  THEN $4::timestamptz + so."offset"
		END,
		$5
	FROM schedule_offsets so
	WHERE so.schedule_id = $3
	  AND CASE so.direction
			WHEN 'before' THEN $4::timestamptz - so."offset"
			WHEN 'after'  THEN $4::timestamptz + so."offset"
		  END > NOW()
	ON CONFLICT (organization_schedule_id, schedule_offset_id, organization_id) WHERE fired_at IS NULL DO NOTHING`

	_, err := s.db.ExecContext(ctx, stmt, os.ID, os.OrganizationID, os.ScheduleID, *os.ScheduledAt, os.Data)
	return err
}

// BackfillUserScheduledEventsForOffset creates user_scheduled_events rows for all
// user_schedules belonging to the given schedule, using a single INSERT...SELECT.
// The fire_at is computed entirely in SQL using the offset's direction and interval.
// Only future events are created. Returns the number of rows inserted.
func (s *ScheduledStore) BackfillUserScheduledEventsForOffset(ctx context.Context, scheduleID, offsetID uuid.UUID) (int64, error) {
	stmt := `
	INSERT INTO user_scheduled_events (user_schedule_id, schedule_offset_id, user_id, schedule_id, fire_at, data)
	SELECT
		us.id,
		so.id,
		us.user_id,
		us.schedule_id,
		CASE so.direction
			WHEN 'before' THEN us.scheduled_at - so."offset"
			WHEN 'after'  THEN us.scheduled_at + so."offset"
		END,
		COALESCE(us.data, '{}'::jsonb)
	FROM user_schedules us
	CROSS JOIN schedule_offsets so
	WHERE us.schedule_id = $1
	  AND so.id = $2
	  AND us.scheduled_at IS NOT NULL
	  AND CASE so.direction
			WHEN 'before' THEN us.scheduled_at - so."offset"
			WHEN 'after'  THEN us.scheduled_at + so."offset"
		  END > NOW()
	ON CONFLICT (user_schedule_id, schedule_offset_id, user_id) WHERE fired_at IS NULL DO NOTHING`

	res, err := s.db.ExecContext(ctx, stmt, scheduleID, offsetID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// BackfillOrgScheduledEventsForOffset creates organization_scheduled_events rows for all
// organization_schedules belonging to the given schedule, using a single INSERT...SELECT.
// The fire_at is computed entirely in SQL using the offset's direction and interval.
// Only future events are created. Returns the number of rows inserted.
func (s *ScheduledStore) BackfillOrgScheduledEventsForOffset(ctx context.Context, scheduleID, offsetID uuid.UUID) (int64, error) {
	stmt := `
	INSERT INTO organization_scheduled_events (organization_schedule_id, schedule_offset_id, organization_id, schedule_id, fire_at, data)
	SELECT
		os.id,
		so.id,
		os.organization_id,
		os.schedule_id,
		CASE so.direction
			WHEN 'before' THEN os.scheduled_at - so."offset"
			WHEN 'after'  THEN os.scheduled_at + so."offset"
		END,
		COALESCE(os.data, '{}'::jsonb)
	FROM organization_schedules os
	CROSS JOIN schedule_offsets so
	WHERE os.schedule_id = $1
	  AND so.id = $2
	  AND os.scheduled_at IS NOT NULL
	  AND CASE so.direction
			WHEN 'before' THEN os.scheduled_at - so."offset"
			WHEN 'after'  THEN os.scheduled_at + so."offset"
		  END > NOW()
	ON CONFLICT (organization_schedule_id, schedule_offset_id, organization_id) WHERE fired_at IS NULL DO NOTHING`

	res, err := s.db.ExecContext(ctx, stmt, scheduleID, offsetID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ScanRecurringOrgSchedulesWithoutPendingEvents finds recurring organization_schedules whose
// scheduled_at is in the past and that have no pending (unfired) organization_scheduled_events.
// For each match it advances scheduled_at by the interval until it is in the future,
// updates the row, and generates the next batch of organization_scheduled_events. The query
// uses FOR UPDATE OF os SKIP LOCKED so that rows already locked by a concurrent
// transaction are silently skipped instead of blocking. This prevents
// double-processing during cluster leader transitions where two nodes may
// briefly both act as leader.
func (s *ScheduledStore) ScanRecurringOrgSchedulesWithoutPendingEvents(ctx context.Context, limit int, fn func(OrganizationSchedule) error) (int, error) {
	stmt := `
	SELECT os.id, os.organization_id, os.schedule_id, os.scheduled_at, os.start_at, os.anchor_at, os.interval, os.occurrence, COALESCE(os.data, '{}'::jsonb) AS data, os.paused_at, os.created_at, os.updated_at
	FROM organization_schedules os
	WHERE os.interval IS NOT NULL
	  AND os.start_at IS NOT NULL
	  AND os.paused_at IS NULL
	  AND os.scheduled_at <= NOW()
	  AND NOT EXISTS (
	    SELECT 1 FROM organization_scheduled_events ose
	    WHERE ose.organization_schedule_id = os.id AND ose.fired_at IS NULL
	  )
	ORDER BY os.scheduled_at ASC
	LIMIT $1
	FOR UPDATE OF os SKIP LOCKED`

	var schedules []OrganizationSchedule
	err := s.db.SelectContext(ctx, &schedules, stmt, limit)
	if err != nil {
		return 0, err
	}

	for n, os := range schedules {
		if err := fn(os); err != nil {
			return n, err
		}
	}

	return len(schedules), nil
}

// AdvanceAndGenerateOrgScheduleEvents advances a recurring organization schedule by
// incrementing the occurrence counter and computing the next scheduled_at
// as anchor_at + (occurrence * interval) entirely in SQL. This avoids drift
// by always multiplying from the anchor rather than chaining additions.
// After advancing it generates new organization_scheduled_events for all offsets.
func (s *ScheduledStore) AdvanceAndGenerateOrgScheduleEvents(ctx context.Context, os OrganizationSchedule) error {
	if os.Interval == nil || os.AnchorAt == nil {
		return nil
	}

	// Atomically increment occurrence and compute scheduled_at = anchor_at + occurrence * interval.
	// The new occurrence is the smallest N > current occurrence where anchor_at + N * interval > NOW().
	advanceStmt := `
	UPDATE organization_schedules
	SET occurrence = sub.n,
	    scheduled_at = sub.next_at
	FROM (
		SELECT n, $2::timestamptz + n * $3::interval AS next_at
		FROM generate_series($4::int + 1, $4::int + 10000) AS n
		WHERE $2::timestamptz + n * $3::interval > NOW()
		ORDER BY n
		LIMIT 1
	) sub
	WHERE id = $1
	RETURNING organization_schedules.scheduled_at, organization_schedules.occurrence`

	var nextAt time.Time
	var newOccurrence int
	err := s.db.QueryRowxContext(ctx, advanceStmt, os.ID, *os.AnchorAt, *os.Interval, os.Occurrence).Scan(&nextAt, &newOccurrence)
	if err != nil {
		return fmt.Errorf("failed to advance organization_schedule %s with interval %q: %w", os.ID, *os.Interval, err)
	}

	os.ScheduledAt = &nextAt
	os.Occurrence = newOccurrence
	return s.generateOrgScheduledEvents(ctx, os)
}
