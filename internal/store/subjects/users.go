package subjects

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store"
)

type Users []User

func (u Users) OAPI() []oapi.User {
	results := make([]oapi.User, len(u))
	for i, user := range u {
		results[i] = user.OAPI()
	}
	return results
}

type User struct {
	ID            uuid.UUID       `db:"id"`
	ProjectID     uuid.UUID       `db:"project_id"`
	AnonymousID   *string         `db:"anonymous_id"`
	ExternalID    *string         `db:"external_id"`
	Email         *string         `db:"email"`
	Phone         *string         `db:"phone"`
	Data          json.RawMessage `db:"data"`
	HasPushDevice bool            `db:"has_push_device"`
	Timezone      *string         `db:"timezone"`
	Locale        *string         `db:"locale"`
	Version       int32           `db:"version"`
	CreatedAt     time.Time       `db:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at"`
}

func (u *User) OAPI() oapi.User {
	anonID := ""
	if u.AnonymousID != nil {
		anonID = *u.AnonymousID
	}
	return oapi.User{
		Id:            u.ID,
		ProjectId:     u.ProjectID,
		AnonymousId:   anonID,
		ExternalId:    u.ExternalID,
		Email:         u.Email,
		Phone:         u.Phone,
		Data:          u.Data,
		Timezone:      u.Timezone,
		Locale:        u.Locale,
		HasPushDevice: u.HasPushDevice,
		Version:       u.Version,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
}

func NewUsersStore(db store.DB) *UsersStore {
	return &UsersStore{db: db}
}

type UsersStore struct {
	db store.DB
}

func (s *UsersStore) CountUsers(ctx context.Context, projectID uuid.UUID) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM users WHERE project_id = $1`, projectID)
	return count, err
}

func (s *UsersStore) LookupUserID(ctx context.Context, projectID uuid.UUID, externalID, anonymousID *string) (uuid.UUID, error) {
	// TODO: support traits lookups
	query := `
	SELECT id
	FROM users
	WHERE project_id = $1
	AND (
		($2::text IS NOT NULL AND external_id = $2::text) OR
		($3::text IS NOT NULL AND anonymous_id = $3::text)
	)
	LIMIT 1`

	var userID uuid.UUID
	err := s.db.GetContext(ctx, &userID, query, projectID, externalID, anonymousID)
	if err != nil {
		return uuid.Nil, err
	}

	return userID, nil
}

func (s *UsersStore) GetUser(ctx context.Context, projectID, userID uuid.UUID) (*User, error) {
	stmt := `
	SELECT
		u.id, u.project_id, u.anonymous_id, u.external_id, u.email, u.phone, u.data, u.timezone, u.locale, u.version, u.created_at, u.updated_at,
		EXISTS(
			SELECT 1 FROM devices d
			WHERE d.user_id = u.id
			AND (
				(d.token IS NOT NULL AND d.token != '') OR
				(d.device_credentials->>'endpoint' IS NOT NULL)
			)
		) as has_push_device
	FROM users u
	WHERE u.id = $1 AND u.project_id = $2`

	var user User
	err := s.db.GetContext(ctx, &user, stmt, userID, projectID)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *UsersStore) GetUserByExternalID(ctx context.Context, projectID uuid.UUID, externalID string) (*User, error) {
	stmt := `
	SELECT
		u.id, u.project_id, u.anonymous_id, u.external_id, u.email, u.phone, u.data, u.timezone, u.locale, u.version, u.created_at, u.updated_at,
		EXISTS(
			SELECT 1 FROM devices d
			WHERE d.user_id = u.id
			AND (
				(d.token IS NOT NULL AND d.token != '') OR
				(d.device_credentials->>'endpoint' IS NOT NULL)
			)
		) as has_push_device
	FROM users u
	WHERE u.external_id = $1 AND u.project_id = $2`

	var user User
	err := s.db.GetContext(ctx, &user, stmt, externalID, projectID)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *UsersStore) GetUserByAnonymousID(ctx context.Context, projectID uuid.UUID, anonymousID string) (*User, error) {
	stmt := `
	SELECT
		u.id, u.project_id, u.anonymous_id, u.external_id, u.email, u.phone, u.data, u.timezone, u.locale, u.version, u.created_at, u.updated_at,
		EXISTS(
			SELECT 1 FROM devices d
			WHERE d.user_id = u.id
			AND (
				(d.token IS NOT NULL AND d.token != '') OR
				(d.device_credentials->>'endpoint' IS NOT NULL)
			)
		) as has_push_device
	FROM users u
	WHERE u.anonymous_id = $1 AND u.project_id = $2`

	var user User
	err := s.db.GetContext(ctx, &user, stmt, anonymousID, projectID)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *UsersStore) InsertUserEvent(ctx context.Context, userID uuid.UUID, eventID uuid.UUID, data map[string]any) (uuid.UUID, error) {
	stmt := `
	INSERT INTO user_events ( user_id, event_id, data)
	VALUES ($1, $2, $3)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, userID, eventID, data)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *UsersStore) ListUsers(ctx context.Context, projectID uuid.UUID, pagination store.Pagination, search string) (Users, int, error) {
	query := `
	SELECT
		u.id, u.project_id, u.anonymous_id, u.external_id, u.email, u.phone, u.data, u.timezone, u.locale, u.version, u.created_at, u.updated_at,
		EXISTS(
			SELECT 1 FROM devices d
			WHERE d.user_id = u.id
			AND (
				(d.token IS NOT NULL AND d.token != '') OR
				(d.device_credentials->>'endpoint' IS NOT NULL)
			)
		) as has_push_device,
		COUNT(*) OVER () AS total_count
	FROM users u
	WHERE u.project_id = $1
	AND (
		$2 = '' OR
		u.external_id ILIKE '%' || $2 || '%' OR
		u.email ILIKE '%' || $2 || '%' OR
		u.phone ILIKE '%' || $2 || '%'
	)
	ORDER BY u.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		User
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, search, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []User{}, 0, nil
	}

	total := results[0].TotalCount
	users := make([]User, len(results))

	for index, result := range results {
		users[index] = result.User
	}

	return users, total, nil
}

func (s *UsersStore) CreateUser(ctx context.Context, user User) (uuid.UUID, error) {
	stmt := `
	INSERT INTO users (project_id, anonymous_id, external_id, email, phone, data, timezone, locale)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id
	`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		user.ProjectID,
		user.AnonymousID,
		user.ExternalID,
		user.Email,
		user.Phone,
		user.Data,
		user.Timezone,
		user.Locale,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

type UpsertUserParams struct {
	AnonymousID *string
	ExternalID  *string
	Email       *string
	Phone       *string
	Timezone    *string
	Locale      *string
	Data        map[string]any
}

func (s *UsersStore) UpsertUser(ctx context.Context, projectID uuid.UUID, params UpsertUserParams) (uuid.UUID, error) {
	data := params.Data
	if data == nil {
		data = make(map[string]any)
	}

	stmt := `
	INSERT INTO users (project_id, anonymous_id, external_id, email, phone, data, timezone, locale)
	VALUES (
		$1,
		$2,
		$3,
		$4,
		$5,
		$6,
		$7,
		$8
	)
	ON CONFLICT (project_id, external_id)
		WHERE external_id IS NOT NULL
	DO UPDATE SET
		email = COALESCE(EXCLUDED.email, users.email),
		phone = COALESCE(EXCLUDED.phone, users.phone),
		timezone = COALESCE(EXCLUDED.timezone, users.timezone),
		locale = COALESCE(EXCLUDED.locale, users.locale),
		data = COALESCE(EXCLUDED.data, users.data)
	RETURNING id
	`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		projectID,
		params.AnonymousID,
		params.ExternalID,
		params.Email,
		params.Phone,
		data,
		params.Timezone,
		params.Locale,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *UsersStore) IdentifyAndGetUser(ctx context.Context, projectID uuid.UUID, params UpsertUserParams, updateSchema bool) (*User, error) {
	userID, err := s.UpsertUser(ctx, projectID, params)
	if err != nil {
		return nil, err
	}

	if updateSchema && params.Data != nil {
		paths := rules.ParsePaths(params.Data)
		err = s.UpsertUserSchema(ctx, projectID, paths)
		if err != nil {
			return nil, err
		}
	}

	user, err := s.GetUser(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

type UserUpdate struct {
	ExternalID *string
	Email      *string
	Phone      *string
	Timezone   *string
	Locale     *string
	Data       *json.RawMessage
}

// UpdateUser updates a user's profile fields. For the data field, new values are merged
// with existing data using PostgreSQL's || operator (shallow merge). Top-level keys in
// the new data will overwrite existing keys, and new keys will be added.
func (s *UsersStore) UpdateUser(ctx context.Context, userID uuid.UUID, update UserUpdate) error {
	stmt := `
	UPDATE users
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
	WHERE id = $1`

	_, err := s.db.ExecContext(ctx, stmt, userID, update.ExternalID, update.Email, update.Phone, update.Timezone, update.Locale, update.Data)
	return err
}

func (s *UsersStore) DeleteUser(ctx context.Context, projectID, userID uuid.UUID) error {
	stmt := `DELETE FROM users WHERE id = $1 AND project_id = $2`
	_, err := s.db.ExecContext(ctx, stmt, userID, projectID)
	return err
}

type UserEvents []UserEvent

func (e UserEvents) OAPI() []oapi.UserEvent {
	results := make([]oapi.UserEvent, len(e))
	for i, event := range e {
		results[i] = event.OAPI()
	}
	return results
}

type UserEvent struct {
	ID        uuid.UUID       `db:"id"`
	ProjectID uuid.UUID       `db:"project_id"`
	UserID    uuid.UUID       `db:"user_id"`
	EventID   uuid.UUID       `db:"event_id"`
	Name      string          `db:"name"`
	Data      json.RawMessage `db:"data"`
	CreatedAt time.Time       `db:"created_at"`
}

func (e *UserEvent) OAPI() oapi.UserEvent {
	return oapi.UserEvent{
		Id:        e.ID,
		ProjectId: e.ProjectID,
		UserId:    e.UserID,
		Name:      e.Name,
		Data:      &e.Data,
		CreatedAt: e.CreatedAt,
	}
}

func (s *UsersStore) ListUserEvents(ctx context.Context, projectID, userID uuid.UUID, pagination store.Pagination, search string) (UserEvents, int, error) {
	query := `
	SELECT
		ue.id, u.project_id, ue.user_id, ue.event_id, e.name, ue.data, ue.created_at,
		COUNT(*) OVER () AS total_count
	FROM user_events ue
	INNER JOIN users u ON ue.user_id = u.id
	INNER JOIN events e ON ue.event_id = e.id
	WHERE u.project_id = $1 AND ue.user_id = $2
	AND (
		$5 = '' OR
		e.name ILIKE '%' || $5 || '%'
	)
	ORDER BY ue.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		UserEvent
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, userID, pagination.Limit, pagination.Offset, search)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []UserEvent{}, 0, nil
	}

	total := results[0].TotalCount
	events := make([]UserEvent, len(results))

	for index, result := range results {
		events[index] = result.UserEvent
	}

	return events, total, nil
}

func (s *UsersStore) CreateUserEvent(ctx context.Context, event UserEvent) (uuid.UUID, error) {
	// First, get the event_id for this event name
	eventsStore := NewEventsStore(s.db)
	eventID, err := eventsStore.UpsertEvent(ctx, event.ProjectID, event.Name, SubjectTypeUser)
	if err != nil {
		return uuid.Nil, err
	}

	stmt := `
	INSERT INTO user_events (user_id, event_id, data)
	VALUES ($1, $2, $3)
	RETURNING id`

	var id uuid.UUID
	err = s.db.GetContext(ctx, &id, stmt,
		event.UserID,
		eventID,
		event.Data,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *UsersStore) UpsertUserSchema(ctx context.Context, projectID uuid.UUID, paths rules.Paths) error {
	return s.UpsertSubjectSchema(ctx, projectID, SubjectTypeUser, paths)
}

func (s *UsersStore) UpsertSubjectSchema(ctx context.Context, projectID uuid.UUID, subjectType SubjectType, paths rules.Paths) error {
	if len(paths) == 0 {
		return nil
	}

	stmt := `
	INSERT INTO subject_schemas (project_id, path, data_type, subject_type)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (project_id, path, data_type, subject_type) DO NOTHING
	`

	// TODO: optimize with batch insert
	for _, path := range paths {
		_, err := s.db.ExecContext(ctx, stmt, projectID, path.Path, path.Type, subjectType)
		if err != nil {
			return err
		}
	}

	return nil
}

type SubjectSchema struct {
	Path  string         `db:"path"`
	Types pq.StringArray `db:"types"`
}

// UserSchema is an alias for SubjectSchema for backwards compatibility
type UserSchema = SubjectSchema

func (s *UsersStore) ListUserSchemas(ctx context.Context, projectID uuid.UUID) ([]SubjectSchema, error) {
	return s.ListSubjectSchemas(ctx, projectID, SubjectTypeUser)
}

func (s *UsersStore) ListSubjectSchemas(ctx context.Context, projectID uuid.UUID, subjectType SubjectType) ([]SubjectSchema, error) {
	stmt := `
	SELECT
		path,
		array_agg(DISTINCT data_type ORDER BY data_type) as types
	FROM subject_schemas
	WHERE project_id = $1 AND subject_type = $2
	GROUP BY path
	ORDER BY path`

	var schemas []SubjectSchema
	err := s.db.SelectContext(ctx, &schemas, stmt, projectID, subjectType)
	if err != nil {
		return nil, err
	}

	return schemas, nil
}
