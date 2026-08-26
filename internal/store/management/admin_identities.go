package management

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
)

// LegacyExternalIDIssuer is the sentinel issuer the admins.external_id column
// was backfilled under. Identities carrying it are adopted (rewritten in place
// to the real issuer) the first time their owner logs in; see the exchange's
// legacy adoption step. When no rows carry it any more the transitional code
// paths can be deleted.
const LegacyExternalIDIssuer = "urn:lunogram:legacy-external-id"

// Identity providers. The set is mirrored by a CHECK constraint on
// admin_identities.provider.
const (
	IdentityProviderPassword = "password"
	IdentityProviderClerk    = "clerk"
	IdentityProviderOIDC     = "oidc"
	IdentityProviderSAML     = "saml"
)

// NewAdminIdentitiesStore builds the identity store. sessions may be nil, in
// which case unlinking an identity does not end the sessions it established.
func NewAdminIdentitiesStore(db store.DB, sessions *AdminSessionsStore) *AdminIdentitiesStore {
	return &AdminIdentitiesStore{db: db, sessions: sessions}
}

type AdminIdentitiesStore struct {
	db       store.DB
	sessions *AdminSessionsStore
}

// AdminIdentity is one upstream identity an admin authenticates with. An admin
// may hold several; the identity is keyed by (issuer, subject), never by
// provider, so per-organization SSO connections with colliding subject spaces
// stay distinguishable.
type AdminIdentity struct {
	ID      uuid.UUID `db:"id"`
	AdminID uuid.UUID `db:"admin_id"`
	// Provider is descriptive (which kind of upstream this is). It is not part
	// of the identity key.
	Provider string  `db:"provider"`
	Issuer   string  `db:"issuer"`
	Subject  string  `db:"subject"`
	Email    *string `db:"email"`
	// EmailVerified records whether the upstream asserted the address as
	// verified at the time the identity was written. It is a record, not an
	// authorization input: the exchange links by email only on a freshly
	// verified claim from the credential being presented.
	EmailVerified bool       `db:"email_verified"`
	SecretHash    *string    `db:"secret_hash"`
	LastLoginAt   *time.Time `db:"last_login_at"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
	DeletedAt     *time.Time `db:"deleted_at"`
}

const adminIdentityColumns = `id, admin_id, provider, issuer, subject, email, email_verified,
	secret_hash, last_login_at, created_at, updated_at, deleted_at`

// GetAdminIdentity resolves an identity by its (issuer, subject) key. Both are
// taken verbatim from a verified credential; subject is opaque and is never
// parsed or interpreted.
func (s *AdminIdentitiesStore) GetAdminIdentity(ctx context.Context, issuer, subject string) (*AdminIdentity, error) {
	stmt := `
	SELECT ` + adminIdentityColumns + `
	FROM admin_identities
	WHERE issuer = $1 AND subject = $2
	AND deleted_at IS NULL`

	var identity AdminIdentity
	if err := s.db.GetContext(ctx, &identity, stmt, issuer, subject); err != nil {
		return nil, err
	}
	return &identity, nil
}

// ListAdminIdentities returns an admin's live identities, oldest first.
func (s *AdminIdentitiesStore) ListAdminIdentities(ctx context.Context, adminID uuid.UUID) ([]AdminIdentity, error) {
	stmt := `
	SELECT ` + adminIdentityColumns + `
	FROM admin_identities
	WHERE admin_id = $1
	AND deleted_at IS NULL
	ORDER BY created_at ASC`

	var identities []AdminIdentity
	if err := s.db.SelectContext(ctx, &identities, stmt, adminID); err != nil {
		return nil, err
	}
	return identities, nil
}

// CreateAdminIdentity links a verified upstream identity to an admin. The
// (issuer, subject) uniqueness is enforced by a partial unique index, so a
// concurrent double-link surfaces as a constraint violation rather than two
// rows claiming the same upstream user.
func (s *AdminIdentitiesStore) CreateAdminIdentity(ctx context.Context, identity AdminIdentity) (uuid.UUID, error) {
	stmt := `
	INSERT INTO admin_identities (admin_id, provider, issuer, subject, email, email_verified, secret_hash)
	VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		identity.AdminID,
		identity.Provider,
		identity.Issuer,
		identity.Subject,
		identity.Email,
		identity.EmailVerified,
		identity.SecretHash,
	)
	if err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

// AdoptLegacyIdentity rewrites a sentinel-issuer identity to the real issuer and
// provider of the credential that just proved it. The row is rewritten in place
// rather than replaced so the identity id (and anything referencing it, such as
// existing sessions) survives the adoption.
//
// The WHERE clause pins the sentinel issuer: adoption may only ever move a row
// OFF the legacy issuer, never re-point an identity that already names a real
// one.
func (s *AdminIdentitiesStore) AdoptLegacyIdentity(ctx context.Context, id uuid.UUID, issuer, provider string) error {
	stmt := `
	UPDATE admin_identities
	SET issuer = $2, provider = $3
	WHERE id = $1 AND issuer = $4
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, id, issuer, provider, LegacyExternalIDIssuer)
	return err
}

// TouchAdminIdentity records a successful login against the identity. It also
// refreshes the recorded email so a renamed upstream account does not keep an
// indefinitely stale address on the identity row; the admin's own email is left
// alone, since that is a profile field the admin owns.
func (s *AdminIdentitiesStore) TouchAdminIdentity(ctx context.Context, id uuid.UUID, email string, emailVerified bool) error {
	stmt := `
	UPDATE admin_identities
	SET last_login_at = NOW(),
	    email = COALESCE(NULLIF($2, ''), email),
	    email_verified = $3
	WHERE id = $1
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, id, email, emailVerified)
	return err
}

// DeleteAdminIdentity soft-deletes an identity, unlinking the upstream account
// from the admin, and ends the sessions that identity established. The
// revocation is the point of the operation: unlinking a compromised upstream
// account that leaves its existing sessions alive would not have removed the
// access. The partial unique index ignores soft-deleted rows, so the same
// (issuer, subject) can later be linked again -- to this admin or another.
func (s *AdminIdentitiesStore) DeleteAdminIdentity(ctx context.Context, id uuid.UUID) error {
	stmt := `UPDATE admin_identities SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	if _, err := s.db.ExecContext(ctx, stmt, id); err != nil {
		return err
	}

	if s.sessions != nil {
		return s.sessions.RevokeAdminSessionsForIdentity(ctx, id)
	}
	return nil
}
