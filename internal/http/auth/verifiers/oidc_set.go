package verifiers

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/lunogram/platform/internal/config"
)

// OIDCSet is the deployment's OpenID Connect providers, in the order the
// operator declared them.
//
// It is not an [auth.Verifier]. A federated credential arrives as a browser
// navigation carrying a provider in its path, which the generic "prove what is
// on this request" contract has no room for, so the login endpoints hold this
// directly instead.
type OIDCSet struct {
	ordered []*OIDC
	byID    map[string]*OIDC
}

// NewOIDCSet builds every configured provider, or refuses.
//
// Every provider is validated at boot rather than at its first login. A
// deployment offering four ways in should not discover on a Monday morning that
// one of them was never going to work.
func NewOIDCSet(cfg config.OIDCAuth, opts OIDCOptions) (*OIDCSet, error) {
	declared, err := cfg.Resolve()
	if err != nil {
		return nil, err
	}
	if len(declared) == 0 {
		return nil, ErrOIDCNotConfigured
	}

	set := &OIDCSet{byID: make(map[string]*OIDC, len(declared))}
	for index, settings := range declared {
		if err := validateProviderID(settings.ID, index); err != nil {
			return nil, err
		}
		if _, duplicate := set.byID[settings.ID]; duplicate {
			return nil, fmt.Errorf("two single sign-on providers share the id %q", settings.ID)
		}

		provider, err := NewOIDC(OIDCOptions{
			Config:     settings,
			Flows:      opts.Flows,
			Discovery:  opts.Discovery,
			Keys:       opts.Keys,
			BaseURL:    opts.BaseURL,
			HTTPClient: opts.HTTPClient,
			Logger:     opts.Logger,
		})
		if err != nil {
			return nil, fmt.Errorf("single sign-on provider %q: %w", settings.ID, err)
		}

		set.ordered = append(set.ordered, provider)
		set.byID[provider.ID()] = provider
	}
	return set, nil
}

// Provider returns the provider with this id, or nil. Every login endpoint
// answers 404 on nil, so an id that names nothing is indistinguishable from a
// deployment that offers no single sign-on at all.
func (s *OIDCSet) Provider(id string) *OIDC {
	if s == nil {
		return nil
	}
	return s.byID[id]
}

// Only returns the sole provider when there is exactly one, so a deployment
// that configured a single one keeps a login URL with nothing to choose in it.
func (s *OIDCSet) Only() *OIDC {
	if s == nil || len(s.ordered) != 1 {
		return nil
	}
	return s.ordered[0]
}

// All returns the providers in declaration order.
func (s *OIDCSet) All() []*OIDC {
	if s == nil {
		return nil
	}
	return slices.Clone(s.ordered)
}

// validateProviderID refuses an id that cannot be half of a URL, since that is
// where it is about to be used.
//
// Dot segments are named separately because PathEscape leaves them alone: a
// provider called ".." escapes to "..", and the callback URL an operator would
// then register resolves in the browser to a path this deployment does not
// serve. The login could never complete, and it would look like the provider's
// fault.
func validateProviderID(id string, index int) error {
	if id == "" {
		return fmt.Errorf("single sign-on provider %d has no id", index)
	}
	if id == "." || id == ".." {
		return fmt.Errorf("single sign-on provider id %q is a URL path segment a browser resolves away", id)
	}
	if id != url.PathEscape(id) || strings.Contains(id, "/") {
		return fmt.Errorf("single sign-on provider id %q is not usable in a URL path", id)
	}
	return nil
}
