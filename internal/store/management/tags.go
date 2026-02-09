package management

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
)

type Tags []Tag

func (tags Tags) OAPI() []oapi.Tag {
	result := make([]oapi.Tag, len(tags))
	for index, tag := range tags {
		result[index] = tag.OAPI()
	}
	return result
}

type Tag struct {
	ID        uuid.UUID  `db:"id"`
	ProjectID uuid.UUID  `db:"project_id"`
	Name      string     `db:"name"`
	CreatedAt time.Time  `db:"created_at"`
	UpdatedAt time.Time  `db:"updated_at"`
	DeletedAt *time.Time `db:"deleted_at"`
}

func (tag Tag) OAPI() oapi.Tag {
	return oapi.Tag{
		Id:        tag.ID,
		ProjectId: tag.ProjectID,
		Name:      tag.Name,
		CreatedAt: tag.CreatedAt,
		UpdatedAt: tag.UpdatedAt,
	}
}

func NewTagsStore(db store.DB) *TagsStore {
	return &TagsStore{
		db: db,
	}
}

type TagsStore struct {
	db store.DB
}

func (s *TagsStore) CreateTag(ctx context.Context, projectID uuid.UUID, name string) (uuid.UUID, error) {
	stmt := `
	INSERT INTO tags (project_id, name)
	VALUES ($1, $2)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, projectID, name)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *TagsStore) ListTags(ctx context.Context, projectID uuid.UUID, pagination store.Pagination, search string) (Tags, int, error) {
	query := `
	SELECT id, project_id, name, created_at, updated_at, deleted_at,
		COUNT(*) OVER () AS total_count
	FROM tags
	WHERE project_id = $1
	AND deleted_at IS NULL
	AND ($2 = '' OR name ILIKE '%' || $2 || '%')
	ORDER BY name ASC
	LIMIT $3 OFFSET $4`

	var results []struct {
		Tag
		TotalCount int `db:"total_count"`
	}
	err := s.db.SelectContext(ctx, &results, query, projectID, search, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	tags := make(Tags, len(results))
	total := 0

	for i, r := range results {
		tags[i] = r.Tag
		if i == 0 {
			total = r.TotalCount
		}
	}

	return tags, total, nil
}

func (s *TagsStore) GetTag(ctx context.Context, projectID, tagID uuid.UUID) (*Tag, error) {
	query := `
	SELECT id, project_id, name, created_at, updated_at, deleted_at
	FROM tags
	WHERE project_id = $1
	AND id = $2
	AND deleted_at IS NULL`

	var tag Tag
	err := s.db.GetContext(ctx, &tag, query, projectID, tagID)
	if err != nil {
		return nil, err
	}

	return &tag, nil
}

func (s *TagsStore) UpdateTag(ctx context.Context, projectID, tagID uuid.UUID, name string) error {
	stmt := `
	UPDATE tags
	SET name = $1
	WHERE project_id = $2
	AND id = $3
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, name, projectID, tagID)
	return err
}

func (s *TagsStore) DeleteTag(ctx context.Context, projectID, tagID uuid.UUID) error {
	stmt := `
	UPDATE tags
	SET deleted_at = NOW()
	WHERE project_id = $1
	AND id = $2
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, projectID, tagID)
	return err
}
