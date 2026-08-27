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

// PasswordDriver is the driver identifier local email/password logins are keyed
// on.
const PasswordDriver = "password"

// Password verifies an email and password against a hash this deployment holds.
//
// It proves a credential and stops. It does not create the admin it fails to
// find, does not send mail, does not decide whether an unverified account may
// sign in and does not touch the response -- registration, verification and
// session creation all live elsewhere, so every driver reaches an account by
// exactly the same route.
type Password struct {
	mgmt   *management.State
	logger *zap.Logger
}

func NewPassword(mgmt *management.State, logger *zap.Logger) *Password {
	return &Password{mgmt: mgmt, logger: logger}
}

func (p *Password) Driver() string { return PasswordDriver }

type passwordCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Verify proves the submitted credential against the stored hash.
//
// Every failure -- unknown address, contested address, no password on the
// account, wrong password -- returns the same [ErrInvalidCredentials], and an
// address with no password identity still pays for a full hash comparison. A
// caller must not be able to tell which accounts exist, and answering "no such
// account" in microseconds while a wrong password takes ~100ms says it just as
// loudly as a different status code would.
func (p *Password) Verify(ctx context.Context, r *http.Request) (*auth.VerifiedIdentity, error) {
	var credentials passwordCredentials
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		return nil, ErrMissingCredentials
	}

	email := strings.ToLower(strings.TrimSpace(credentials.Email))
	if email == "" || credentials.Password == "" {
		return nil, ErrMissingCredentials
	}

	identity, err := p.lookup(ctx, email)
	if err != nil {
		return nil, err
	}
	if identity == nil || identity.SecretHash == nil {
		password.VerifyDummy(credentials.Password)
		return nil, ErrInvalidCredentials
	}

	match, outdated, err := password.Verify(*identity.SecretHash, credentials.Password)
	if err != nil {
		// A hash the database holds but this build cannot parse is a data
		// problem, not a wrong password. It is logged as such and still answered
		// as a failed login, because there is no safe way to let the caller in.
		p.logger.Error("stored password hash could not be read",
			zap.String("admin_id", identity.AdminID.String()), zap.Error(err))
		return nil, ErrInvalidCredentials
	}
	if !match {
		return nil, ErrInvalidCredentials
	}

	if outdated {
		p.upgrade(ctx, identity, credentials.Password)
	}

	return &auth.VerifiedIdentity{
		Issuer:   management.PasswordIssuer,
		Subject:  identity.Subject,
		Provider: management.IdentityProviderPassword,
		Email:    email,
		// Taken from the stored flag, never asserted by the login: an account
		// that has not confirmed its address may sign in, but it stays
		// unverified, so the exchange will not let it link itself onto another
		// identity by email.
		EmailVerified: identity.EmailVerified,
	}, nil
}

// lookup resolves the password identity owning an address, or nil when there is
// none. A contested address (one the duplicate-email reconciliation quarantined)
// resolves to nobody, because nothing can say which account it names.
func (p *Password) lookup(ctx context.Context, email string) (*management.AdminIdentity, error) {
	admin, err := p.mgmt.ResolveAdminByEmail(ctx, email)
	switch {
	case errors.Is(err, sql.ErrNoRows), errors.Is(err, management.ErrAmbiguousEmail):
		return nil, nil
	case err != nil:
		return nil, err
	}

	identity, err := p.mgmt.GetPasswordIdentity(ctx, admin.ID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case err != nil:
		return nil, err
	}

	return identity, nil
}

// upgrade re-hashes a proven password under the current cost parameters.
//
// This is the one write a verifier makes, and it is a write to the credential it
// has just proved rather than to anything the login goes on to decide: no admin
// is found or created, no session is opened, nothing about the returned identity
// changes. Without it, raising the parameters would only ever protect accounts
// created afterwards, and the oldest, weakest hashes -- the ones a leak would
// crack first -- would stay weak forever.
//
// A failure is logged and dropped. Refusing a login because the maintenance
// write failed would turn a housekeeping problem into an outage.
func (p *Password) upgrade(ctx context.Context, identity *management.AdminIdentity, plain string) {
	rehashed, err := password.Hash(plain)
	if err != nil {
		p.logger.Error("failed to re-hash a password under the current parameters", zap.Error(err))
		return
	}

	if err := p.mgmt.SetAdminIdentitySecret(ctx, identity.ID, rehashed); err != nil {
		p.logger.Error("failed to store a re-hashed password", zap.Error(err))
		return
	}

	p.logger.Info("re-hashed a password under the current parameters",
		zap.String("admin_id", identity.AdminID.String()))
}
