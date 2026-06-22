package management

import (
	"context"
	"time"

	"github.com/google/uuid"
	iredis "github.com/lunogram/platform/internal/redis"
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

// NewAuthStore builds the auth lookup store. The caches may be nil (or wrap a
// nil client), in which case lookups read straight from the database.
func NewAuthStore(db store.DB, apiKeys *iredis.Cache[APIKey], issuers *iredis.Cache[TrustedIssuerAuthMethod]) *AuthStore {
	return &AuthStore{db: db, apiKeys: apiKeys, issuers: issuers}
}

type AuthStore struct {
	db      store.DB
	apiKeys *iredis.Cache[APIKey]
	issuers *iredis.Cache[TrustedIssuerAuthMethod]
}

// GetAPIKeyBySecret looks up an API-key auth method by the presented plaintext
// secret. The secret is matched by its SHA-256 hash; the plaintext is never
// stored. The result is cached by hash (read-through); auth-method writes
// invalidate it.
func (s *AuthStore) GetAPIKeyBySecret(ctx context.Context, key string) (*APIKey, error) {
	hash := hashSecret(key)
	result, err := s.apiKeys.GetOrLoad(ctx, hash, func(ctx context.Context) (APIKey, error) {
		const query = `
		SELECT m.id, p.organization_id, m.project_id, m.name, m.description, m.created_at, m.updated_at, m.role
		FROM auth_method_api_keys k
		JOIN auth_methods m ON m.id = k.auth_method_id
		JOIN projects p ON p.id = m.project_id
		WHERE k.secret_hash = $1 AND m.deleted_at IS NULL`
		var r APIKey
		if err := s.db.GetContext(ctx, &r, query, hash); err != nil {
			return APIKey{}, err
		}
		return r, nil
	})
	if err != nil {
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

// GetTrustedIssuer resolves the trusted_issuer auth method registered for the
// given JWT `iss` within projectID. Resolution is project-scoped so a
// self-asserted issuer can never be served to a different project. It is used to
// pick the verification keys and expected claims for an incoming external token.
// The result is cached by (projectID, issuer) (read-through); auth-method writes
// invalidate it.
func (s *AuthStore) GetTrustedIssuer(ctx context.Context, projectID uuid.UUID, issuer string) (*TrustedIssuerAuthMethod, error) {
	result, err := s.issuers.GetOrLoad(ctx, trustedIssuerCacheKey(projectID, issuer), func(ctx context.Context) (TrustedIssuerAuthMethod, error) {
		const query = `
		SELECT m.id, p.organization_id, m.project_id, m.role, m.subject_scope,
		       t.jwks_url, t.public_cert, t.issuer, t.audience, t.subject_claim
		FROM auth_method_trusted_issuers t
		JOIN auth_methods m ON m.id = t.auth_method_id
		JOIN projects p ON p.id = m.project_id
		WHERE m.project_id = $1 AND t.issuer = $2 AND m.deleted_at IS NULL`
		var r TrustedIssuerAuthMethod
		if err := s.db.GetContext(ctx, &r, query, projectID, issuer); err != nil {
			return TrustedIssuerAuthMethod{}, err
		}
		return r, nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// trustedIssuerCacheKey namespaces the issuer cache by project so a method
// registered for project A can never be served to project B from the cache.
func trustedIssuerCacheKey(projectID uuid.UUID, issuer string) string {
	return projectID.String() + ":" + issuer
}

// SessionAuthMethod is a session auth method resolved when verifying a minted
// session token: its RBAC identity (the scope the session confers) plus its
// project/org.
type SessionAuthMethod struct {
	ID             uuid.UUID `db:"id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	ProjectID      uuid.UUID `db:"project_id"`
	SubjectScope   string    `db:"subject_scope"`
	TTLSeconds     int       `db:"ttl_seconds"`
}

// GetSessionAuthMethod resolves a session auth method (the "policy") by id. It is
// used both when minting a session (to read its TTL) and when verifying a minted
// token (to scope the resulting actor). Soft-deleted methods are not returned.
func (s *AuthStore) GetSessionAuthMethod(methodID uuid.UUID) (*SessionAuthMethod, error) {
	query := `
	SELECT m.id, p.organization_id, m.project_id, m.subject_scope, sess.ttl_seconds
	FROM auth_method_sessions sess
	JOIN auth_methods m ON m.id = sess.auth_method_id
	JOIN projects p ON p.id = m.project_id
	WHERE m.id = $1 AND m.deleted_at IS NULL`

	result := SessionAuthMethod{}
	if err := s.db.Get(&result, query, methodID); err != nil {
		return nil, err
	}

	return &result, nil
}
