package management

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

type ApiKey struct {
	ID          uuid.UUID `db:"id"`
	ProjectID   uuid.UUID `db:"project_id"`
	Value       string    `db:"value"`
	Scope       *string   `db:"scope"`
	Name        string    `db:"name"`
	Description *string   `db:"description"`
	Role        string    `db:"role"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (k *ApiKey) OAPI() oapi.ApiKey {
	result := oapi.ApiKey{
		Id:        k.ID,
		ProjectId: k.ProjectID,
		Value:     k.Value,
		Name:      k.Name,
		Role:      oapi.ProjectRole(k.Role),
		CreatedAt: k.CreatedAt,
		UpdatedAt: k.UpdatedAt,
	}

	if k.Scope != nil {
		result.Scope = oapi.ApiKeyScope(*k.Scope)
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

// generateKeyValue generates a cryptographically secure 32-byte random value encoded as hex
func generateKeyValue() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func (s *ApiKeysStore) CreateApiKey(ctx context.Context, projectID uuid.UUID, name string, scope string, role string, description *string) (*ApiKey, error) {
	keyValue, err := generateKeyValue()
	if err != nil {
		return nil, err
	}

	stmt := `
	INSERT INTO project_api_keys (project_id, value, scope, name, description, role)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id, project_id, value, scope, name, description, role, created_at, updated_at`

	var apiKey ApiKey
	err = s.db.GetContext(ctx, &apiKey, stmt, projectID, keyValue, scope, name, description, role)
	if err != nil {
		return nil, err
	}

	return &apiKey, nil
}

func (s *ApiKeysStore) GetApiKey(ctx context.Context, projectID, keyID uuid.UUID) (*ApiKey, error) {
	stmt := `
	SELECT id, project_id, value, scope, name, description, role, created_at, updated_at
	FROM project_api_keys
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL`

	var apiKey ApiKey
	err := s.db.GetContext(ctx, &apiKey, stmt, keyID, projectID)
	if err != nil {
		return nil, err
	}

	return &apiKey, nil
}

func (s *ApiKeysStore) ListApiKeys(ctx context.Context, projectID uuid.UUID, pagination store.Pagination) (ApiKeys, int, error) {
	query := `
	SELECT
		id,
		project_id,
		value,
		scope,
		name,
		description,
		role,
		created_at,
		updated_at,
		COUNT(*) OVER () AS total_count
	FROM project_api_keys
	WHERE project_id = $1 AND deleted_at IS NULL
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	type result struct {
		ApiKey
		TotalCount int `db:"total_count"`
	}

	var results []result
	err := s.db.SelectContext(ctx, &results, query, projectID, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	if len(results) == 0 {
		return []ApiKey{}, 0, nil
	}

	total := results[0].TotalCount
	apiKeys := make([]ApiKey, len(results))

	for index, r := range results {
		apiKeys[index] = r.ApiKey
	}

	return apiKeys, total, nil
}

func (s *ApiKeysStore) UpdateApiKey(ctx context.Context, projectID, keyID uuid.UUID, name *string, role *string, description *string) error {
	stmt := `
	UPDATE project_api_keys
	SET 
		name = COALESCE($1, name),
		role = COALESCE($2, role),
		description = COALESCE($3, description),
		updated_at = CURRENT_TIMESTAMP
	WHERE id = $4 AND project_id = $5 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, name, role, description, keyID, projectID)
	return err
}

func (s *ApiKeysStore) DeleteApiKey(ctx context.Context, projectID, keyID uuid.UUID) error {
	stmt := `
	UPDATE project_api_keys
	SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, keyID, projectID)
	return err
}
