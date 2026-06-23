package management

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store"
)

type ApiKeys []ApiKey

func (keys ApiKeys) OAPI() []oapi.ApiKey {
	results := make([]oapi.ApiKey, len(keys))
	for i, key := range keys {
		results[i] = key.OAPI()
	}
	return results
}

// ApiKey is the api_key flavour of an auth method, joining auth_methods with
// auth_method_api_keys. It powers the legacy /keys endpoints.
type ApiKey struct {
	ID           uuid.UUID `db:"id"`
	ProjectID    uuid.UUID `db:"project_id"`
	SecretPrefix string    `db:"secret_prefix"`
	Name         string    `db:"name"`
	Description  *string   `db:"description"`
	Role         string    `db:"role"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`

	// Plaintext holds the full secret and is populated only by CreateApiKey, so
	// the value is shown to the caller exactly once on creation.
	Plaintext string `db:"-"`
}

func (k *ApiKey) OAPI() oapi.ApiKey {
	// The full secret is returned only on creation (Plaintext); afterwards only
	// the display prefix is exposed.
	value := k.Plaintext
	if value == "" {
		value = k.SecretPrefix
	}

	result := oapi.ApiKey{
		Id:        k.ID,
		ProjectId: k.ProjectID,
		Value:     value,
		Name:      k.Name,
		Role:      oapi.ProjectRole(k.Role),
		CreatedAt: k.CreatedAt,
		UpdatedAt: k.UpdatedAt,
	}

	if k.Description != nil {
		result.Description = k.Description
	}

	return result
}

func NewApiKeysStore(db store.DB) *ApiKeysStore {
	return &ApiKeysStore{db: db}
}

type ApiKeysStore struct {
	db store.DB
}

// apiKeySelect is the shared projection joining an auth method to its api-key
// credential. Callers append their own WHERE clause.
const apiKeySelect = `
	SELECT m.id, m.project_id, k.secret_prefix, m.name, m.description, m.role, m.created_at, m.updated_at
	FROM auth_methods m
	JOIN auth_method_api_keys k ON k.auth_method_id = m.id`

func (s *ApiKeysStore) CreateApiKey(ctx context.Context, projectID uuid.UUID, name string, role string, description *string) (*ApiKey, error) {
	plaintext, prefix, hash, err := newSecret()
	if err != nil {
		return nil, err
	}

	// Insert the auth method and its api-key credential atomically via a
	// data-modifying CTE (store.DB exposes no transaction).
	id := uuid.New()
	const stmt = `
	WITH m AS (
		INSERT INTO auth_methods (id, project_id, type, name, description, role)
		VALUES ($1, $2, 'api_key', $3, $4, $5)
		RETURNING id
	)
	INSERT INTO auth_method_api_keys (auth_method_id, secret_hash, secret_prefix)
	SELECT id, $6, $7 FROM m`

	if _, err := s.db.ExecContext(ctx, stmt, id, projectID, name, description, role, hash, prefix); err != nil {
		return nil, err
	}

	apiKey, err := s.GetApiKey(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	apiKey.Plaintext = plaintext // shown to the caller exactly once
	return apiKey, nil
}

func (s *ApiKeysStore) GetApiKey(ctx context.Context, projectID, keyID uuid.UUID) (*ApiKey, error) {
	stmt := apiKeySelect + `
	WHERE m.id = $1 AND m.project_id = $2 AND m.type = 'api_key' AND m.deleted_at IS NULL`

	var apiKey ApiKey
	if err := s.db.GetContext(ctx, &apiKey, stmt, keyID, projectID); err != nil {
		return nil, err
	}
	return &apiKey, nil
}

func (s *ApiKeysStore) ListApiKeys(ctx context.Context, projectID uuid.UUID, pagination store.Pagination) (ApiKeys, int, error) {
	stmt := `
	SELECT m.id, m.project_id, k.secret_prefix, m.name, m.description, m.role, m.created_at, m.updated_at,
	       COUNT(*) OVER () AS total_count
	FROM auth_methods m
	JOIN auth_method_api_keys k ON k.auth_method_id = m.id
	WHERE m.project_id = $1 AND m.type = 'api_key' AND m.deleted_at IS NULL
	ORDER BY m.created_at DESC
	LIMIT $2 OFFSET $3`

	type result struct {
		ApiKey
		TotalCount int `db:"total_count"`
	}

	var results []result
	if err := s.db.SelectContext(ctx, &results, stmt, projectID, pagination.Limit, pagination.Offset); err != nil {
		return nil, 0, err
	}
	if len(results) == 0 {
		return []ApiKey{}, 0, nil
	}

	apiKeys := make([]ApiKey, len(results))
	for i, r := range results {
		apiKeys[i] = r.ApiKey
	}
	return apiKeys, results[0].TotalCount, nil
}

func (s *ApiKeysStore) UpdateApiKey(ctx context.Context, projectID, keyID uuid.UUID, name *string, role *string, description *string) error {
	stmt := `
	UPDATE auth_methods
	SET name        = COALESCE($1, name),
	    role        = COALESCE($2, role),
	    description = COALESCE($3, description)
	WHERE id = $4 AND project_id = $5 AND type = 'api_key' AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, name, role, description, keyID, projectID)
	return err
}

func (s *ApiKeysStore) DeleteApiKey(ctx context.Context, projectID, keyID uuid.UUID) error {
	stmt := `
	UPDATE auth_methods
	SET deleted_at = NOW()
	WHERE id = $1 AND project_id = $2 AND type = 'api_key' AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, keyID, projectID)
	return err
}
