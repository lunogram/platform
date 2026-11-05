package v1

import (
	"errors"
	"net/http"

	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/http/auth/verifiers"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	httpjson "github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/sso"
	"go.uber.org/zap"
)

// StartSAMLLogin sends the browser to the deployment's identity provider.
//
// The binding cookie set here is what ties the authentication request to this
// browser. Unlike the OpenID Connect one it is SameSite=None, because the
// browser comes back by cross-site form POST and a Lax cookie is not sent on
// one; see [auth.SetSAMLBindingCookie].
func (c *AuthController) StartSAMLLogin(w http.ResponseWriter, r *http.Request, provider string, params oapi.StartSAMLLoginParams) {
	ctx := r.Context()

	idp := c.federated.SAML.Provider(provider)
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
		// The provider is unreachable or its metadata no longer passes, which
		// is this deployment's problem rather than the caller's.
		c.logger.Error("failed to start a saml login",
			zap.String("provider", provider), zap.Error(err))
		c.redirectToLogin(w, r, "failed")
		return
	}

	auth.SetSAMLBindingCookie(w, authorization.Binding, sso.FlowTTL)

	if authorization.URL != "" {
		http.Redirect(w, r, authorization.URL, http.StatusFound)
		return
	}

	// The provider advertises only the HTTP-POST binding, so the request goes
	// out as a self-submitting form rather than a redirect. The document is
	// built by this deployment from its own AuthnRequest; nothing the provider
	// sent is echoed into it.
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	// The form carries an inline script that submits it, which is the whole
	// point of the binding, so this document gets a policy that permits exactly
	// that and nothing else.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; form-action *; script-src 'unsafe-inline'")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(authorization.PostForm); err != nil {
		c.logger.Warn("failed to write the saml authentication request form", zap.Error(err))
	}
}

// CompleteSAMLLogin is the assertion consumer service: it finishes a login and
// lands the browser back in the console.
//
// It answers with a redirect rather than a status, because what arrives here is
// a form POST from a navigation and the person on the other end needs a page
// either way. A failure carries a short, non-specific reason in the query
// string: enough for the login view to say something true, never enough to
// describe the deployment's identity provider to a stranger.
func (c *AuthController) CompleteSAMLLogin(w http.ResponseWriter, r *http.Request, provider string) {
	ctx := r.Context()

	idp := c.federated.SAML.Provider(provider)
	if idp == nil {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("single sign-on is not available on this deployment")))
		return
	}

	identity, redirect, err := idp.Complete(ctx, r)
	if err != nil {
		c.logger.Warn("saml login failed",
			zap.String("provider", provider), zap.Error(err))
		c.redirectToLogin(w, r, samlFailureReason(err))
		return
	}

	if _, err := c.exchanger.Exchange(ctx, w, r, identity); err != nil {
		c.logger.Error("session exchange failed after a saml login",
			zap.String("provider", provider), zap.Error(err))
		c.redirectToLogin(w, r, "exchange")
		return
	}

	http.Redirect(w, r, redirect, http.StatusFound)
}

// ListSAMLProviders names the providers the login page may offer.
//
// It is readable by anybody, which is the same disclosure the login page
// already makes by offering the buttons: these are the deployment's own
// providers, and there is no other tenant whose existence could leak.
func (c *AuthController) ListSAMLProviders(w http.ResponseWriter, r *http.Request) {
	if c.federated.SAML == nil {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("single sign-on is not available on this deployment")))
		return
	}

	declared := c.federated.SAML.All()
	providers := make([]oapi.SAMLProvider, 0, len(declared))
	for _, provider := range declared {
		providers = append(providers, oapi.SAMLProvider{Id: provider.ID(), Name: provider.Name()})
	}
	httpjson.Write(w, http.StatusOK, providers)
}

// GetSAMLMetadata publishes this deployment's own service provider metadata, so
// an operator can hand their identity provider a URL rather than copy an entity
// id, an assertion consumer service URL and a certificate between two screens.
//
// It carries no secret: the entity id and the ACS URL are public by
// construction and the certificate is the public half of the key pair.
func (c *AuthController) GetSAMLMetadata(w http.ResponseWriter, r *http.Request, provider string) {
	idp := c.federated.SAML.Provider(provider)
	if idp == nil {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("single sign-on is not available on this deployment")))
		return
	}

	document, err := idp.Metadata(r.Context())
	if err != nil {
		c.logger.Error("failed to build saml service provider metadata",
			zap.String("provider", provider), zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("the metadata could not be built")))
		return
	}

	w.Header().Set("Content-Type", "application/samlmetadata+xml")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(document); err != nil {
		c.logger.Warn("failed to write saml service provider metadata", zap.Error(err))
	}
}

// samlFailureReason maps a login failure onto the handful of things the login
// view can usefully say. It shares the reason vocabulary with the OpenID
// Connect callback, so the console has one set of messages rather than one per
// protocol.
func samlFailureReason(err error) string {
	switch {
	case errors.Is(err, verifiers.ErrSAMLFlowInvalid):
		return "expired"
	case errors.Is(err, verifiers.ErrSAMLProviderDenied):
		return "denied"
	case errors.Is(err, verifiers.ErrSAMLDomainNotAllowed):
		return "domain"
	case errors.Is(err, verifiers.ErrSAMLTransientNameID):
		return "transient"
	case errors.Is(err, verifiers.ErrInvalidEmail):
		return "email"
	default:
		return "failed"
	}
}
