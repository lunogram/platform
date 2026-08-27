// Package verifiers holds the credential verifiers behind the console login
// callback. A verifier proves a credential against its upstream and returns an
// [auth.VerifiedIdentity]; everything that happens afterwards -- resolving or
// creating the admin, provisioning membership, recording a session, minting a
// token, setting a cookie -- belongs to [auth.Exchanger].
//
// That split is the point of the package. It replaces the previous "provider"
// abstraction, in which each driver independently found-or-created admins,
// created organizations, wrote RBAC tuples and set cookies, so the security
// invariants of a login had as many implementations as there were drivers.
package verifiers

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

var (
	ErrNoSession          = errors.New("no session token provided")
	ErrInvalidToken       = errors.New("invalid authentication token")
	ErrInvalidEmail       = errors.New("user has no email address")
	ErrWebhookDenied      = errors.New("webhook signature verification failed")
	ErrUnknownDriver      = errors.New("unknown auth driver")
	ErrMissingCredentials = errors.New("email and password are required")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrMissingJWKS        = errors.New("missing AUTH_JWKS_URL")
)

// Provisioner resolves a verified identity to an admin without minting a
// session. Upstream webhooks use it to mirror a user we have not seen log in
// yet, going through the same resolution order as a real login rather than a
// second, divergent create path. [auth.Exchanger] implements it.
type Provisioner interface {
	Provision(ctx context.Context, identity *auth.VerifiedIdentity) (uuid.UUID, error)
}

// New builds the verifier for the configured driver.
func New(cfg config.Auth, mgmt *management.State, logger *zap.Logger, provisioner Provisioner) (auth.Verifier, error) {
	switch cfg.Driver {
	case "basic":
		return NewBasic(cfg.Basic), nil
	case "clerk":
		// Refuse to start on key material that cannot verify anything. Without a
		// JWKS the parse in [Clerk.Verify] has no key, so every login fails --
		// closed, but silently, and it presents as "Clerk login is broken"
		// rather than as a variable nobody set.
		keyFunc := cfg.JWKS.Unwrap()
		if keyFunc == nil {
			return nil, fmt.Errorf("%w: the clerk auth driver verifies sessions against the provider's JWKS; set AUTH_JWKS_URL to your Clerk instance's JWKS endpoint (https://<your-instance>/.well-known/jwks.json)", ErrMissingJWKS)
		}
		return NewClerk(cfg.Clerk, mgmt, logger, keyFunc, provisioner)
	default:
		return nil, ErrUnknownDriver
	}
}
