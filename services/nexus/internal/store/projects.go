package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lunogram/platform/services/nexus/oapi"
)

type Project struct {
	ID                uuid.UUID      `db:"id"`
	OrganizationID    *uuid.UUID     `db:"organization_id"`
	Name              string         `db:"name"`
	Description       *string        `db:"description"`
	Timezone          string         `db:"timezone"`
	TextOptOutMessage *string        `db:"text_opt_out_message"`
	LinkWrapEmail     bool           `db:"link_wrap_email"`
	TextHelpMessage   *string        `db:"text_help_message"`
	LinkWrapPush      bool           `db:"link_wrap_push"`
	Tools             pq.StringArray `db:"tools"`
	Locale            string         `db:"locale"`
	HasProvider       bool           `db:"has_provider"`
	CreatedAt         time.Time      `db:"created_at"`
	UpdatedAt         time.Time      `db:"updated_at"`
}

func (p *Project) OAPI() oapi.Project {
	project := oapi.Project{
		Id:            p.ID,
		Name:          p.Name,
		Timezone:      p.Timezone,
		Locale:        p.Locale,
		LinkWrapEmail: &p.LinkWrapEmail,
		LinkWrapPush:  &p.LinkWrapPush,
		HasProvider:   &p.HasProvider,
		Role:          "admin", // NOTE: we hardcode this for now; this has to be refactored later when we are addressing RBAC, the role is right now checked in the controller
		CreatedAt:     p.CreatedAt,
		UpdatedAt:     p.UpdatedAt,
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

	if len(p.Tools) > 0 {
		tools := make([]string, len(p.Tools))
		copy(tools, p.Tools)
		project.Tools = &tools
	}

	return project
}

func NewProjectsStore(db DB) *ProjectsStore {
	return &ProjectsStore{db: db}
}

type ProjectsStore struct {
	db DB
}

func (s *ProjectsStore) CreateProject(ctx context.Context, project Project) (uuid.UUID, error) {
	stmt := `
	INSERT INTO projects (organization_id, name, description, timezone, text_opt_out_message, link_wrap_email, text_help_message, link_wrap_push, tools, locale)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, project.OrganizationID, project.Name, project.Description, project.Timezone, project.TextOptOutMessage, project.LinkWrapEmail, project.TextHelpMessage, project.LinkWrapPush, project.Tools, project.Locale)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *ProjectsStore) GetProject(ctx context.Context, id uuid.UUID) (*Project, error) {
	query := `
	SELECT id, organization_id, name, description, timezone, text_opt_out_message, link_wrap_email, text_help_message, link_wrap_push, tools, locale, created_at, updated_at,
	       EXISTS(
		       SELECT 1 FROM providers
		       WHERE providers.project_id = projects.id
		         AND providers.deleted_at IS NULL
	       ) AS has_provider
	FROM projects
	WHERE id = $1
	AND deleted_at IS NULL`

	var project Project
	err := s.db.GetContext(ctx, &project, query, id)
	if err != nil {
		return nil, err
	}

	return &project, nil
}

func (s *ProjectsStore) ListProjects(ctx context.Context, adminID, organizationID uuid.UUID, pagination Pagination, search string) ([]Project, int, error) {
	query := `
	SELECT DISTINCT p.id, p.organization_id, p.name, p.description, p.timezone, p.text_opt_out_message, 
	       p.link_wrap_email, p.text_help_message, p.link_wrap_push, p.tools, p.locale, 
	       p.created_at, p.updated_at,
	       EXISTS(
		       SELECT 1 FROM providers
		       WHERE providers.project_id = p.id
		         AND providers.deleted_at IS NULL
	       ) AS has_provider,
	       COUNT(*) OVER() AS total_count
	FROM projects p
	LEFT JOIN project_admins pa ON pa.project_id = p.id
	WHERE p.deleted_at IS NULL
	  AND (p.organization_id = $1 OR pa.admin_id = $2)
	  AND ($3 = '' OR p.name ILIKE '%' || $3 || '%')
	ORDER BY p.created_at DESC
	LIMIT $4 OFFSET $5`

	type result struct {
		Project
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, organizationID, adminID, search, pagination.Limit, pagination.Offset)
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
	LinkWrapEmail     *bool
	LinkWrapPush      *bool
	Tools             pq.StringArray
}

func (s *ProjectsStore) UpdateProject(ctx context.Context, projectID uuid.UUID, update ProjectUpdate) error {
	query := `
	UPDATE projects
	SET name = COALESCE($2, name),
	    description = COALESCE($3, description),
	    timezone = COALESCE($4, timezone),
	    locale = COALESCE($5, locale),
	    text_opt_out_message = COALESCE($6, text_opt_out_message),
	    text_help_message = COALESCE($7, text_help_message),
	    link_wrap_email = COALESCE($8, link_wrap_email),
	    link_wrap_push = COALESCE($9, link_wrap_push),
	    tools = COALESCE($10, tools)
	WHERE id = $1
	  AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, projectID, update.Name, update.Description, update.Timezone, update.Locale, update.TextOptOutMessage, update.TextHelpMessage, update.LinkWrapEmail, update.LinkWrapPush, update.Tools)
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
