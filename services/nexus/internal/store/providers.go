package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
)

type Providers []Provider

func (p Providers) OAPI() []oapi.Provider {
	result := make([]oapi.Provider, len(p))
	for i, provider := range p {
		result[i] = provider.OAPI()
	}
	return result
}

type Provider struct {
	ID           uuid.UUID       `db:"id"`
	ProjectID    uuid.UUID       `db:"project_id"`
	Module       string          `db:"module"`
	Channel      string          `db:"channel"`
	Data         json.RawMessage `db:"data"`
	IsDefault    bool            `db:"is_default"`
	RateLimit    *int32          `db:"rate_limit"`
	RateInterval *string         `db:"rate_interval"`
	Name         string          `db:"name"`
	CreatedAt    time.Time       `db:"created_at"`
	UpdatedAt    time.Time       `db:"updated_at"`
}

func (provider Provider) OAPI() oapi.Provider {
	result := oapi.Provider{
		Id:        provider.ID,
		Data:      &provider.Data,
		Channel:   oapi.Channel(provider.Channel),
		IsDefault: provider.IsDefault,
		Name:      provider.Name,
		ProjectId: provider.ProjectID,
		Module:    provider.Module,
		CreatedAt: provider.CreatedAt,
		UpdatedAt: provider.UpdatedAt,
	}

	if provider.RateLimit != nil {
		result.RateLimit = provider.RateLimit
	}

	if provider.RateInterval != nil {
		interval := oapi.ProviderRateInterval(*provider.RateInterval)
		result.RateInterval = &interval
	}

	return result
}

func NewProvidersStore(db DB) *ProvidersStore {
	return &ProvidersStore{db: db}
}

type ProvidersStore struct {
	db DB
}

func (s *ProvidersStore) GetProvider(ctx context.Context, id uuid.UUID) (*Provider, error) {
	query := `
	SELECT id, project_id, module, channel, data, is_default, rate_limit, rate_interval, created_at, updated_at, name
	FROM providers
	WHERE id = $1`

	var provider Provider
	err := s.db.GetContext(ctx, &provider, query, id)
	if err != nil {
		return nil, err
	}

	return &provider, nil
}

func (s *ProvidersStore) GetDefaultProviderChannel(ctx context.Context, projectID uuid.UUID, group string) (*Provider, error) {
	query := `
	SELECT id, project_id, module, channel, data, is_default, rate_limit, rate_interval, created_at, updated_at, name
	FROM providers
	WHERE project_id = $1
	AND channel = $2
	AND is_default = true
	LIMIT 1`

	var provider Provider
	err := s.db.GetContext(ctx, &provider, query, projectID, group)
	if err != nil {
		return nil, err
	}

	return &provider, nil
}

func (s *ProvidersStore) HasProvider(ctx context.Context, projectID uuid.UUID) (bool, error) {
	query := `
	SELECT EXISTS(
		SELECT 1 FROM providers
		WHERE project_id = $1
		  AND deleted_at IS NULL
	)`

	var exists bool
	err := s.db.GetContext(ctx, &exists, query, projectID)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (s *ProvidersStore) CreateProvider(ctx context.Context, provider Provider) (uuid.UUID, error) {
	stmt := `
	INSERT INTO providers (project_id, module, channel, data, name, is_default, rate_limit, rate_interval)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		provider.ProjectID,
		provider.Module,
		provider.Channel,
		provider.Data,
		provider.Name,
		provider.IsDefault,
		provider.RateLimit,
		provider.RateInterval,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *ProvidersStore) ListProviders(ctx context.Context, projectID uuid.UUID, pagination Pagination) (Providers, int, error) {
	query := `
	SELECT id, project_id, module, channel, data, is_default, rate_limit, rate_interval, name, created_at, updated_at,
		COUNT(*) OVER () AS total_count
	FROM providers
	WHERE project_id = $1
	AND deleted_at IS NULL
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	var results []struct {
		Provider
		TotalCount int `db:"total_count"`
	}
	err := s.db.SelectContext(ctx, &results, query, projectID, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	providers := make(Providers, len(results))
	total := 0

	for i, r := range results {
		providers[i] = r.Provider
		if i == 0 {
			total = r.TotalCount
		}
	}

	return providers, total, nil
}

func (s *ProvidersStore) ListAllProviders(ctx context.Context, projectID uuid.UUID) (Providers, error) {
	query := `
	SELECT id, project_id, module, channel, data, is_default, rate_limit, rate_interval, name, created_at, updated_at
	FROM providers
	WHERE project_id = $1
	AND deleted_at IS NULL
	ORDER BY created_at DESC`

	var providers Providers
	err := s.db.SelectContext(ctx, &providers, query, projectID)
	if err != nil {
		return nil, err
	}

	return providers, nil
}

func (s *ProvidersStore) GetProviderByProject(ctx context.Context, projectID, providerID uuid.UUID) (*Provider, error) {
	query := `
	SELECT id, project_id, module, channel, data, is_default, rate_limit, rate_interval, name, created_at, updated_at
	FROM providers
	WHERE project_id = $1
	AND id = $2
	AND deleted_at IS NULL`

	var provider Provider
	err := s.db.GetContext(ctx, &provider, query, projectID, providerID)
	if err != nil {
		return nil, err
	}

	return &provider, nil
}

type ProviderUpdate struct {
	Name         *string
	Data         *json.RawMessage
	IsDefault    *bool
	RateLimit    *int32
	RateInterval *string
}

func (s *ProvidersStore) UpdateProvider(ctx context.Context, projectID, providerID uuid.UUID, update ProviderUpdate) error {
	query := `
	UPDATE providers
	SET
		name = COALESCE($1, name),
		data = COALESCE($2, data),
		is_default = COALESCE($3, is_default),
		rate_limit = COALESCE($4, rate_limit),
		rate_interval = COALESCE($5, rate_interval),
		updated_at = NOW()
	WHERE project_id = $6
	AND id = $7
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, update.Name, update.Data, update.IsDefault, update.RateLimit, update.RateInterval, projectID, providerID)
	return err
}

func (s *ProvidersStore) DeleteProvider(ctx context.Context, projectID, providerID uuid.UUID) error {
	query := `
	UPDATE providers
	SET deleted_at = NOW()
	WHERE project_id = $1
	AND id = $2
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, projectID, providerID)
	return err
}
