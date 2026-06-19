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
	SELECT m.id, p.organization_id, m.project_id, m.name, m.description, m.created_at, m.updated_at, m.role
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

// TrustedIssuerAuthMethod is a trusted_issuer auth method resolved for
// authenticating an external JWT: the RBAC identity (ID/org/project/role) plus
// the verification config.
type TrustedIssuerAuthMethod struct {
	ID             uuid.UUID    `db:"id"`
	OrganizationID uuid.UUID    `db:"organization_id"`
	ProjectID      uuid.UUID    `db:"project_id"`
	Role           string       `db:"role"`
	SubjectScope   SubjectScope `db:"subject_scope"`
	JWKSURL        *string      `db:"jwks_url"`
	PublicCert     *string      `db:"public_cert"`
	Issuer         string       `db:"issuer"`
	Audience       *string      `db:"audience"`
	SubjectClaim   string       `db:"subject_claim"`
}

// GetTrustedIssuerByIssuer resolves the trusted_issuer auth method registered
// for the given JWT `iss`. It is used to pick the verification keys and expected
// claims for an incoming external token.
func (s *AuthStore) GetTrustedIssuerByIssuer(issuer string) (*TrustedIssuerAuthMethod, error) {
	query := `
	SELECT m.id, p.organization_id, m.project_id, m.role, m.subject_scope,
	       t.jwks_url, t.public_cert, t.issuer, t.audience, t.subject_claim
	FROM auth_method_trusted_issuers t
	JOIN auth_methods m ON m.id = t.auth_method_id
	JOIN projects p ON p.id = m.project_id
	WHERE t.issuer = $1 AND m.deleted_at IS NULL`

	result := TrustedIssuerAuthMethod{}
	if err := s.db.Get(&result, query, issuer); err != nil {
		return nil, err
	}

	return &result, nil
}
