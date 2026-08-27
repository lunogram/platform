package management

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
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

// GetProject returns a project by ID, regardless of which organization owns it.
//
// Callers that reached this point through an organization-scoped permission
// check must use [ProjectsStore.GetOrganizationProject] instead: proving an
// actor may read projects in its own organization says nothing about the
// project ID it supplied.
func (s *ProjectsStore) GetProject(ctx context.Context, id uuid.UUID, adminID *uuid.UUID) (*Project, error) {
	return s.getProject(ctx, id, nil, adminID)
}

// GetOrganizationProject returns a project only when the given organization
// owns it, and sql.ErrNoRows otherwise -- reporting a project in another
// organization exactly as it reports one that does not exist, so the two cannot
// be told apart.
func (s *ProjectsStore) GetOrganizationProject(ctx context.Context, id, organizationID uuid.UUID, adminID *uuid.UUID) (*Project, error) {
	return s.getProject(ctx, id, &organizationID, adminID)
}

// getProject reads a project, optionally confined to one organization. A nil
// organizationID matches any owner, including a project with none.
func (s *ProjectsStore) getProject(ctx context.Context, id uuid.UUID, organizationID, adminID *uuid.UUID) (*Project, error) {
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
	AND ($3::uuid IS NULL OR organization_id = $3)
	AND deleted_at IS NULL`

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

func (s *ProjectsStore) ListProjectsForAdmin(ctx context.Context, adminID uuid.UUID, pagination store.Pagination, search string) ([]Project, int, error) {
	// TODO: include counts as in GetProject
	query := `
	SELECT p.id, p.name, p.timezone, p.text_help_message, p.locale,
	       p.created_at, p.updated_at,
	       pa.role,
	       COUNT(*) OVER() AS total_count
	FROM projects p
	JOIN project_admins pa ON pa.project_id = p.id AND pa.admin_id = $1 AND pa.deleted_at IS NULL
	WHERE p.deleted_at IS NULL
	AND ($2 = '' OR p.name ILIKE '%' || $2 || '%')
	ORDER BY p.created_at DESC
	LIMIT $3 OFFSET $4`

	type result struct {
		Project
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, adminID, search, pagination.Limit, pagination.Offset)
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

type ProjectUpdate struct {
	Name              *string
	Description       *string
	Timezone          *string
	Locale            *string
	TextOptOutMessage *string
	TextHelpMessage   *string
}

// UpdateProject applies an update to a project the given organization owns, and
// returns sql.ErrNoRows when it owns no such project. The ownership test is part
// of the statement rather than a preceding read, so there is no window between
// the two for the project to change hands.
func (s *ProjectsStore) UpdateProject(ctx context.Context, projectID, organizationID uuid.UUID, update ProjectUpdate) error {
	query := `
	UPDATE projects
	SET name = COALESCE($3, name),
	    description = COALESCE($4, description),
	    timezone = COALESCE($5, timezone),
	    locale = COALESCE($6, locale),
	    text_opt_out_message = COALESCE($7, text_opt_out_message),
	    text_help_message = COALESCE($8, text_help_message)
	WHERE id = $1
	  AND organization_id = $2
	  AND deleted_at IS NULL`

	result, err := s.db.ExecContext(ctx, query, projectID, organizationID, update.Name, update.Description, update.Timezone, update.Locale, update.TextOptOutMessage, update.TextHelpMessage)
	if err != nil {
		return err
	}

	return affected(result)
}

// DeleteProject soft-deletes a project the given organization owns, and returns
// sql.ErrNoRows when it owns no such project.
func (s *ProjectsStore) DeleteProject(ctx context.Context, projectID, organizationID uuid.UUID) error {
	query := `
	UPDATE projects
	SET deleted_at = NOW()
	WHERE id = $1
	  AND organization_id = $2
	  AND deleted_at IS NULL`

	result, err := s.db.ExecContext(ctx, query, projectID, organizationID)
	if err != nil {
		return err
	}

	return affected(result)
}

// affected translates a statement that matched nothing into sql.ErrNoRows, so
// callers branch on a missing project the same way whether they read it or wrote
// it.
func affected(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
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
