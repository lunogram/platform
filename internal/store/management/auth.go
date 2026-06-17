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
	SecretHash     *string   `db:"secret_hash"`
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

// GetAPIKeyBySecret looks up an API key by the presented plaintext secret. The
// secret is matched by its SHA-256 hash; the plaintext is never stored.
func (s *AuthStore) GetAPIKeyBySecret(key string) (*APIKey, error) {
	query := `
	SELECT k.id, p.organization_id, k.project_id, k.secret_hash, k.scope, k.name, k.description, k.created_at, k.updated_at, k.role
	FROM project_api_keys k
	JOIN projects p ON p.id = k.project_id
	WHERE k.secret_hash = $1`

	result := APIKey{}
	err := s.db.Get(&result, query, hashSecret(key))
	if err != nil {
		return nil, err
	}

	return &result, nil
}
