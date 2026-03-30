package subjects

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store"
)

// ErrUserNotFound is returned when no user matches the given identifiers.
var ErrUserNotFound = errors.New("no user found for the given identifiers")

// ErrConflictingIdentifiers is returned when identifiers in a request resolve to different existing users.
var ErrConflictingIdentifiers = errors.New("identifiers resolve to different existing users")

// ErrLastIdentifier is returned when attempting to delete the last remaining external ID for a user.
var ErrLastIdentifier = errors.New("cannot delete the last remaining identifier")

// ErrIdentifierBelongsToOther is returned when an external identifier already belongs to a different user.
var ErrIdentifierBelongsToOther = errors.New("identifier already belongs to a different user")

// ExternalIDParam represents an external identifier provided in API requests.
// It is a type alias for the oapi.ExternalIDParam type to avoid conversion overhead.
type ExternalIDParam = oapi.ExternalIDParam

// ExternalIDRecord represents a stored external identifier with full metadata.
type ExternalIDRecord struct {
	ID         uuid.UUID       `db:"id" json:"id"`
	ProjectID  uuid.UUID       `db:"project_id" json:"project_id"`
	SubjectID  uuid.UUID       `db:"subject_id" json:"subject_id"`
	Source     string          `db:"source" json:"source"`
	ExternalID string          `db:"external_id" json:"external_id"`
	Metadata   json.RawMessage `db:"metadata" json:"metadata"`
	CreatedAt  time.Time       `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time       `db:"updated_at" json:"updated_at"`
}

// ExternalIDs is a scannable slice of ExternalIDRecord.
// It implements sql.Scanner so that a jsonb_agg column can be scanned directly
// into a subject struct without a separate loading query.
type ExternalIDs []ExternalIDRecord

// Scan implements sql.Scanner for reading a JSON array from a jsonb_agg column.
func (ids *ExternalIDs) Scan(value any) error {
	if value == nil {
		*ids = []ExternalIDRecord{}
		return nil
	}

	var buf []byte
	switch v := value.(type) {
	case []byte:
		buf = v
	case string:
		buf = []byte(v)
	default:
		return fmt.Errorf("ExternalIDs.Scan: unsupported type %T", value)
	}

	var records []ExternalIDRecord
	if err := json.Unmarshal(buf, &records); err != nil {
		return fmt.Errorf("ExternalIDs.Scan: %w", err)
	}

	if records == nil {
		records = []ExternalIDRecord{}
	}
	*ids = records
	return nil
}

type Users []User

// TODO: update after oapi regeneration
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
	Email         *string         `db:"email"`
	Phone         *string         `db:"phone"`
	Data          json.RawMessage `db:"data"`
	HasPushDevice bool            `db:"has_push_device"`
	Timezone      *string         `db:"timezone"`
	Locale        *string         `db:"locale"`
	Version       int32           `db:"version"`
	CreatedAt     time.Time       `db:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at"`
	ExternalIDs   ExternalIDs     `db:"external_ids"`
}

// ExternalIDBySource returns the first external ID matching the given source, or nil.
func (u *User) ExternalIDBySource(source string) *ExternalIDRecord {
	for i := range u.ExternalIDs {
		if u.ExternalIDs[i].Source == source {
			return &u.ExternalIDs[i]
		}
	}
	return nil
}

func (u *User) OAPI() oapi.User {
	return oapi.User{
		Id:            u.ID,
		ProjectId:     u.ProjectID,
		Identifier:    u.ExternalIDs.OAPI(),
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

// OAPI converts a single ExternalIDRecord to its API response representation.
func (r *ExternalIDRecord) OAPI() oapi.ExternalIDResponse {
	var metadata *map[string]any
	if len(r.Metadata) > 0 {
		m := make(map[string]any)
		if err := json.Unmarshal(r.Metadata, &m); err == nil {
			metadata = &m
		}
	}
	return oapi.ExternalIDResponse{
		Id:         r.ID,
		ExternalId: r.ExternalID,
		Source:     r.Source,
		Metadata:   metadata,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

// OAPI converts a slice of ExternalIDRecord to their API response representations.
func (ids ExternalIDs) OAPI() []oapi.ExternalIDResponse {
	results := make([]oapi.ExternalIDResponse, len(ids))
	for i := range ids {
		results[i] = ids[i].OAPI()
	}
	return results
}

// Params converts a slice of ExternalIDRecord to ExternalIDParam, extracting
// only the Source, ExternalID, and Metadata fields.
func (ids ExternalIDs) Params() []ExternalIDParam {
	result := make([]ExternalIDParam, len(ids))
	for i, r := range ids {
		result[i] = ExternalIDParam{
			Source:     r.Source,
			ExternalID: r.ExternalID,
		}
		if len(r.Metadata) > 0 {
			m := make(map[string]any)
			if err := json.Unmarshal(r.Metadata, &m); err == nil {
				result[i].Metadata = m
			}
		}
	}
	return result
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

// LookupUserID resolves a user's internal ID from a set of external identifiers.
// If multiple identifiers resolve to different users, returns ErrConflictingIdentifiers.
func (s *UsersStore) LookupUserID(ctx context.Context, projectID uuid.UUID, identifiers []ExternalIDParam) (uuid.UUID, error) {
	if len(identifiers) == 0 {
		return uuid.Nil, fmt.Errorf("at least one identifier is required")
	}

	qa := store.NewQueryArgs()
	projectPlaceholder := qa.Add(projectID)

	conditions := make([]string, 0, len(identifiers))
	for _, ident := range identifiers {
		conditions = append(conditions, qa.Clause("(source = %s AND external_id = %s)", qa.Add(ident.Source), qa.Add(ident.ExternalID)))
	}

	query := fmt.Sprintf(`
	SELECT DISTINCT user_id
	FROM user_external_ids
	WHERE project_id = %s
	AND (%s)`, projectPlaceholder, strings.Join(conditions, " OR "))

	var userIDs []uuid.UUID
	err := s.db.SelectContext(ctx, &userIDs, query, qa.Args()...)
	if err != nil {
		return uuid.Nil, err
	}

	switch len(userIDs) {
	case 0:
		return uuid.Nil, ErrUserNotFound
	case 1:
		return userIDs[0], nil
	default:
		return uuid.Nil, ErrConflictingIdentifiers
	}
}

func (s *UsersStore) GetUser(ctx context.Context, projectID, userID uuid.UUID) (*User, error) {
	stmt := `
	SELECT u.id, u.project_id, u.email, u.phone, u.data, u.timezone, u.locale, u.version, u.created_at, u.updated_at,
		EXISTS(
			SELECT 1 FROM devices d
			WHERE d.user_id = u.id
			AND d.token IS NOT NULL
			AND d.token != ''
		) as has_push_device,
		COALESCE(ueia.external_ids, '[]'::jsonb) AS external_ids
	FROM users u
	LEFT JOIN user_external_ids_agg ueia ON ueia.user_id = u.id
	WHERE u.id = $1 AND u.project_id = $2`

	var user User
	err := s.db.GetContext(ctx, &user, stmt, userID, projectID)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetUserByExternalID retrieves a user by a specific source and external_id pair.
func (s *UsersStore) GetUserByExternalID(ctx context.Context, projectID uuid.UUID, source, externalID string) (*User, error) {
	stmt := `
	SELECT u.id, u.project_id, u.email, u.phone, u.data, u.timezone, u.locale, u.version, u.created_at, u.updated_at,
		EXISTS(
			SELECT 1 FROM devices d
			WHERE d.user_id = u.id
			AND d.token IS NOT NULL
			AND d.token != ''
		) as has_push_device,
		COALESCE(ueia.external_ids, '[]'::jsonb) AS external_ids
	FROM users u
	INNER JOIN user_external_ids uei ON u.id = uei.user_id
	LEFT JOIN user_external_ids_agg ueia ON ueia.user_id = u.id
	WHERE uei.source = $1 AND uei.external_id = $2 AND u.project_id = $3`

	var user User
	err := s.db.GetContext(ctx, &user, stmt, source, externalID, projectID)
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
	SELECT u.id, u.project_id, u.email, u.phone, u.data, u.timezone, u.locale, u.version, u.created_at, u.updated_at,
		EXISTS(
			SELECT 1 FROM devices d
			WHERE d.user_id = u.id
			AND d.token IS NOT NULL
			AND d.token != ''
		) as has_push_device,
		COALESCE(ueia.external_ids, '[]'::jsonb) AS external_ids,
		COUNT(*) OVER () AS total_count
	FROM users u
	LEFT JOIN user_external_ids_agg ueia ON ueia.user_id = u.id
	WHERE u.project_id = $1
	AND (
		$2 = '' OR
		u.email ILIKE '%' || $2 || '%' OR
		u.phone ILIKE '%' || $2 || '%' OR
		EXISTS (
			SELECT 1 FROM user_external_ids uei
			WHERE uei.user_id = u.id
			AND uei.external_id ILIKE '%' || $2 || '%'
		)
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
	for i, r := range results {
		users[i] = r.User
	}

	return users, total, nil
}

// CreateUser creates a new user and optionally attaches external identifiers.
func (s *UsersStore) CreateUser(ctx context.Context, projectID uuid.UUID, email, phone *string, data json.RawMessage, timezone, locale *string, identifiers []ExternalIDParam) (uuid.UUID, error) {
	stmt := `
	INSERT INTO users (project_id, email, phone, data, timezone, locale)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, projectID, email, phone, data, timezone, locale)
	if err != nil {
		return uuid.Nil, err
	}

	// Insert external identifiers
	for _, ident := range identifiers {
		if err := s.upsertExternalID(ctx, projectID, id, ident); err != nil {
			return uuid.Nil, err
		}
	}

	return id, nil
}

type UpsertUserParams struct {
	Identifiers []ExternalIDParam
	Email       *string
	Phone       *string
	Timezone    *string
	Locale      *string
	Data        map[string]any
}

// UpsertUser creates or updates a user based on the provided identifiers.
// If any identifier matches an existing user, that user is updated. If identifiers
// resolve to multiple different users, returns ErrConflictingIdentifiers.
func (s *UsersStore) UpsertUser(ctx context.Context, projectID uuid.UUID, params UpsertUserParams) (uuid.UUID, error) {
	data := params.Data
	if data == nil {
		data = make(map[string]any)
	}

	// Try to find an existing user by any of the identifiers
	var existingUserID uuid.UUID
	if len(params.Identifiers) > 0 {
		id, err := s.LookupUserID(ctx, projectID, params.Identifiers)
		if err != nil && !errors.Is(err, ErrUserNotFound) {
			return uuid.Nil, err
		}
		existingUserID = id
	}

	if existingUserID != uuid.Nil {
		// Update existing user
		dataJSON, err := json.Marshal(data)
		if err != nil {
			return uuid.Nil, err
		}
		rawData := json.RawMessage(dataJSON)

		stmt := `
		UPDATE users
		SET
			email = COALESCE($2, email),
			phone = COALESCE($3, phone),
			timezone = COALESCE($4, timezone),
			locale = COALESCE($5, locale),
			data = CASE
				WHEN $6::jsonb IS NOT NULL THEN data || $6::jsonb
				ELSE data
			END
		WHERE id = $1`

		_, err = s.db.ExecContext(ctx, stmt, existingUserID, params.Email, params.Phone, params.Timezone, params.Locale, rawData)
		if err != nil {
			return uuid.Nil, err
		}

		// Upsert all identifiers to this user
		for _, ident := range params.Identifiers {
			if err := s.upsertExternalID(ctx, projectID, existingUserID, ident); err != nil {
				return uuid.Nil, err
			}
		}

		return existingUserID, nil
	}

	// Create new user
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return uuid.Nil, err
	}

	stmt := `
	INSERT INTO users (project_id, email, phone, data, timezone, locale)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id`

	var id uuid.UUID
	err = s.db.GetContext(ctx, &id, stmt, projectID, params.Email, params.Phone, dataJSON, params.Timezone, params.Locale)
	if err != nil {
		return uuid.Nil, err
	}

	// Insert all identifiers
	for _, ident := range params.Identifiers {
		if err := s.upsertExternalID(ctx, projectID, id, ident); err != nil {
			return uuid.Nil, err
		}
	}

	return id, nil
}

// upsertExternalID inserts or updates an external identifier for a user.
// Metadata is shallow-merged using the PostgreSQL || operator.
// Returns ErrIdentifierBelongsToOther if the identifier already belongs to a different user.
func (s *UsersStore) upsertExternalID(ctx context.Context, projectID, userID uuid.UUID, ident ExternalIDParam) error {
	metadata := ident.Metadata
	if metadata == nil {
		metadata = make(map[string]any)
	}

	// Check if this identifier already exists for a different user.
	var existingUserID *uuid.UUID
	err := s.db.GetContext(ctx, &existingUserID,
		`SELECT user_id FROM user_external_ids
		 WHERE project_id = $1 AND source = $2 AND external_id = $3`,
		projectID, ident.Source, ident.ExternalID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if existingUserID != nil && *existingUserID != userID {
		return ErrIdentifierBelongsToOther
	}

	stmt := `
	INSERT INTO user_external_ids (project_id, user_id, source, external_id, metadata)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (project_id, source, external_id)
	DO UPDATE SET
		metadata = user_external_ids.metadata || EXCLUDED.metadata`

	_, err = s.db.ExecContext(ctx, stmt, projectID, userID, ident.Source, ident.ExternalID, metadata)
	return err
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
	Email    *string
	Phone    *string
	Timezone *string
	Locale   *string
	Data     *json.RawMessage
}

// UpdateUser updates a user's profile fields. For the data field, new values are merged
// with existing data using PostgreSQL's || operator (shallow merge). Top-level keys in
// the new data will overwrite existing keys, and new keys will be added.
func (s *UsersStore) UpdateUser(ctx context.Context, userID uuid.UUID, update UserUpdate) error {
	stmt := `
	UPDATE users
	SET
		email = COALESCE($2, email),
		phone = COALESCE($3, phone),
		timezone = COALESCE($4, timezone),
		locale = COALESCE($5, locale),
		data = CASE
			WHEN $6::jsonb IS NOT NULL THEN data || $6::jsonb
			ELSE data
		END
	WHERE id = $1`

	_, err := s.db.ExecContext(ctx, stmt, userID, update.Email, update.Phone, update.Timezone, update.Locale, update.Data)
	return err
}

// AddUserExternalID adds or updates a single external identifier for a user.
func (s *UsersStore) AddUserExternalID(ctx context.Context, projectID, userID uuid.UUID, ident ExternalIDParam) error {
	return s.upsertExternalID(ctx, projectID, userID, ident)
}

// DeleteUserExternalID removes a specific external identifier from a user.
// Returns ErrLastIdentifier if this is the user's only remaining identifier.
// The check and delete are performed atomically to prevent TOCTOU races.
func (s *UsersStore) DeleteUserExternalID(ctx context.Context, projectID, userID uuid.UUID, source, externalID string) error {
	stmt := `
	WITH guard AS (
		SELECT COUNT(*) AS cnt FROM user_external_ids WHERE user_id = $2
	)
	DELETE FROM user_external_ids
	USING guard
	WHERE guard.cnt > 1
	  AND project_id = $1 AND user_id = $2 AND source = $3 AND external_id = $4`

	result, err := s.db.ExecContext(ctx, stmt, projectID, userID, source, externalID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		// Distinguish "last identifier" from "not found": check current count.
		var count int
		if err := s.db.GetContext(ctx, &count,
			`SELECT COUNT(*) FROM user_external_ids WHERE user_id = $1`, userID); err != nil {
			return err
		}
		if count <= 1 {
			return ErrLastIdentifier
		}
		return sql.ErrNoRows
	}
	return nil
}

// DeleteUserExternalIDByID removes a specific external identifier by its primary key ID.
// Returns ErrLastIdentifier if this is the user's only remaining identifier.
// Returns sql.ErrNoRows if the identifier does not exist.
// The check and delete are performed atomically to prevent TOCTOU races.
func (s *UsersStore) DeleteUserExternalIDByID(ctx context.Context, userID, identifierID uuid.UUID) error {
	stmt := `
	WITH guard AS (
		SELECT COUNT(*) AS cnt FROM user_external_ids WHERE user_id = $2
	)
	DELETE FROM user_external_ids
	USING guard
	WHERE guard.cnt > 1
	  AND id = $1 AND user_id = $2`

	result, err := s.db.ExecContext(ctx, stmt, identifierID, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		// Distinguish "last identifier" from "not found": check current count.
		var count int
		if err := s.db.GetContext(ctx, &count,
			`SELECT COUNT(*) FROM user_external_ids WHERE user_id = $1`, userID); err != nil {
			return err
		}
		if count <= 1 {
			return ErrLastIdentifier
		}
		return sql.ErrNoRows
	}
	return nil
}

// ListUserExternalIDs returns all external identifiers for a user.
func (s *UsersStore) ListUserExternalIDs(ctx context.Context, userID uuid.UUID) ([]ExternalIDRecord, error) {
	stmt := `
	SELECT id, project_id, user_id AS subject_id, source, external_id, metadata, created_at, updated_at
	FROM user_external_ids
	WHERE user_id = $1
	ORDER BY created_at ASC`

	var records []ExternalIDRecord
	err := s.db.SelectContext(ctx, &records, stmt, userID)
	if err != nil {
		return nil, err
	}

	return records, nil
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

// ScanUsersMatchingData iterates over user IDs whose JSONB data column contains
// the given match map (PostgreSQL @> operator) and calls fn for each user ID.
// Rows are streamed from the database driver and iterated one at a time, so the
// full result set is never loaded into memory at once.
func (s *UsersStore) ScanUsersMatchingData(ctx context.Context, projectID uuid.UUID, match map[string]any, fn func(userID uuid.UUID) error) (int, error) {
	jsonb, err := json.Marshal(match)
	if err != nil {
		return 0, fmt.Errorf("marshal match filter: %w", err)
	}

	query := `SELECT id FROM users WHERE project_id = $1 AND data @> $2`
	rows, err := s.db.QueryxContext(ctx, query, projectID, jsonb)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var users int
	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return users, err
		}
		if err := fn(userID); err != nil {
			return users, err
		}
		users++
	}

	return users, rows.Err()
}
