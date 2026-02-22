package subjects

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
)

type Organizations []Organization

type Organization struct {
	ID         uuid.UUID       `db:"id"`
	ProjectID  uuid.UUID       `db:"project_id"`
	ExternalID string          `db:"external_id"`
	Name       *string         `db:"name"`
	Data       json.RawMessage `db:"data"`
	Version    int32           `db:"version"`
	CreatedAt  time.Time       `db:"created_at"`
	UpdatedAt  time.Time       `db:"updated_at"`
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
	SELECT id, project_id, external_id, name, data, version, created_at, updated_at
	FROM organizations
	WHERE id = $1 AND project_id = $2`

	var org Organization
	err := s.db.GetContext(ctx, &org, stmt, orgID, projectID)
	if err != nil {
		return nil, err
	}

	return &org, nil
}

// GetOrganizationByExternalID retrieves an organization by its external ID.
func (s *OrganizationsStore) GetOrganizationByExternalID(ctx context.Context, projectID uuid.UUID, externalID string) (*Organization, error) {
	stmt := `
	SELECT id, project_id, external_id, name, data, version, created_at, updated_at
	FROM organizations
	WHERE external_id = $1 AND project_id = $2`

	var org Organization
	err := s.db.GetContext(ctx, &org, stmt, externalID, projectID)
	if err != nil {
		return nil, err
	}

	return &org, nil
}

type UpsertOrganizationParams struct {
	ExternalID string
	Name       *string
	Data       map[string]any
}

// UpsertOrganization creates or updates an organization by external ID.
func (s *OrganizationsStore) UpsertOrganization(ctx context.Context, projectID uuid.UUID, params UpsertOrganizationParams) (uuid.UUID, error) {
	data := params.Data
	if data == nil {
		data = make(map[string]any)
	}

	stmt := `
	INSERT INTO organizations (project_id, external_id, name, data)
	VALUES ($1, $2, $3, $4)
	ON CONFLICT (project_id, external_id)
	DO UPDATE SET
		name = COALESCE(EXCLUDED.name, organizations.name),
		data = organizations.data || EXCLUDED.data
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, projectID, params.ExternalID, params.Name, data)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
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

// DeleteOrganization deletes an organization and all its user memberships (via CASCADE).
func (s *OrganizationsStore) DeleteOrganization(ctx context.Context, projectID, orgID uuid.UUID) error {
	stmt := `DELETE FROM organizations WHERE id = $1 AND project_id = $2`
	_, err := s.db.ExecContext(ctx, stmt, orgID, projectID)
	return err
}

// ListOrganizations lists all organizations for a project with pagination.
func (s *OrganizationsStore) ListOrganizations(ctx context.Context, projectID uuid.UUID, pagination store.Pagination) (Organizations, int, error) {
	query := `
	SELECT
		id, project_id, external_id, name, data, version, created_at, updated_at,
		COUNT(*) OVER () AS total_count
	FROM organizations
	WHERE project_id = $1
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	type result struct {
		Organization
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, pagination.Limit, pagination.Offset)
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
	CreatedAt      time.Time       `db:"created_at"`
	UpdatedAt      time.Time       `db:"updated_at"`
}

// UpsertOrganizationMember adds or updates a user's membership in an organization with optional org-specific data.
func (s *OrganizationsStore) UpsertOrganizationMember(ctx context.Context, orgID, userID uuid.UUID, data map[string]any) error {
	if data == nil {
		data = make(map[string]any)
	}

	stmt := `
	INSERT INTO organization_users (organization_id, user_id, data)
	VALUES ($1, $2, $3)
	ON CONFLICT (organization_id, user_id) DO UPDATE SET
		data = organization_users.data || EXCLUDED.data`

	_, err := s.db.ExecContext(ctx, stmt, orgID, userID, data)
	return err
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

type OrganizationMembers []OrganizationMember

// ListOrganizationMembers lists all users belonging to an organization with pagination.
func (s *OrganizationsStore) ListOrganizationMembers(ctx context.Context, projectID, orgID uuid.UUID, pagination store.Pagination) (OrganizationMembers, int, error) {
	query := `
	SELECT
		u.id, u.project_id, u.anonymous_id, u.external_id, u.email, u.phone, u.data, u.timezone, u.locale, u.version, u.created_at, u.updated_at,
		EXISTS(
			SELECT 1 FROM devices d
			WHERE d.user_id = u.id
			AND d.token IS NOT NULL
			AND d.token != ''
		) as has_push_device,
		ou.data as org_data,
		COUNT(*) OVER () AS total_count
	FROM users u
	INNER JOIN organization_users ou ON u.id = ou.user_id
	WHERE ou.organization_id = $1 AND u.project_id = $2
	ORDER BY ou.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		OrganizationMember
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, orgID, projectID, pagination.Limit, pagination.Offset)
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
func (s *OrganizationsStore) ListUserOrganizations(ctx context.Context, projectID, userID uuid.UUID) ([]Organization, error) {
	query := `
	SELECT o.id, o.project_id, o.external_id, o.name, o.data, o.version, o.created_at, o.updated_at
	FROM organizations o
	INNER JOIN organization_users ou ON o.id = ou.organization_id
	WHERE ou.user_id = $1 AND o.project_id = $2
	ORDER BY ou.created_at DESC`

	var orgs []Organization
	err := s.db.SelectContext(ctx, &orgs, query, userID, projectID)
	if err != nil {
		return nil, err
	}

	return orgs, nil
}

// CountOrganizationMembers returns the number of members in an organization.
func (s *OrganizationsStore) CountOrganizationMembers(ctx context.Context, orgID uuid.UUID) (int, error) {
	query := `SELECT COUNT(*) FROM organization_users WHERE organization_id = $1`

	var count int
	err := s.db.GetContext(ctx, &count, query, orgID)
	return count, err
}
