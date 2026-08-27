package management

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
)

type Project struct {
	ID                uuid.UUID  `db:"id"`
	OrganizationID    *uuid.UUID `db:"organization_id"`
	Name              string     `db:"name"`
	Description       *string    `db:"description"`
	Timezone          string     `db:"timezone"`
	TextOptOutMessage *string    `db:"text_opt_out_message"`
	TextHelpMessage   *string    `db:"text_help_message"`
	Locale            string     `db:"locale"`
	Role              string     `db:"role"`
	IntegrationsCount int        `db:"integrations_count"`
	CampaignsCount    int        `db:"campaigns_count"`
	JourneysCount     int        `db:"journeys_count"`
	UsersCount        int        `db:"users_count"`
	ListsCount        int        `db:"lists_count"`
	CreatedAt         time.Time  `db:"created_at"`
	UpdatedAt         time.Time  `db:"updated_at"`
}

// ProjectAccess is a project together with the organization membership role of
// the admin the query was run for. The membership role is carried out of SQL
// unresolved so that org→project inheritance is applied in one place in Go
// (see rbac.EffectiveProjectRole) rather than being reimplemented per query.
type ProjectAccess struct {
	Project
	MembershipRole string `db:"membership_role"`
}

func (p *Project) OAPI() oapi.Project {
	project := oapi.Project{
		Id:                p.ID,
		Name:              p.Name,
		Timezone:          p.Timezone,
		Locale:            p.Locale,
		IntegrationsCount: &p.IntegrationsCount,
		CampaignsCount:    &p.CampaignsCount,
		JourneysCount:     &p.JourneysCount,
		UsersCount:        &p.UsersCount,
		ListsCount:        &p.ListsCount,
		Role:              p.Role,
		CreatedAt:         p.CreatedAt,
		UpdatedAt:         p.UpdatedAt,
	}

	if p.OrganizationID != nil {
		project.OrganizationId = p.OrganizationID
	}

	if p.Description != nil {
		project.Description = p.Description
	}

	if p.TextOptOutMessage != nil {
		project.TextOptOutMessage = p.TextOptOutMessage
	}

	if p.TextHelpMessage != nil {
		project.TextHelpMessage = p.TextHelpMessage
	}

	return project
}

func NewProjectsStore(db store.DB) *ProjectsStore {
	return &ProjectsStore{db: db}
}

type ProjectsStore struct {
	db store.DB
}

func (s *ProjectsStore) CreateProject(ctx context.Context, project Project) (uuid.UUID, error) {
	stmt := `
	INSERT INTO projects (organization_id, name, description, timezone, text_opt_out_message, text_help_message, locale)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, project.OrganizationID, project.Name, project.Description, project.Timezone, project.TextOptOutMessage, project.TextHelpMessage, project.Locale)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *ProjectsStore) GetProject(ctx context.Context, id uuid.UUID, adminID *uuid.UUID) (*Project, error) {
	return s.getProject(ctx, id, nil, adminID)
}

// GetProjectInOrganization loads a project only when it belongs to the given
// organization. A project owned by another organization is reported as
// sql.ErrNoRows, exactly like a project that does not exist: an
// organization-scoped caller must not be able to tell the two apart, or a
// project id becomes an existence oracle across tenants.
func (s *ProjectsStore) GetProjectInOrganization(ctx context.Context, id, organizationID uuid.UUID, adminID *uuid.UUID) (*Project, error) {
	return s.getProject(ctx, id, &organizationID, adminID)
}

func (s *ProjectsStore) getProject(ctx context.Context, id uuid.UUID, organizationID *uuid.UUID, adminID *uuid.UUID) (*Project, error) {
	query := `
	SELECT id, organization_id, name, description, timezone, text_opt_out_message, text_help_message, locale, created_at, updated_at,
		COALESCE(pr.integrations_count, 0) AS integrations_count,
		COALESCE(ca.campaigns_count, 0)    AS campaigns_count,
		COALESCE(pa.role, '') AS role
	FROM projects
	LEFT JOIN (
		SELECT project_id, COUNT(*) AS integrations_count
		FROM providers
		WHERE deleted_at IS NULL
		GROUP BY project_id
	) pr ON pr.project_id = projects.id
	LEFT JOIN (
		SELECT project_id, COUNT(*) AS campaigns_count
		FROM campaigns
		WHERE deleted_at IS NULL
		GROUP BY project_id
	) ca ON ca.project_id = projects.id
	LEFT JOIN (
		SELECT project_id, role
		FROM project_admins
		WHERE $2::uuid IS NOT NULL AND admin_id = $2 AND deleted_at IS NULL	
	) pa ON pa.project_id = projects.id
	WHERE id = $1
	AND deleted_at IS NULL
	AND ($3::uuid IS NULL OR organization_id = $3)`

	var project Project
	err := s.db.GetContext(ctx, &project, query, id, adminID, organizationID)
	if err != nil {
		return nil, err
	}

	return &project, nil
}

func (s *ProjectsStore) ListProjects(ctx context.Context, organizationID uuid.UUID, pagination store.Pagination, search string) ([]Project, int, error) {
	// TODO: include counts as in GetProject
	query := `
	SELECT DISTINCT p.id, p.organization_id, p.name, p.description, p.timezone, p.text_opt_out_message,
	       p.text_help_message, p.locale,
	       p.created_at, p.updated_at,
	       COUNT(*) OVER() AS total_count
	FROM projects p
	WHERE p.deleted_at IS NULL
	  AND p.organization_id = $1
	  AND ($2 = '' OR p.name ILIKE '%' || $2 || '%')
	ORDER BY p.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		Project
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, organizationID, search, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []Project{}, 0, nil
	}

	projects := make([]Project, len(results))
	for i, r := range results {
		projects[i] = r.Project
	}

	return projects, results[0].TotalCount, nil
}

// ListProjectsForAdmin returns the projects of one organization that the admin
// can reach: those they hold an explicit project_admins row for, plus — for an
// organization owner or admin — every project in the organization, since those
// roles are project admins by inheritance and would otherwise never see a
// project they did not create a row for.
//
// The organization predicate is not optional. Without it the list spans every
// organization the admin has ever been given a project role in, leaking other
// tenants' project names into the session of whichever organization happens to
// be active.
//
// inheritingOrgRoles are the organization roles that confer project admin by
// inheritance; they are passed in rather than spelled out in SQL so that
// [rbac.OrganizationRolesInheritingProjectAdmin] stays the only definition of
// that rule. The membership role is returned alongside the explicit project
// role so the caller can resolve the effective role in Go.
func (s *ProjectsStore) ListProjectsForAdmin(ctx context.Context, organizationID, adminID uuid.UUID, inheritingOrgRoles []string, pagination store.Pagination, search string) ([]ProjectAccess, int, error) {
	// TODO: include counts as in GetProject
	query := `
	SELECT p.id, p.organization_id, p.name, p.timezone, p.text_help_message, p.locale,
	       p.created_at, p.updated_at,
	       COALESCE(pa.role, '') AS role,
	       om.role AS membership_role,
	       COUNT(*) OVER() AS total_count
	FROM projects p
	JOIN organization_members om ON om.organization_id = p.organization_id AND om.admin_id = $1 AND om.deleted_at IS NULL
	LEFT JOIN project_admins pa ON pa.project_id = p.id AND pa.admin_id = $1 AND pa.deleted_at IS NULL
	WHERE p.deleted_at IS NULL
	AND p.organization_id = $2
	AND (pa.id IS NOT NULL OR om.role = ANY($3))
	AND ($4 = '' OR p.name ILIKE '%' || $4 || '%')
	ORDER BY p.created_at DESC
	LIMIT $5 OFFSET $6`

	type result struct {
		ProjectAccess
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, adminID, organizationID, pq.Array(inheritingOrgRoles), search, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []ProjectAccess{}, 0, nil
	}

	projects := make([]ProjectAccess, len(results))
	for i, r := range results {
		projects[i] = r.ProjectAccess
	}

	return projects, results[0].TotalCount, nil
}

type ProjectUpdate struct {
	Name              *string
	Description       *string
	Timezone          *string
	Locale            *string
	TextOptOutMessage *string
	TextHelpMessage   *string
}

func (s *ProjectsStore) UpdateProject(ctx context.Context, projectID uuid.UUID, update ProjectUpdate) error {
	query := `
	UPDATE projects
	SET name = COALESCE($2, name),
	    description = COALESCE($3, description),
	    timezone = COALESCE($4, timezone),
	    locale = COALESCE($5, locale),
	    text_opt_out_message = COALESCE($6, text_opt_out_message),
	    text_help_message = COALESCE($7, text_help_message)
	WHERE id = $1
	  AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, projectID, update.Name, update.Description, update.Timezone, update.Locale, update.TextOptOutMessage, update.TextHelpMessage)
	return err
}

func (s *ProjectsStore) DeleteProject(ctx context.Context, projectID uuid.UUID) error {
	query := `
	UPDATE projects
	SET deleted_at = NOW()
	WHERE id = $1 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, projectID)
	return err
}

// RemoveProjectAdmins soft-deletes the whole roster of a project. It runs with
// [ProjectsStore.DeleteProject] so a deleted project leaves no live membership
// rows behind, which would otherwise be replayed as grants to revoke long after
// the tuples they refer to are gone.
func (s *ProjectsStore) RemoveProjectAdmins(ctx context.Context, projectID uuid.UUID) error {
	query := `
	UPDATE project_admins
	SET deleted_at = NOW()
	WHERE project_id = $1 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, projectID)
	return err
}

func (s *ProjectsStore) GetProjectRole(ctx context.Context, projectID, adminID uuid.UUID) (string, error) {
	query := `
	SELECT role
	FROM project_admins
	WHERE project_id = $1
	  AND admin_id = $2
	  AND deleted_at IS NULL`

	var role string
	err := s.db.GetContext(ctx, &role, query, projectID, adminID)
	if err != nil {
		return "", err
	}

	return role, nil
}

func (s *ProjectsStore) AddProjectAdmin(ctx context.Context, projectID, adminID uuid.UUID, role string) error {
	query := `
	INSERT INTO project_admins (project_id, admin_id, role)
	VALUES ($1, $2, $3)`

	_, err := s.db.ExecContext(ctx, query, projectID, adminID, role)
	return err
}

// ProjectRole is one project_admins grant: the project and the role held in it.
type ProjectRole struct {
	ProjectID uuid.UUID `db:"project_id"`
	Role      string    `db:"role"`
}

// ListProjectRolesInOrganization returns every explicit project role the admin
// holds in the organization's projects. Removing an admin from an organization
// must revoke these as well: project access resolves from the direct
// project:<id>#<role> tuple alone, so an untouched project role keeps working
// long after the organization membership is gone.
func (s *ProjectsStore) ListProjectRolesInOrganization(ctx context.Context, organizationID, adminID uuid.UUID) ([]ProjectRole, error) {
	query := `
	SELECT pa.project_id, pa.role
	FROM project_admins pa
	JOIN projects p ON p.id = pa.project_id
	WHERE p.organization_id = $1
	  AND p.deleted_at IS NULL
	  AND pa.admin_id = $2
	  AND pa.deleted_at IS NULL`

	var roles []ProjectRole
	if err := s.db.SelectContext(ctx, &roles, query, organizationID, adminID); err != nil {
		return nil, err
	}
	return roles, nil
}

// RemoveProjectAdminsInOrganization soft-deletes the admin's project_admins
// rows for every project in the organization. Deleted projects are included on
// purpose: the row is dead either way and leaving it behind would resurrect the
// membership if the project ever came back.
func (s *ProjectsStore) RemoveProjectAdminsInOrganization(ctx context.Context, organizationID, adminID uuid.UUID) error {
	query := `
	UPDATE project_admins
	SET deleted_at = NOW()
	WHERE admin_id = $2
	  AND deleted_at IS NULL
	  AND project_id IN (SELECT id FROM projects WHERE organization_id = $1)`

	_, err := s.db.ExecContext(ctx, query, organizationID, adminID)
	return err
}

// ProjectAdminRole is one project_admins grant: the admin and the role held.
type ProjectAdminRole struct {
	AdminID uuid.UUID `db:"admin_id"`
	Role    string    `db:"role"`
}

// ListProjectAdminRoles returns every explicit role grant on the project. It is
// read before a project is deleted so the per-admin role tuples can be revoked;
// deprovisioning the project itself only removes the project→organization and
// resource→project tuples, leaving the direct role grants behind.
func (s *ProjectsStore) ListProjectAdminRoles(ctx context.Context, projectID uuid.UUID) ([]ProjectAdminRole, error) {
	query := `
	SELECT admin_id, role
	FROM project_admins
	WHERE project_id = $1
	  AND deleted_at IS NULL`

	var roles []ProjectAdminRole
	if err := s.db.SelectContext(ctx, &roles, query, projectID); err != nil {
		return nil, err
	}
	return roles, nil
}

// HasOtherProjectAdmin reports whether the project would still have somebody
// able to administer it once excludingAdminID no longer holds project admin.
// That is true when another active project_admins row carries adminRole, or
// when the owning organization still has a member holding one of
// inheritingOrgRoles (who is a project admin by inheritance, with or without a
// row of their own).
//
// Callers use this to refuse the change that would orphan a project. Counting
// the inherited admins matters: without them the guard would block an
// organization owner from ever removing a departed project admin.
func (s *ProjectsStore) HasOtherProjectAdmin(ctx context.Context, projectID, excludingAdminID uuid.UUID, adminRole string, inheritingOrgRoles []string) (bool, error) {
	query := `
	SELECT EXISTS (
		SELECT 1
		FROM project_admins pa
		WHERE pa.project_id = $1
		  AND pa.admin_id <> $2
		  AND pa.role = $3
		  AND pa.deleted_at IS NULL
	) OR EXISTS (
		SELECT 1
		FROM projects p
		JOIN organization_members om ON om.organization_id = p.organization_id AND om.deleted_at IS NULL
		WHERE p.id = $1
		  AND p.deleted_at IS NULL
		  AND om.admin_id <> $2
		  AND om.role = ANY($4)
	)`

	var exists bool
	if err := s.db.GetContext(ctx, &exists, query, projectID, excludingAdminID, adminRole, pq.Array(inheritingOrgRoles)); err != nil {
		return false, err
	}
	return exists, nil
}
