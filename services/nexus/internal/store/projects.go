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
	CreatedAt         time.Time      `db:"created_at"`
	UpdatedAt         time.Time      `db:"updated_at"`
	DeletedAt         *time.Time     `db:"deleted_at"`
	Timezone          string         `db:"timezone"`
	TextOptOutMessage *string        `db:"text_opt_out_message"`
	LinkWrapEmail     bool           `db:"link_wrap_email"`
	TextHelpMessage   *string        `db:"text_help_message"`
	LinkWrapPush      bool           `db:"link_wrap_push"`
	Tools             pq.StringArray `db:"tools"`
	Locale            string         `db:"locale"`
}

func NewProjectsStore(db DB) *ProjectsStore {
	return &ProjectsStore{db: db}
}

type ProjectsStore struct {
	db DB
}

func (s *ProjectsStore) GetProject(ctx context.Context, id uuid.UUID) (*Project, error) {
	query := `
	SELECT * FROM projects
	WHERE id = $1
	AND deleted_at IS NULL`

	var project Project
	err := s.db.GetContext(ctx, &project, query, id)
	if err != nil {
		return nil, err
	}

	return &project, nil
}
