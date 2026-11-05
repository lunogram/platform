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
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/jwks"
	"github.com/lunogram/platform/internal/sso"
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

// Deps are the collaborators a verifier may need beyond the configuration and
// the management store. They are supplied by the caller rather than built here
// because the deployment decides the SSRF policy on outbound calls and owns the
// Redis connection and the JWKS cache the rest of the process shares.
type Deps struct {
	Mgmt        *management.State
	Logger      *zap.Logger
	Provisioner Provisioner
	// Keys, Flows, Discovery and HTTPClient are only read by the oidc driver. A
	// deployment that does not configure it may leave them nil.
	Keys       *jwks.Cache
	Flows      *sso.FlowStore
	Discovery  *sso.Discovery
	HTTPClient *http.Client
	// SAMLFlows, Assertions and SAMLMetadata are only read by the saml driver,
	// and may be nil for the same reason.
	//
	// The flow store is a separate one from Flows rather than a shared store
	// holding both protocols' flows. They key on different values -- a state
	// parameter and a RelayState -- and keeping the keyspaces apart is what
	// stops a value issued for one protocol being redeemable in the other.
	SAMLFlows    *sso.SAMLFlowStore
	Assertions   *sso.AssertionReplayStore
	SAMLMetadata *sso.SAMLMetadata
	// BaseURL is the deployment's public URL, which the OpenID Connect
	// redirect_uri is derived from.
	BaseURL string
}

// New builds a verifier for one driver.
func New(driver string, cfg config.Auth, deps Deps) (auth.Verifier, error) {
	mgmt, logger := deps.Mgmt, deps.Logger

	switch driver {
	case BasicDriver:
		return NewBasic(mgmt, logger), nil
	case ClerkDriver:
		// Refuse to start on key material that cannot verify anything. Without a
		// JWKS the parse in [Clerk.Verify] has no key, so every login fails --
		// closed, but silently, and it presents as "Clerk login is broken"
		// rather than as a variable nobody set.
		keyFunc := cfg.JWKS.Unwrap()
		if keyFunc == nil {
			return nil, fmt.Errorf("%w: the clerk auth driver verifies sessions against the provider's JWKS; set AUTH_JWKS_URL to your Clerk instance's JWKS endpoint (https://<your-instance>/.well-known/jwks.json)", ErrMissingJWKS)
		}
		return NewClerk(cfg.Clerk, mgmt, logger, keyFunc, deps.Provisioner)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownDriver, driver)
	}
}

// Federated is the deployment's redirect-based logins, one set per protocol.
//
// They are held apart rather than merged into one list because the protocols
// have nothing in common past the identity they produce: they are started at
// different endpoints, completed by different methods, and proved against
// different key material. What they share -- turning a proved identity into a
// session -- is [auth.Exchanger]'s and is already shared.
type Federated struct {
	OIDC *OIDCSet
	SAML *SAMLSet
}

// Any reports whether the deployment offers any federated login at all.
func (f *Federated) Any() bool {
	return f != nil && (f.OIDC != nil || f.SAML != nil)
}

// Build constructs every verifier the deployment has configured, keyed by
// driver, and separately the federated providers.
//
// The federated providers come back on their own because they are not verifiers
// in the same sense: such a credential arrives as a browser navigation naming a
// provider, not as something to prove on the request in hand. Keeping them out
// of the map is also what makes the generic login callback answer 404 for
// `oidc` and `saml` without a special case asking it to.
//
// Several drivers may be enabled at once -- an organization migrating from a
// shared basic credential to per-admin passwords needs both live at the same
// time -- so this returns a set rather than the single verifier the platform
// used to allow. Each one lands on its own callback and all of them are offered
// by GET /api/auth/methods.
func Build(cfg config.Auth, deps Deps) (map[string]auth.Verifier, *Federated, error) {
	built := make(map[string]auth.Verifier, len(cfg.Drivers))
	federated := &Federated{}
	seen := make(map[string]bool, len(cfg.Drivers))

	for _, driver := range cfg.Drivers {
		driver = strings.ToLower(strings.TrimSpace(driver))
		if driver == "" || seen[driver] {
			continue
		}
		seen[driver] = true

		switch driver {
		case OIDCDriver:
			set, err := NewOIDCSet(cfg.OIDC, OIDCOptions{
				Flows:      deps.Flows,
				Discovery:  deps.Discovery,
				Keys:       deps.Keys,
				BaseURL:    deps.BaseURL,
				HTTPClient: deps.HTTPClient,
				Logger:     deps.Logger.Named("oidc"),
			})
			if err != nil {
				return nil, nil, err
			}
			federated.OIDC = set

		case SAMLDriver:
			set, err := NewSAMLSet(cfg.SAML, SAMLOptions{
				Flows:      deps.SAMLFlows,
				Assertions: deps.Assertions,
				Metadata:   deps.SAMLMetadata,
				BaseURL:    deps.BaseURL,
				Logger:     deps.Logger.Named("saml"),
			})
			if err != nil {
				return nil, nil, err
			}
			federated.SAML = set

		default:
			verifier, err := New(driver, cfg, deps)
			if err != nil {
				return nil, nil, err
			}
			built[driver] = verifier
		}
	}

	return built, federated, nil
}
