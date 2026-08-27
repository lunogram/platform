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
	"strings"

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

// New builds a verifier for one driver.
func New(driver string, cfg config.Auth, mgmt *management.State, logger *zap.Logger, provisioner Provisioner) (auth.Verifier, error) {
	switch driver {
	case BasicDriver:
		return NewBasic(cfg.Basic), nil
	case PasswordDriver:
		return NewPassword(mgmt, logger), nil
	case ClerkDriver:
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
		return nil, fmt.Errorf("%w: %q", ErrUnknownDriver, driver)
	}
}

// Build constructs every verifier the deployment has configured, keyed by
// driver.
//
// Several drivers may be enabled at once -- an organization migrating from a
// shared basic credential to per-admin passwords needs both live at the same
// time -- so this returns a set rather than the single verifier the platform
// used to allow. Each one lands on its own callback and all of them are offered
// by GET /api/auth/methods.
func Build(cfg config.Auth, mgmt *management.State, logger *zap.Logger, provisioner Provisioner) (map[string]auth.Verifier, error) {
	built := make(map[string]auth.Verifier, len(cfg.Drivers))

	for _, driver := range cfg.Drivers {
		driver = strings.ToLower(strings.TrimSpace(driver))
		if driver == "" {
			continue
		}
		if _, duplicate := built[driver]; duplicate {
			continue
		}

		verifier, err := New(driver, cfg, mgmt, logger, provisioner)
		if err != nil {
			return nil, err
		}
		built[driver] = verifier
	}

	return built, nil
}
