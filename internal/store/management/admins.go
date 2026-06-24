package management

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/store"
)

func NewAdminsStore(db store.DB) *AdminsStore {
	return &AdminsStore{db: db}
}

type AdminsStore struct {
	db store.DB
}

type Admin struct {
	ID                   uuid.UUID  `db:"id"`
	OrganizationID       uuid.UUID  `db:"organization_id"`
	ActiveOrganizationID *uuid.UUID `db:"active_organization_id"`
	ExternalID           *string    `db:"external_id"`
	Email                string     `db:"email"`
	FirstName            *string    `db:"first_name"`
	LastName             *string    `db:"last_name"`
	ImageURL             *string    `db:"image_url"`
	Role                 string     `db:"role"`
	CreatedAt            time.Time  `db:"created_at"`
	UpdatedAt            time.Time  `db:"updated_at"`
}

func (admin *Admin) OAPI() oapi.Admin {
	return oapi.Admin{
		Id:             admin.ID,
		OrganizationId: admin.OrganizationID,
		ExternalId:     admin.ExternalID,
		Email:          admin.Email,
		FirstName:      admin.FirstName,
		LastName:       admin.LastName,
		ImageUrl:       admin.ImageURL,
		Role:           oapi.OrganizationRole(admin.Role),
		CreatedAt:      admin.CreatedAt,
		UpdatedAt:      admin.UpdatedAt,
	}
}

func (s *AdminsStore) GetAdmin(ctx context.Context, id uuid.UUID) (*Admin, error) {
	stmt := `
	SELECT id, organization_id, active_organization_id, external_id, email, first_name, last_name, image_url, role, created_at, updated_at
	FROM admins
	WHERE id = $1
	AND deleted_at IS NULL`

	var admin Admin
	err := s.db.GetContext(ctx, &admin, stmt, id)
	if err != nil {
		return nil, err
	}

	return &admin, nil
}

func (s *AdminsStore) GetAdminByExternalID(ctx context.Context, externalID string) (*Admin, error) {
	stmt := `
	SELECT id, organization_id, active_organization_id, external_id, email, first_name, last_name, image_url, role, created_at, updated_at
	FROM admins
	WHERE external_id = $1
	AND deleted_at IS NULL`

	var admin Admin
	err := s.db.GetContext(ctx, &admin, stmt, externalID)
	if err != nil {
		return nil, err
	}

	return &admin, nil
}

func (s *AdminsStore) GetAdminBySubject(ctx context.Context, issuer, subject string) (*Admin, error) {
	if issuer != "" {
		admin, err := s.GetAdminByExternalID(ctx, subject)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		if admin != nil {
			return admin, nil
		}
	}

	adminID, err := uuid.Parse(subject)
	if err != nil {
		return nil, problem.ErrUnauthorized(problem.Describe("invalid token"))
	}

	return s.GetAdmin(ctx, adminID)
}

func (s *AdminsStore) CreateAdmin(ctx context.Context, admin Admin) (uuid.UUID, error) {
	// A newly created admin's active organization defaults to its home
	// organization; the switcher can change it later.
	stmt := `
	INSERT INTO admins (organization_id, active_organization_id, external_id, email, first_name, last_name, image_url, role)
	VALUES ($1, $1, $2, $3, $4, $5, $6, $7)
	RETURNING id
	`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		admin.OrganizationID,
		admin.ExternalID,
		admin.Email,
		admin.FirstName,
		admin.LastName,
		admin.ImageURL,
		admin.Role,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *AdminsStore) ListAdmins(ctx context.Context, organizationID uuid.UUID, pagination store.Pagination, search string) ([]Admin, int, error) {
	var admins []Admin
	var total int

	// Lists the members of the organization. Membership is the source of truth
	// (an admin can belong to several organizations), so this joins
	// organization_members rather than filtering admins.organization_id.
	query := `
	SELECT
		a.id, a.organization_id, a.active_organization_id, a.external_id, a.email, a.first_name, a.last_name, a.image_url, a.role, a.created_at, a.updated_at,
		COUNT(*) OVER () AS total_count
	FROM admins a
	JOIN organization_members om ON om.admin_id = a.id AND om.organization_id = $1 AND om.deleted_at IS NULL
	WHERE a.deleted_at IS NULL
	AND (
		$2 = '' OR
		a.first_name ILIKE '%' || $2 || '%' OR
		a.last_name ILIKE '%' || $2 || '%' OR
		a.email ILIKE '%' || $2 || '%'
	)
	ORDER BY a.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		Admin
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, organizationID, search, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []Admin{}, 0, nil
	}

	total = results[0].TotalCount
	for _, r := range results {
		admins = append(admins, r.Admin)
	}

	return admins, total, nil
}

// GetAdminByEmail looks up an admin by email, case-insensitively. Email
// matching must be case-insensitive everywhere it affects authorization (e.g.
// the cross-organization invite guard and login provisioning); an exact match
// would let an uppercase-stored email silently bypass those checks.
func (s *AdminsStore) GetAdminByEmail(ctx context.Context, email string) (*Admin, error) {
	stmt := `
	SELECT id, organization_id, active_organization_id, external_id, email, first_name, last_name, image_url, role, created_at, updated_at
	FROM admins
	WHERE lower(email) = lower($1)
	AND deleted_at IS NULL`

	var admin Admin
	err := s.db.GetContext(ctx, &admin, stmt, email)
	if err != nil {
		return nil, err
	}

	return &admin, nil
}

type AdminUpdate struct {
	Email     *string
	FirstName *string
	LastName  *string
	Role      *string
}

func (s *AdminsStore) UpdateAdmin(ctx context.Context, id uuid.UUID, update AdminUpdate) error {
	stmt := `
	UPDATE admins
	SET
		email = COALESCE($2, email),
		first_name = COALESCE($3, first_name),
		last_name = COALESCE($4, last_name),
		role = COALESCE($5, role)
	WHERE id = $1
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, id, update.Email, update.FirstName, update.LastName, update.Role)
	return err
}

func (s *AdminsStore) DeleteAdmin(ctx context.Context, id uuid.UUID) error {
	stmt := `UPDATE admins SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	_, err := s.db.ExecContext(ctx, stmt, id)
	return err
}

// SetActiveOrganization updates the admin's active organization, the one that
// scopes their session. Callers must verify the admin is a member of the
// organization first.
func (s *AdminsStore) SetActiveOrganization(ctx context.Context, adminID, organizationID uuid.UUID) error {
	stmt := `UPDATE admins SET active_organization_id = $2 WHERE id = $1 AND deleted_at IS NULL`
	_, err := s.db.ExecContext(ctx, stmt, adminID, organizationID)
	return err
}

// Project Admin methods

type ProjectAdmin struct {
	ID        uuid.UUID  `db:"id"`
	ProjectID uuid.UUID  `db:"project_id"`
	AdminID   uuid.UUID  `db:"admin_id"`
	Role      string     `db:"role"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
	Email     *string    `db:"email"`
	FirstName *string    `db:"first_name"`
	LastName  *string    `db:"last_name"`
}

func (pa *ProjectAdmin) OAPI() oapi.ProjectAdmin {
	result := oapi.ProjectAdmin{
		Id:        pa.ID,
		ProjectId: pa.ProjectID,
		AdminId:   pa.AdminID,
		Role:      oapi.ProjectRole(pa.Role),
		CreatedAt: pa.CreatedAt,
		UpdatedAt: pa.UpdatedAt,
	}

	if pa.Email != nil {
		result.Email = pa.Email
	}
	if pa.FirstName != nil {
		result.FirstName = pa.FirstName
	}
	if pa.LastName != nil {
		result.LastName = pa.LastName
	}

	return result
}

type ProjectAdmins []ProjectAdmin

func (pas ProjectAdmins) OAPI() []oapi.ProjectAdmin {
	results := make([]oapi.ProjectAdmin, len(pas))
	for i, pa := range pas {
		results[i] = pa.OAPI()
	}
	return results
}

func (s *AdminsStore) ListProjectAdmins(ctx context.Context, projectID uuid.UUID, pagination store.Pagination, search string) ([]ProjectAdmin, int, error) {
	var projectAdmins []ProjectAdmin
	var total int

	query := `
	SELECT
		project_admins.id,
		project_admins.project_id,
		project_admins.admin_id,
		project_admins.role,
		project_admins.created_at,
		project_admins.updated_at,
		admins.email,
		admins.first_name,
		admins.last_name,
		COUNT(*) OVER () AS total_count
	FROM project_admins
	JOIN admins ON project_admins.admin_id = admins.id
	WHERE project_admins.project_id = $1
	AND project_admins.deleted_at IS NULL
	AND (
		$2 = '' OR
		admins.first_name ILIKE '%' || $2 || '%' OR
		admins.last_name ILIKE '%' || $2 || '%' OR
		admins.email ILIKE '%' || $2 || '%'
	)
	ORDER BY project_admins.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		ProjectAdmin
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, search, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []ProjectAdmin{}, 0, nil
	}

	total = results[0].TotalCount
	for _, r := range results {
		projectAdmins = append(projectAdmins, r.ProjectAdmin)
	}

	return projectAdmins, total, nil
}

func (s *AdminsStore) GetProjectAdmin(ctx context.Context, projectID, adminID uuid.UUID) (*ProjectAdmin, error) {
	query := `
	SELECT
		project_admins.id,
		project_admins.project_id,
		project_admins.admin_id,
		project_admins.role,
		project_admins.created_at,
		project_admins.updated_at,
		admins.email,
		admins.first_name,
		admins.last_name
	FROM project_admins
	JOIN admins ON project_admins.admin_id = admins.id
	WHERE project_admins.project_id = $1
	AND project_admins.admin_id = $2
	AND project_admins.deleted_at IS NULL`

	var projectAdmin ProjectAdmin
	err := s.db.GetContext(ctx, &projectAdmin, query, projectID, adminID)
	if err != nil {
		return nil, err
	}

	return &projectAdmin, nil
}

func (s *AdminsStore) AddAdminToProject(ctx context.Context, projectID, adminID uuid.UUID, role string) error {
	query := `
	INSERT INTO project_admins (project_id, admin_id, role)
	VALUES ($1, $2, $3)`

	_, err := s.db.ExecContext(ctx, query, projectID, adminID, role)
	return err
}

func (s *AdminsStore) UpdateProjectAdminRole(ctx context.Context, projectID, adminID uuid.UUID, role string) error {
	query := `
	UPDATE project_admins
	SET role = $1
	WHERE project_id = $2
	AND admin_id = $3
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, role, projectID, adminID)
	return err
}

func (s *AdminsStore) DeleteProjectAdmin(ctx context.Context, projectID, adminID uuid.UUID) error {
	query := `
	UPDATE project_admins
	SET deleted_at = NOW()
	WHERE project_id = $1
	AND admin_id = $2
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, projectID, adminID)
	return err
}

func (s *AdminsStore) HardDeleteProjectAdmin(ctx context.Context, projectID, adminID uuid.UUID) error {
	query := `
	DELETE FROM project_admins
	WHERE project_id = $1
	AND admin_id = $2`

	_, err := s.db.ExecContext(ctx, query, projectID, adminID)
	return err
}
