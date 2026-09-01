package v1

import (
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/http/auth/verifiers"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	httpjson "github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/sso"
	"go.uber.org/zap"
)

// oidcProviderTimeout bounds a single call out to the identity provider --
// discovery, or the token exchange. A login is a person waiting at a redirect,
// so a provider that has stopped answering has to fail rather than hang.
const oidcProviderTimeout = 10 * time.Second

// StartOIDCLogin sends the browser to the deployment's identity provider.
//
// The binding cookie set here is what ties the authorization request to this
// browser. Without it the state parameter would be a bearer value: somebody
// could authenticate as themselves, stop before following the callback, and
// hand that URL to another person, whose browser would be given the attacker's
// session.
func (c *AuthController) StartOIDCLogin(w http.ResponseWriter, r *http.Request, provider string, params oapi.StartOIDCLoginParams) {
	ctx := r.Context()

	idp := c.federated.OIDC.Provider(provider)
	if idp == nil {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("single sign-on is not available on this deployment")))
		return
	}

	redirect := ""
	if params.R != nil {
		redirect = *params.R
	}

	authorization, err := idp.Authorize(ctx, r, redirect)
	if err != nil {
		// The provider is unreachable or its discovery document no longer
		// passes, which is this deployment's problem rather than the caller's.
		c.logger.Error("failed to start a federated login",
			zap.String("provider", provider), zap.Error(err))
		c.redirectToLogin(w, r, "failed")
		return
	}

	auth.SetOIDCBindingCookie(w, r, authorization.Binding, sso.FlowTTL)
	http.Redirect(w, r, authorization.URL, http.StatusFound)
}

// CompleteOIDCLogin finishes a federated login and lands the browser back in the
// console.
//
// It answers with a redirect rather than a status, because what arrives here is
// a navigation and the person on the other end needs a page either way. A
// failure carries a short, non-specific reason in the query string: enough for
// the login view to say something true, never enough to describe the
// deployment's identity provider to a stranger.
func (c *AuthController) CompleteOIDCLogin(w http.ResponseWriter, r *http.Request, provider string, _ oapi.CompleteOIDCLoginParams) {
	ctx := r.Context()

	idp := c.federated.OIDC.Provider(provider)
	if idp == nil {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("single sign-on is not available on this deployment")))
		return
	}

	identity, redirect, err := idp.Complete(ctx, r)
	if err != nil {
		c.logger.Warn("federated login failed",
			zap.String("provider", provider), zap.Error(err))
		c.redirectToLogin(w, r, oidcFailureReason(err))
		return
	}

	if _, err := c.exchanger.Exchange(ctx, w, r, identity); err != nil {
		c.logger.Error("session exchange failed after a federated login",
			zap.String("provider", provider), zap.Error(err))
		c.redirectToLogin(w, r, "exchange")
		return
	}

	http.Redirect(w, r, redirect, http.StatusFound)
}

// ListOIDCProviders names the providers the login page may offer.
//
// It is readable by anybody, which is the same disclosure as the login page
// already makes by offering the buttons: these are the deployment's own
// providers, and there is no other tenant whose existence could leak.
func (c *AuthController) ListOIDCProviders(w http.ResponseWriter, r *http.Request) {
	if c.federated.OIDC == nil {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("single sign-on is not available on this deployment")))
		return
	}

	providers := make([]oapi.OIDCProvider, 0, len(c.federated.OIDC.All()))
	for _, provider := range c.federated.OIDC.All() {
		providers = append(providers, oapi.OIDCProvider{Id: provider.ID(), Name: provider.Name()})
	}
	httpjson.Write(w, http.StatusOK, providers)
}

// redirectToLogin sends a failed federated login back to the login view with a
// coarse reason.
func (c *AuthController) redirectToLogin(w http.ResponseWriter, r *http.Request, reason string) {
	target := url.URL{Path: "/login", RawQuery: url.Values{"sso_error": {reason}}.Encode()}
	http.Redirect(w, r, target.RequestURI(), http.StatusFound)
}

// oidcFailureReason maps a login failure onto the handful of things the login
// view can usefully say. Anything unexpected is "failed".
func oidcFailureReason(err error) string {
	switch {
	case errors.Is(err, verifiers.ErrOIDCFlowInvalid):
		return "expired"
	case errors.Is(err, verifiers.ErrOIDCProviderDenied):
		return "denied"
	case errors.Is(err, verifiers.ErrOIDCDomainNotAllowed):
		return "domain"
	case errors.Is(err, verifiers.ErrInvalidEmail):
		return "email"
	default:
		return "failed"
	}
}
