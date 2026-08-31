package verifiers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/password"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

// BasicDriver is the driver identifier local email/password logins are keyed on.
const BasicDriver = "basic"

// Basic verifies an email and password against a hash this deployment holds.
//
// It used to be two drivers. "basic" compared the single pair in
// AUTH_BASIC_EMAIL / AUTH_BASIC_PASSWORD, and "password" verified a per-admin
// hash — the same kind of credential behind two verifiers, of which only one had
// the login throttle, the constant-time comparison against a dummy hash, and the
// re-hash on login. The configured pair is now seeded into an admin like any
// other (see [github.com/lunogram/platform/internal/http/auth.Seed]), which
// leaves one credential path and makes the quickstart account a real one: it
// appears in the admin list, holds its own RBAC tuples, and can change its own
// password. It also means the plaintext is needed once, at first boot, rather
// than living in the environment for as long as the deployment does.
//
// It proves a credential and stops. It does not create the admin it fails to
// find, does not send mail, does not decide whether an unverified account may
// sign in and does not touch the response — registration, verification and
// session creation all live elsewhere, so every driver reaches an account by
// exactly the same route.
type Basic struct {
	mgmt   *management.State
	logger *zap.Logger
}

func NewBasic(mgmt *management.State, logger *zap.Logger) *Basic {
	return &Basic{mgmt: mgmt, logger: logger}
}

func (b *Basic) Driver() string { return BasicDriver }

type basicCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Verify proves the submitted credential against the stored hash.
//
// Every failure — unknown address, contested address, no password on the
// account, wrong password — returns the same [ErrInvalidCredentials], and an
// address with no local identity still pays for a full hash comparison. A caller
// must not be able to tell which accounts exist, and answering "no such account"
// in microseconds while a wrong password takes ~100ms says it just as loudly as
// a different status code would.
func (b *Basic) Verify(ctx context.Context, r *http.Request) (*auth.VerifiedIdentity, error) {
	var credentials basicCredentials
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		return nil, ErrMissingCredentials
	}

	email := strings.ToLower(strings.TrimSpace(credentials.Email))
	if email == "" || credentials.Password == "" {
		return nil, ErrMissingCredentials
	}

	identity, err := b.lookup(ctx, email)
	if err != nil {
		return nil, err
	}
	if identity == nil || identity.SecretHash == nil {
		password.VerifyDummy(credentials.Password)
		return nil, ErrInvalidCredentials
	}

	match, err := password.Verify(*identity.SecretHash, credentials.Password)
	if err != nil {
		// A hash the database holds but this build cannot parse is a data
		// problem, not a wrong password. It is logged as such and still answered
		// as a failed login, because there is no safe way to let the caller in.
		b.logger.Error("stored password hash could not be read",
			zap.String("admin_id", identity.AdminID.String()), zap.Error(err))
		return nil, ErrInvalidCredentials
	}
	if !match {
		return nil, ErrInvalidCredentials
	}

	return &auth.VerifiedIdentity{
		Issuer:   management.LocalIssuer,
		Subject:  identity.Subject,
		Provider: management.IdentityProviderBasic,
		Email:    email,
		// Taken from the stored flag, never asserted by the login: an account
		// that has not confirmed its address may sign in, but it stays
		// unverified, so the exchange will not let it link itself onto another
		// identity by email.
		EmailVerified: identity.EmailVerified,
	}, nil
}

// lookup resolves the local identity owning an address, or nil when there is
// none. A contested address (one the duplicate-email reconciliation quarantined)
// resolves to nobody, because nothing can say which account it names.
func (b *Basic) lookup(ctx context.Context, email string) (*management.AdminIdentity, error) {
	admin, err := b.mgmt.ResolveAdminByEmail(ctx, email)
	switch {
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, management.ErrAmbiguousEmail):
		return nil, nil
	case err != nil:
		return nil, err
	}

	identity, err := b.mgmt.GetLocalIdentity(ctx, admin.ID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, err
	}

	return identity, nil
}
