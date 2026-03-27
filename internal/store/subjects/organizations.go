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
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/rules/query"
	"github.com/lunogram/platform/internal/store"
)

// ErrOrgNotFound is returned when no organization matches the given identifiers.
var ErrOrgNotFound = errors.New("no organization found for the given identifiers")

// ErrOrgConflictingIdentifiers is returned when identifiers in a request resolve to different existing organizations.
var ErrOrgConflictingIdentifiers = errors.New("identifiers resolve to different existing organizations")

// ErrOrgLastIdentifier is returned when attempting to delete the last remaining external ID for an organization.
var ErrOrgLastIdentifier = errors.New("cannot delete the last remaining identifier for organization")

// ErrOrgIdentifierBelongsToOther is returned when an external identifier already belongs to a different organization.
var ErrOrgIdentifierBelongsToOther = errors.New("identifier already belongs to a different organization")

type Organizations []Organization

// TODO: update after oapi regeneration
func (o Organizations) OAPI() []oapi.Organization {
	results := make([]oapi.Organization, len(o))
	for i, org := range o {
		results[i] = org.OAPI()
	}
	return results
}

type Organization struct {
	ID          uuid.UUID       `db:"id"`
	ProjectID   uuid.UUID       `db:"project_id"`
	Name        *string         `db:"name"`
	Data        json.RawMessage `db:"data"`
	Version     int32           `db:"version"`
	CreatedAt   time.Time       `db:"created_at"`
	UpdatedAt   time.Time       `db:"updated_at"`
	ExternalIDs ExternalIDs     `db:"external_ids"`
}

// ExternalIDBySource returns the first external ID matching the given source, or nil.
func (o *Organization) ExternalIDBySource(source string) *ExternalIDRecord {
	for i := range o.ExternalIDs {
		if o.ExternalIDs[i].Source == source {
			return &o.ExternalIDs[i]
		}
	}
	return nil
}

func (o *Organization) OAPI() oapi.Organization {
	return oapi.Organization{
		Id:         o.ID,
		ProjectId:  o.ProjectID,
		Identifier: o.ExternalIDs.OAPI(),
		Name:       o.Name,
		Data:       o.Data,
		Version:    o.Version,
		CreatedAt:  o.CreatedAt,
		UpdatedAt:  o.UpdatedAt,
	}
}

func NewOrganizationsStore(db store.DB) *OrganizationsStore {
	return &OrganizationsStore{db: db}
}

type OrganizationsStore struct {
	db store.DB
}

// GetOrganization retrieves an organization by its internal ID.
func (s *OrganizationsStore) GetOrganization(ctx context.Context, projectID, orgID uuid.UUID) (*Organization, error) {
	stmt := `
	SELECT o.id, o.project_id, o.name, o.data, o.version, o.created_at, o.updated_at,
		COALESCE(oeia.external_ids, '[]'::jsonb) AS external_ids
	FROM organizations o
	LEFT JOIN organization_external_ids_agg oeia ON oeia.organization_id = o.id
	WHERE o.id = $1 AND o.project_id = $2`

	var org Organization
	err := s.db.GetContext(ctx, &org, stmt, orgID, projectID)
	if err != nil {
		return nil, err
	}

	return &org, nil
}

// GetOrganizationByExternalID retrieves an organization by a specific source and external_id pair.
func (s *OrganizationsStore) GetOrganizationByExternalID(ctx context.Context, projectID uuid.UUID, source, externalID string) (*Organization, error) {
	stmt := `
	SELECT o.id, o.project_id, o.name, o.data, o.version, o.created_at, o.updated_at,
		COALESCE(oeia.external_ids, '[]'::jsonb) AS external_ids
	FROM organizations o
	INNER JOIN organization_external_ids oei ON o.id = oei.organization_id
	LEFT JOIN organization_external_ids_agg oeia ON oeia.organization_id = o.id
	WHERE oei.source = $1 AND oei.external_id = $2 AND o.project_id = $3`

	var org Organization
	err := s.db.GetContext(ctx, &org, stmt, source, externalID, projectID)
	if err != nil {
		return nil, err
	}

	return &org, nil
}

// LookupOrganizationID resolves an organization's internal ID from a set of external identifiers.
// If multiple identifiers resolve to different organizations, returns ErrOrgConflictingIdentifiers.
func (s *OrganizationsStore) LookupOrganizationID(ctx context.Context, projectID uuid.UUID, identifiers []ExternalIDParam) (uuid.UUID, error) {
	if len(identifiers) == 0 {
		return uuid.Nil, fmt.Errorf("at least one identifier is required")
	}

	arguments := store.NewQueryArgs()

	conditions := make([]string, 0, len(identifiers))
	for _, ident := range identifiers {
		conditions = append(conditions, arguments.Clause("(source = %s AND external_id = %s)", arguments.Add(ident.Source), arguments.Add(ident.ExternalID)))
	}

	q := fmt.Sprintf(`
	SELECT DISTINCT organization_id
	FROM organization_external_ids
	WHERE project_id = %s
	AND (%s)`, arguments.Add(projectID), strings.Join(conditions, " OR "))

	var organizations []uuid.UUID
	err := s.db.SelectContext(ctx, &organizations, q, arguments.Args()...)
	if err != nil {
		return uuid.Nil, err
	}

	switch len(organizations) {
	case 0:
		return uuid.Nil, ErrOrgNotFound
	case 1:
		return organizations[0], nil
	default:
		return uuid.Nil, ErrOrgConflictingIdentifiers
	}
}

type UpsertOrganizationParams struct {
	Identifiers []ExternalIDParam
	Name        *string
	Data        map[string]any
}

// UpsertOrganization creates or updates an organization based on the provided identifiers.
func (s *OrganizationsStore) UpsertOrganization(ctx context.Context, projectID uuid.UUID, params UpsertOrganizationParams) (uuid.UUID, error) {
	data := params.Data
	if data == nil {
		data = make(map[string]any)
	}

	// Try to find an existing organization by any of the identifiers
	var existingOrgID uuid.UUID
	if len(params.Identifiers) > 0 {
		id, err := s.LookupOrganizationID(ctx, projectID, params.Identifiers)
		if err != nil && !errors.Is(err, ErrOrgNotFound) {
			return uuid.Nil, err
		}
		existingOrgID = id
	}

	if existingOrgID != uuid.Nil {
		// Update existing organization
		dataJSON, err := json.Marshal(data)
		if err != nil {
			return uuid.Nil, err
		}
		rawData := json.RawMessage(dataJSON)

		stmt := `
		UPDATE organizations
		SET
			name = COALESCE($2, name),
			data = CASE
				WHEN $3::jsonb IS NOT NULL THEN data || $3::jsonb
				ELSE data
			END
		WHERE id = $1`

		_, err = s.db.ExecContext(ctx, stmt, existingOrgID, params.Name, rawData)
		if err != nil {
			return uuid.Nil, err
		}

		// Upsert all identifiers to this organization
		for _, ident := range params.Identifiers {
			if err := s.upsertOrgExternalID(ctx, projectID, existingOrgID, ident); err != nil {
				return uuid.Nil, err
			}
		}

		return existingOrgID, nil
	}

	// Create new organization
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return uuid.Nil, err
	}

	stmt := `
	INSERT INTO organizations (project_id, name, data)
	VALUES ($1, $2, $3)
	RETURNING id`

	var id uuid.UUID
	err = s.db.GetContext(ctx, &id, stmt, projectID, params.Name, dataJSON)
	if err != nil {
		return uuid.Nil, err
	}

	// Insert all identifiers
	for _, ident := range params.Identifiers {
		if err := s.upsertOrgExternalID(ctx, projectID, id, ident); err != nil {
			return uuid.Nil, err
		}
	}

	return id, nil
}

// upsertOrgExternalID inserts or updates an external identifier for an organization.
// Returns ErrOrgIdentifierBelongsToOther if the identifier already belongs to a different organization.
func (s *OrganizationsStore) upsertOrgExternalID(ctx context.Context, projectID, orgID uuid.UUID, ident ExternalIDParam) error {
	metadata := ident.Metadata
	if metadata == nil {
		metadata = make(map[string]any)
	}

	// Check if this identifier already exists for a different organization.
	var existingOrgID *uuid.UUID
	err := s.db.GetContext(ctx, &existingOrgID,
		`SELECT organization_id FROM organization_external_ids
		 WHERE project_id = $1 AND source = $2 AND external_id = $3`,
		projectID, ident.Source, ident.ExternalID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if existingOrgID != nil && *existingOrgID != orgID {
		return ErrOrgIdentifierBelongsToOther
	}

	stmt := `
	INSERT INTO organization_external_ids (project_id, organization_id, source, external_id, metadata)
	VALUES ($1, $2, $3, $4, $5)
	ON CONFLICT (project_id, source, external_id)
	DO UPDATE SET
		metadata = organization_external_ids.metadata || EXCLUDED.metadata`

	_, err = s.db.ExecContext(ctx, stmt, projectID, orgID, ident.Source, ident.ExternalID, metadata)
	return err
}

type OrganizationUpdate struct {
	Name *string
	Data *json.RawMessage
}

// UpdateOrganization updates an organization's fields. For the data field, new values are merged
// with existing data using PostgreSQL's || operator (shallow merge).
func (s *OrganizationsStore) UpdateOrganization(ctx context.Context, projectID, orgID uuid.UUID, update OrganizationUpdate) error {
	stmt := `
	UPDATE organizations
	SET
		name = COALESCE($3, name),
		data = CASE
			WHEN $4::jsonb IS NOT NULL THEN data || $4::jsonb
			ELSE data
		END
	WHERE id = $1 AND project_id = $2`

	_, err := s.db.ExecContext(ctx, stmt, orgID, projectID, update.Name, update.Data)
	return err
}

// AddOrgExternalID adds or updates a single external identifier for an organization.
func (s *OrganizationsStore) AddOrgExternalID(ctx context.Context, projectID, orgID uuid.UUID, ident ExternalIDParam) error {
	return s.upsertOrgExternalID(ctx, projectID, orgID, ident)
}

// DeleteOrgExternalID removes a specific external identifier from an organization.
// Returns ErrOrgLastIdentifier if this is the organization's only remaining identifier.
// The check and delete are performed atomically to prevent TOCTOU races.
func (s *OrganizationsStore) DeleteOrgExternalID(ctx context.Context, projectID, orgID uuid.UUID, source, externalID string) error {
	stmt := `
	WITH guard AS (
		SELECT COUNT(*) AS cnt FROM organization_external_ids WHERE organization_id = $2
	)
	DELETE FROM organization_external_ids
	USING guard
	WHERE guard.cnt > 1
	  AND project_id = $1 AND organization_id = $2 AND source = $3 AND external_id = $4`

	result, err := s.db.ExecContext(ctx, stmt, projectID, orgID, source, externalID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		var count int
		if err := s.db.GetContext(ctx, &count,
			`SELECT COUNT(*) FROM organization_external_ids WHERE organization_id = $1`, orgID); err != nil {
			return err
		}
		if count <= 1 {
			return ErrOrgLastIdentifier
		}
		return sql.ErrNoRows
	}
	return nil
}

// DeleteOrgExternalIDByID removes a specific external identifier by its primary key ID.
// Returns ErrOrgLastIdentifier if this is the organization's only remaining identifier.
// Returns sql.ErrNoRows if the identifier does not exist.
// The check and delete are performed atomically to prevent TOCTOU races.
func (s *OrganizationsStore) DeleteOrgExternalIDByID(ctx context.Context, orgID, identifierID uuid.UUID) error {
	stmt := `
	WITH guard AS (
		SELECT COUNT(*) AS cnt FROM organization_external_ids WHERE organization_id = $1
	)
	DELETE FROM organization_external_ids
	USING guard
	WHERE guard.cnt > 1
	  AND id = $2 AND organization_id = $1`

	result, err := s.db.ExecContext(ctx, stmt, orgID, identifierID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		var count int
		if err := s.db.GetContext(ctx, &count,
			`SELECT COUNT(*) FROM organization_external_ids WHERE organization_id = $1`, orgID); err != nil {
			return err
		}
		if count <= 1 {
			return ErrOrgLastIdentifier
		}
		return sql.ErrNoRows
	}
	return nil
}

// DeleteOrganization deletes an organization and all its user memberships (via CASCADE).
func (s *OrganizationsStore) DeleteOrganization(ctx context.Context, projectID, orgID uuid.UUID) error {
	stmt := `DELETE FROM organizations WHERE id = $1 AND project_id = $2`
	_, err := s.db.ExecContext(ctx, stmt, orgID, projectID)
	return err
}

// ListOrganizations lists all organizations for a project with pagination and optional search.
func (s *OrganizationsStore) ListOrganizations(ctx context.Context, projectID uuid.UUID, pagination store.Pagination, search string) (Organizations, int, error) {
	q := `
	SELECT o.id, o.project_id, o.name, o.data, o.version, o.created_at, o.updated_at,
		COALESCE(oeia.external_ids, '[]'::jsonb) AS external_ids,
		COUNT(*) OVER () AS total_count
	FROM organizations o
	LEFT JOIN organization_external_ids_agg oeia ON oeia.organization_id = o.id
	WHERE o.project_id = $1
	AND (
		$2 = '' OR
		o.name ILIKE '%' || $2 || '%' OR
		EXISTS (
			SELECT 1 FROM organization_external_ids oei
			WHERE oei.organization_id = o.id
			AND oei.external_id ILIKE '%' || $2 || '%'
		)
	)
	ORDER BY o.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		Organization
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, q, projectID, search, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []Organization{}, 0, nil
	}

	total := results[0].TotalCount
	orgs := make([]Organization, len(results))
	for i, r := range results {
		orgs[i] = r.Organization
	}

	return orgs, total, nil
}

// OrganizationUser represents a user's membership in an organization.
type OrganizationUser struct {
	ID             uuid.UUID       `db:"id"`
	OrganizationID uuid.UUID       `db:"organization_id"`
	UserID         uuid.UUID       `db:"user_id"`
	Data           json.RawMessage `db:"data"`
	Version        int32           `db:"version"`
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
}

// UpsertAndGetOrganizationMember adds or updates a user's membership in an organization with optional org-specific data.
// Returns the full organization user record including the version (version 0 = newly created, version > 0 = updated).
func (s *OrganizationsStore) UpsertAndGetOrganizationMember(ctx context.Context, orgID, userID uuid.UUID, data map[string]any) (*OrganizationUser, error) {
	if data == nil {
		data = make(map[string]any)
	}

	stmt := `
	INSERT INTO organization_users (organization_id, user_id, data)
	VALUES ($1, $2, $3)
	ON CONFLICT (organization_id, user_id) DO UPDATE SET
		data = organization_users.data || EXCLUDED.data
	RETURNING id, organization_id, user_id, data, version, created_at, updated_at`

	var user OrganizationUser
	err := s.db.GetContext(ctx, &user, stmt, orgID, userID, data)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// RemoveUserFromOrganization removes a user from an organization.
func (s *OrganizationsStore) RemoveUserFromOrganization(ctx context.Context, orgID, userID uuid.UUID) error {
	stmt := `DELETE FROM organization_users WHERE organization_id = $1 AND user_id = $2`
	_, err := s.db.ExecContext(ctx, stmt, orgID, userID)
	return err
}

// UpdateOrganizationUserData updates the org-specific data for a user in an organization.
func (s *OrganizationsStore) UpdateOrganizationUserData(ctx context.Context, orgID, userID uuid.UUID, data json.RawMessage) error {
	stmt := `
	UPDATE organization_users
	SET data = data || $3::jsonb
	WHERE organization_id = $1 AND user_id = $2`

	_, err := s.db.ExecContext(ctx, stmt, orgID, userID, data)
	return err
}

// OrganizationMember represents a user with their org-specific data.
type OrganizationMember struct {
	User
	OrganizationData json.RawMessage `db:"org_data"`
}

func (m *OrganizationMember) OAPI() oapi.OrganizationMember {
	return oapi.OrganizationMember{
		Id:               m.ID,
		ProjectId:        m.ProjectID,
		Identifier:       m.ExternalIDs.OAPI(),
		Email:            m.Email,
		Phone:            m.Phone,
		Data:             m.Data,
		Timezone:         m.Timezone,
		Locale:           m.Locale,
		HasPushDevice:    m.HasPushDevice,
		Version:          m.Version,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
		OrganizationData: m.OrganizationData,
	}
}

type OrganizationMembers []OrganizationMember

func (m OrganizationMembers) OAPI() []oapi.OrganizationMember {
	results := make([]oapi.OrganizationMember, len(m))
	for i, member := range m {
		results[i] = member.OAPI()
	}
	return results
}

// ListOrganizationMembers lists all users belonging to an organization with pagination.
func (s *OrganizationsStore) ListOrganizationMembers(ctx context.Context, projectID, orgID uuid.UUID, pagination store.Pagination) (OrganizationMembers, int, error) {
	q := `
	SELECT u.id, u.project_id, u.email, u.phone, u.data, u.timezone, u.locale, u.version, u.created_at, u.updated_at,
		EXISTS(
			SELECT 1 FROM devices d
			WHERE d.user_id = u.id
			AND d.token IS NOT NULL
			AND d.token != ''
		) as has_push_device,
		COALESCE(ueia.external_ids, '[]'::jsonb) AS external_ids,
		ou.data as org_data,
		COUNT(*) OVER () AS total_count
	FROM users u
	INNER JOIN organization_users ou ON u.id = ou.user_id
	LEFT JOIN user_external_ids_agg ueia ON ueia.user_id = u.id
	WHERE ou.organization_id = $1 AND u.project_id = $2
	ORDER BY ou.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		OrganizationMember
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, q, orgID, projectID, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []OrganizationMember{}, 0, nil
	}

	total := results[0].TotalCount
	members := make([]OrganizationMember, len(results))
	for i, r := range results {
		members[i] = r.OrganizationMember
	}

	return members, total, nil
}

// ListUserOrganizations lists all organizations a user belongs to.
func (s *OrganizationsStore) ListUserOrganizations(ctx context.Context, projectID, userID uuid.UUID, pagination store.Pagination, search string) ([]Organization, int, error) {
	q := `
	SELECT o.id, o.project_id, o.name, o.data, o.version, o.created_at, o.updated_at,
		COALESCE(oeia.external_ids, '[]'::jsonb) AS external_ids,
		COUNT(*) OVER () AS total_count
	FROM organizations o
	INNER JOIN organization_users ou ON o.id = ou.organization_id
	LEFT JOIN organization_external_ids_agg oeia ON oeia.organization_id = o.id
	WHERE ou.user_id = $1 AND o.project_id = $2
	AND (
		$5 = '' OR
		o.name ILIKE '%' || $5 || '%' OR
		EXISTS (
			SELECT 1 FROM organization_external_ids oei
			WHERE oei.organization_id = o.id
			AND oei.external_id ILIKE '%' || $5 || '%'
		)
	)
	ORDER BY ou.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		Organization
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, q, userID, projectID, pagination.Limit, pagination.Offset, search)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []Organization{}, 0, nil
	}

	total := results[0].TotalCount
	orgs := make([]Organization, len(results))
	for i, r := range results {
		orgs[i] = r.Organization
	}

	return orgs, total, nil
}

// CountOrganizationMembers returns the number of members in an organization.
func (s *OrganizationsStore) CountOrganizationMembers(ctx context.Context, orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM organization_users WHERE organization_id = $1`

	var count int
	err := s.db.GetContext(ctx, &count, query, orgID)
	return count, err
}

// UpsertOrganizationSchema inserts or updates schema paths for organization data.
func (s *OrganizationsStore) UpsertOrganizationSchema(ctx context.Context, projectID uuid.UUID, paths rules.Paths) error {
	return s.upsertSubjectSchema(ctx, projectID, SubjectTypeOrganization, paths)
}

// UpsertOrganizationUserSchema inserts or updates schema paths for organization user data.
func (s *OrganizationsStore) UpsertOrganizationUserSchema(ctx context.Context, projectID uuid.UUID, paths rules.Paths) error {
	return s.upsertSubjectSchema(ctx, projectID, SubjectTypeOrganizationUser, paths)
}

func (s *OrganizationsStore) upsertSubjectSchema(ctx context.Context, projectID uuid.UUID, subjectType SubjectType, paths rules.Paths) error {
	if len(paths) == 0 {
		return nil
	}

	stmt := `
	INSERT INTO subject_schemas (project_id, path, data_type, subject_type)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (project_id, path, data_type, subject_type) DO NOTHING`

	// TODO: consider batch insert if path count becomes large enough to impact performance.
	for _, path := range paths {
		_, err := s.db.ExecContext(ctx, stmt, projectID, path.Path, path.Type, subjectType)
		if err != nil {
			return err
		}
	}

	return nil
}

// OrganizationSchema is an alias for SubjectSchema for backwards compatibility
type OrganizationSchema = SubjectSchema

// OrganizationUserSchema is an alias for SubjectSchema for backwards compatibility
type OrganizationUserSchema = SubjectSchema

// ListOrganizationSchemas returns all schema paths for organizations in a project.
func (s *OrganizationsStore) ListOrganizationSchemas(ctx context.Context, projectID uuid.UUID) ([]SubjectSchema, error) {
	return s.listSubjectSchemas(ctx, projectID, SubjectTypeOrganization)
}

// ListOrganizationUserSchemas returns all schema paths for organization users in a project.
func (s *OrganizationsStore) ListOrganizationUserSchemas(ctx context.Context, projectID uuid.UUID) ([]SubjectSchema, error) {
	return s.listSubjectSchemas(ctx, projectID, SubjectTypeOrganizationUser)
}

func (s *OrganizationsStore) listSubjectSchemas(ctx context.Context, projectID uuid.UUID, subjectType SubjectType) ([]SubjectSchema, error) {
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

// InsertOrganizationEvent inserts an event occurrence for an organization.
func (s *OrganizationsStore) InsertOrganizationEvent(ctx context.Context, organizationID uuid.UUID, eventID uuid.UUID, data map[string]any) (uuid.UUID, error) {
	stmt := `
	INSERT INTO organization_events (organization_id, event_id, data)
	VALUES ($1, $2, $3)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, organizationID, eventID, data)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

// OrganizationEvent represents an event occurrence for an organization.
type OrganizationEvent struct {
	ID             uuid.UUID       `db:"id"`
	ProjectID      uuid.UUID       `db:"project_id"`
	OrganizationID uuid.UUID       `db:"organization_id"`
	EventID        uuid.UUID       `db:"event_id"`
	Name           string          `db:"name"`
	Data           json.RawMessage `db:"data"`
	CreatedAt      time.Time       `db:"created_at"`
}

func (e *OrganizationEvent) OAPI() oapi.OrganizationEvent {
	return oapi.OrganizationEvent{
		Id:             e.ID,
		ProjectId:      e.ProjectID,
		OrganizationId: e.OrganizationID,
		Name:           e.Name,
		Data:           &e.Data,
		CreatedAt:      e.CreatedAt,
	}
}

type OrganizationEvents []OrganizationEvent

func (e OrganizationEvents) OAPI() []oapi.OrganizationEvent {
	results := make([]oapi.OrganizationEvent, len(e))
	for i, event := range e {
		results[i] = event.OAPI()
	}
	return results
}

// ListOrganizationEvents retrieves events for an organization with pagination.
func (s *OrganizationsStore) ListOrganizationEvents(ctx context.Context, projectID, organizationID uuid.UUID, pagination store.Pagination) (OrganizationEvents, int, error) {
	query := `
	SELECT
		oe.id, o.project_id, oe.organization_id, oe.event_id, e.name, oe.data, oe.created_at,
		COUNT(*) OVER () AS total_count
	FROM organization_events oe
	INNER JOIN organizations o ON oe.organization_id = o.id
	INNER JOIN events e ON oe.event_id = e.id
	WHERE o.project_id = $1 AND oe.organization_id = $2
	ORDER BY oe.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		OrganizationEvent
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, organizationID, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []OrganizationEvent{}, 0, nil
	}

	total := results[0].TotalCount
	events := make([]OrganizationEvent, len(results))

	for index, result := range results {
		events[index] = result.OrganizationEvent
	}

	return events, total, nil
}

// ListOrganizationUserIDs returns all user IDs belonging to an organization.
func (s *OrganizationsStore) ListOrganizationUserIDs(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error) {
	q := `
	SELECT user_id
	FROM organization_users
	WHERE organization_id = $1`

	var ids []uuid.UUID
	err := s.db.SelectContext(ctx, &ids, q, orgID)
	if err != nil {
		return nil, err
	}

	return ids, nil
}

// QueryOrganizationUserIDs returns a cursor for iterating over user IDs in an organization.
// The caller is responsible for closing the rows.
func (s *OrganizationsStore) QueryOrganizationUserIDs(ctx context.Context, orgID uuid.UUID) (*sqlx.Rows, error) {
	q := `
	SELECT user_id
	FROM organization_users
	WHERE organization_id = $1`

	return s.db.QueryxContext(ctx, q, orgID)
}

// QueryOrganizationUsersMatchingRule returns a cursor for iterating over user IDs in an organization
// that match the given ruleset. The caller is responsible for closing the rows.
func (s *OrganizationsStore) QueryOrganizationUsersMatchingRule(ctx context.Context, projectID, orgID uuid.UUID, ruleset rules.RuleSet) (*sqlx.Rows, error) {
	builder := query.NewQueryBuilder(projectID, nil)
	result, err := builder.Query(ruleset)
	if err != nil {
		return nil, err
	}

	// Wrap the ruleset query to filter only users in the specified organization
	q := fmt.Sprintf(`
	SELECT u.id AS user_id
	FROM organization_users ou
	JOIN (%s) u ON u.id = ou.user_id
	WHERE ou.organization_id = $%d`, result.SQL, len(result.Args)+1)

	args := append(result.Args, orgID)
	return s.db.QueryxContext(ctx, q, args...)
}

// ScanOrganizationMembers iterates over user IDs in an organization, optionally
// filtered by a ruleset, and calls fn for each user ID. Rows are read via a
// cursor so large result sets do not need to be held entirely in memory.
func (s *OrganizationsStore) ScanOrganizationMembers(ctx context.Context, projectID, orgID uuid.UUID, userRule *rules.RuleSet, fn func(userID uuid.UUID) error) error {
	var rows *sqlx.Rows
	var err error

	if userRule != nil {
		rows, err = s.QueryOrganizationUsersMatchingRule(ctx, projectID, orgID, *userRule)
	} else {
		rows, err = s.QueryOrganizationUserIDs(ctx, orgID)
	}
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var userID uuid.UUID
		if err := rows.Scan(&userID); err != nil {
			return err
		}
		if err := fn(userID); err != nil {
			return err
		}
	}

	return rows.Err()
}
