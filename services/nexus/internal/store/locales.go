package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/oapi"
)

type Locales []Locale

func (locales Locales) OAPI() []oapi.Locale {
	result := make([]oapi.Locale, len(locales))
	for index, locale := range locales {
		result[index] = locale.OAPI()
	}
	return result
}

type Locale struct {
	ID        uuid.UUID `db:"id"`
	ProjectID uuid.UUID `db:"project_id"`
	Key       string    `db:"key"`
	Label     string    `db:"label"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

func (locale Locale) OAPI() oapi.Locale {
	return oapi.Locale{
		Id:        locale.ID,
		ProjectId: locale.ProjectID,
		Key:       locale.Key,
		Label:     locale.Label,
		CreatedAt: locale.CreatedAt,
		UpdatedAt: locale.UpdatedAt,
	}
}

func NewLocalesStore(db DB) *LocalesStore {
	return &LocalesStore{
		db: db,
	}
}

type LocalesStore struct {
	db DB
}

func (s *LocalesStore) CreateLocale(ctx context.Context, locale Locale) (uuid.UUID, error) {
	stmt := `
	INSERT INTO locales (project_id, key, label)
	VALUES ($1, $2, $3)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, locale.ProjectID, locale.Key, locale.Label)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *LocalesStore) ListLocales(ctx context.Context, projectID uuid.UUID, pagination Pagination) (Locales, int, error) {
	query := `
	SELECT id, project_id, key, label, created_at, updated_at,
		COUNT(*) OVER () AS total_count
	FROM locales
	WHERE project_id = $1
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	var results []struct {
		Locale
		TotalCount int `db:"total_count"`
	}
	err := s.db.SelectContext(ctx, &results, query, projectID, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	locales := make(Locales, len(results))
	total := 0

	for i, r := range results {
		locales[i] = r.Locale
		if i == 0 {
			total = r.TotalCount
		}
	}

	return locales, total, nil
}

func (s *LocalesStore) GetLocale(ctx context.Context, projectID uuid.UUID, localeID string) (*Locale, error) {
	query := `
	SELECT id, project_id, key, label, created_at, updated_at
	FROM locales
	WHERE project_id = $1
	AND (id::text = $2 OR key = $2)`

	var locale Locale
	err := s.db.GetContext(ctx, &locale, query, projectID, localeID)
	if err != nil {
		return nil, err
	}

	return &locale, nil
}

func (s *LocalesStore) DeleteLocale(ctx context.Context, projectID, localeID uuid.UUID) error {
	query := `
	DELETE FROM locales
	WHERE project_id = $1
	AND id = $2`

	_, err := s.db.ExecContext(ctx, query, projectID, localeID)
	return err
}
