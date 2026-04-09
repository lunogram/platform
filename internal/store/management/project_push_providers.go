package management

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/store"
)

// Platform constants for push providers.
const (
	PlatformIOS     = "ios"
	PlatformAndroid = "android"
	PlatformWeb     = "web"
)

type ProjectPushProviders []ProjectPushProvider

func (p ProjectPushProviders) OAPI() []oapi.ProjectPushProvider {
	result := make([]oapi.ProjectPushProvider, len(p))
	for i, pp := range p {
		result[i] = pp.OAPI()
	}
	return result
}

type ProjectPushProvider struct {
	ID         uuid.UUID `db:"id"`
	ProjectID  uuid.UUID `db:"project_id"`
	ProviderID uuid.UUID `db:"provider_id"`
	Platform   string    `db:"platform"`
	CreatedAt  time.Time `db:"created_at"`
	UpdatedAt  time.Time `db:"updated_at"`
}

func (p ProjectPushProvider) OAPI() oapi.ProjectPushProvider {
	return oapi.ProjectPushProvider{
		Id:         p.ID,
		ProjectId:  p.ProjectID,
		ProviderId: p.ProviderID,
		Platform:   oapi.ProjectPushProviderPlatform(p.Platform),
		CreatedAt:  p.CreatedAt,
		UpdatedAt:  p.UpdatedAt,
	}
}

func NewProjectPushProvidersStore(db store.DB) *ProjectPushProvidersStore {
	return &ProjectPushProvidersStore{db: db}
}

type ProjectPushProvidersStore struct {
	db store.DB
}

// UpsertProjectPushProvider creates or updates the push provider for a given project+platform.
// Returns the full row after the upsert.
func (s *ProjectPushProvidersStore) UpsertProjectPushProvider(ctx context.Context, pp ProjectPushProvider) (*ProjectPushProvider, error) {
	stmt := `
	INSERT INTO project_push_providers (project_id, provider_id, platform)
	VALUES ($1, $2, $3)
	ON CONFLICT (project_id, platform)
	DO UPDATE SET provider_id = EXCLUDED.provider_id
	RETURNING id, project_id, provider_id, platform, created_at, updated_at`

	var result ProjectPushProvider
	err := s.db.GetContext(ctx, &result, stmt, pp.ProjectID, pp.ProviderID, pp.Platform)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return nil, problem.ErrBadRequest(problem.Describe("provider not found"))
		}
		return nil, err
	}

	return &result, nil
}

// ListProjectPushProviders returns all push provider mappings for a project.
func (s *ProjectPushProvidersStore) ListProjectPushProviders(ctx context.Context, projectID uuid.UUID) (ProjectPushProviders, error) {
	query := `
	SELECT id, project_id, provider_id, platform, created_at, updated_at
	FROM project_push_providers
	WHERE project_id = $1
	ORDER BY platform`

	var result ProjectPushProviders
	err := s.db.SelectContext(ctx, &result, query, projectID)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetProjectPushProvider returns the push provider for a specific project+platform.
func (s *ProjectPushProvidersStore) GetProjectPushProvider(ctx context.Context, projectID uuid.UUID, platform string) (*ProjectPushProvider, error) {
	query := `
	SELECT id, project_id, provider_id, platform, created_at, updated_at
	FROM project_push_providers
	WHERE project_id = $1
	AND platform = $2`

	var pp ProjectPushProvider
	err := s.db.GetContext(ctx, &pp, query, projectID, platform)
	if err != nil {
		return nil, err
	}

	return &pp, nil
}

// DeleteProjectPushProvider removes the push provider mapping for a project+platform.
func (s *ProjectPushProvidersStore) DeleteProjectPushProvider(ctx context.Context, projectID uuid.UUID, platform string) error {
	query := `
	DELETE FROM project_push_providers
	WHERE project_id = $1
	AND platform = $2`

	_, err := s.db.ExecContext(ctx, query, projectID, platform)
	return err
}
