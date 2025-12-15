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
	Type         string          `db:"type"`
	Group        string          `db:"group"`
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
		Group:     oapi.Channel(provider.Group),
		IsDefault: provider.IsDefault,
		Name:      provider.Name,
		ProjectId: provider.ProjectID,
		Type:      provider.Type,
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
	SELECT id, project_id, type, "group", data, is_default, rate_limit, rate_interval, created_at, updated_at, name
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
	SELECT id, project_id, type, "group", data, is_default, rate_limit, rate_interval, created_at, updated_at, name
	FROM providers
	WHERE project_id = $1
	AND "group" = $2
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
	INSERT INTO providers (project_id, type, "group", data, name, is_default, rate_limit, rate_interval)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		provider.ProjectID,
		provider.Type,
		provider.Group,
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
