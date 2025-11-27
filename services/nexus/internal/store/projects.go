package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
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
	CreatedAt         time.Time      `db:"created_at"`
	UpdatedAt         time.Time      `db:"updated_at"`
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
	SELECT id, organization_id, name, description, timezone, text_opt_out_message, link_wrap_email, text_help_message, link_wrap_push, tools, locale, created_at, updated_at
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
