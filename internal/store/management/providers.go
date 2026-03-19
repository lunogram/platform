package management

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
)

// Channels is a []string that serialises to/from a JSONB array column.
type Channels []string

func (c *Channels) Scan(src any) error {
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, c)
	case string:
		return json.Unmarshal([]byte(v), c)
	case nil:
		*c = nil
		return nil
	default:
		return fmt.Errorf("cannot scan %T into Channels", src)
	}
}

func (c Channels) Value() (driver.Value, error) {
	if c == nil {
		return nil, nil
	}
	return json.Marshal(c)
}

type Providers []Provider

func (p Providers) OAPI() []oapi.Provider {
	result := make([]oapi.Provider, len(p))
	for i, provider := range p {
		result[i] = provider.OAPI()
	}
	return result
}

type Provider struct {
	ID        uuid.UUID       `db:"id"`
	ProjectID uuid.UUID       `db:"project_id"`
	Module    string          `db:"module"`
	Channels  Channels        `db:"channels"`
	Data      json.RawMessage `db:"data"`
	LinkWrap  bool            `db:"link_wrap"`
	Name      string          `db:"name"`
	CreatedAt time.Time       `db:"created_at"`
	UpdatedAt time.Time       `db:"updated_at"`
}

func (provider Provider) OAPI() oapi.Provider {
	channels := make([]oapi.Channel, len(provider.Channels))
	for i, ch := range provider.Channels {
		channels[i] = oapi.Channel(ch)
	}

	result := oapi.Provider{
		Id:        provider.ID,
		Data:      &provider.Data,
		Channels:  channels,
		LinkWrap:  &provider.LinkWrap,
		Name:      provider.Name,
		ProjectId: provider.ProjectID,
		Module:    provider.Module,
		CreatedAt: provider.CreatedAt,
		UpdatedAt: provider.UpdatedAt,
	}

	return result
}

func NewProvidersStore(db store.DB) *ProvidersStore {
	return &ProvidersStore{db: db}
}

type ProvidersStore struct {
	db store.DB
}

func (s *ProvidersStore) GetProvider(ctx context.Context, id uuid.UUID) (*Provider, error) {
	query := `
	SELECT id, project_id, module, channels, data, link_wrap, created_at, updated_at, name
	FROM providers
	WHERE id = $1`

	var provider Provider
	err := s.db.GetContext(ctx, &provider, query, id)
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
	INSERT INTO providers (project_id, module, channels, data, name, link_wrap)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		provider.ProjectID,
		provider.Module,
		provider.Channels,
		provider.Data,
		provider.Name,
		provider.LinkWrap,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *ProvidersStore) ListProviders(ctx context.Context, projectID uuid.UUID, pagination store.Pagination) (Providers, int, error) {
	query := `
	SELECT id, project_id, module, channels, data, link_wrap, name, created_at, updated_at,
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

func (s *ProvidersStore) GetProviderByProject(ctx context.Context, projectID, providerID uuid.UUID) (*Provider, error) {
	query := `
	SELECT id, project_id, module, channels, data, link_wrap, name, created_at, updated_at
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
	Name     *string
	Data     *json.RawMessage
	LinkWrap *bool
}

func (s *ProvidersStore) UpdateProvider(ctx context.Context, projectID, providerID uuid.UUID, update ProviderUpdate) error {
	query := `
	UPDATE providers
	SET
		name = COALESCE($1, name),
		data = COALESCE($2, data),
		link_wrap = COALESCE($3, link_wrap),
		updated_at = NOW()
	WHERE project_id = $4
	AND id = $5
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, query, update.Name, update.Data, update.LinkWrap, projectID, providerID)
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
