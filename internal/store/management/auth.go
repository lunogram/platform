package management

import (
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
)

type APIKey struct {
	ID             string    `db:"id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	ProjectID      uuid.UUID `db:"project_id"`
	Scope          *string   `db:"scope"`
	Name           string    `db:"name"`
	Description    *string   `db:"description"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
	Role           string    `db:"role"`
}

func NewAuthStore(db store.DB) *AuthStore {
	return &AuthStore{db: db}
}

type AuthStore struct {
	db store.DB
}

// GetAPIKeyBySecret looks up an API-key auth method by the presented plaintext
// secret. The secret is matched by its SHA-256 hash; the plaintext is never
// stored.
func (s *AuthStore) GetAPIKeyBySecret(key string) (*APIKey, error) {
	query := `
	SELECT m.id, p.organization_id, m.project_id, k.scope, m.name, m.description, m.created_at, m.updated_at, m.role
	FROM auth_method_api_keys k
	JOIN auth_methods m ON m.id = k.auth_method_id
	JOIN projects p ON p.id = m.project_id
	WHERE k.secret_hash = $1 AND m.deleted_at IS NULL`

	result := APIKey{}
	if err := s.db.Get(&result, query, hashSecret(key)); err != nil {
		return nil, err
	}

	return &result, nil
}
