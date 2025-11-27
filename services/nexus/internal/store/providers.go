package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/oapi"
)

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

func (provider *Provider) OAPI() *oapi.Provider {
	if provider == nil {
		return nil
	}

	result := oapi.Provider{
		Id:        provider.ID,
		Data:      &provider.Data,
		Group:     oapi.ProviderGroup(provider.Group),
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

	return &result
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
