package verifiers

import (
	"fmt"
	"slices"

	"github.com/lunogram/platform/internal/config"
)

// SAMLSet is the deployment's SAML identity providers, in the order the
// operator declared them.
//
// It is the counterpart of [OIDCSet] and is not an [auth.Verifier] for the same
// reason: a federated credential arrives as a browser navigation carrying a
// provider in its path, which the generic "prove what is on this request"
// contract has no room for.
type SAMLSet struct {
	ordered []*SAML
	byID    map[string]*SAML
}

// NewSAMLSet builds every configured provider, or refuses.
//
// Every provider is validated at boot rather than at its first login. A
// deployment offering four ways in should not discover on a Monday morning that
// one of them was never going to work.
func NewSAMLSet(cfg config.SAMLAuth, opts SAMLOptions) (*SAMLSet, error) {
	declared, err := cfg.Resolve()
	if err != nil {
		return nil, err
	}
	if len(declared) == 0 {
		return nil, ErrSAMLNotConfigured
	}

	keypair, err := ParseSAMLKeypair(cfg.Certificate, cfg.PrivateKey)
	if err != nil {
		return nil, err
	}

	set := &SAMLSet{byID: make(map[string]*SAML, len(declared))}
	for index, settings := range declared {
		if err := validateProviderID(settings.ID, index); err != nil {
			return nil, err
		}
		if _, duplicate := set.byID[settings.ID]; duplicate {
			return nil, fmt.Errorf("two single sign-on providers share the id %q", settings.ID)
		}

		provider, err := NewSAML(SAMLOptions{
			Config:     settings,
			EntityID:   cfg.EntityID,
			Keypair:    keypair,
			Flows:      opts.Flows,
			Assertions: opts.Assertions,
			Metadata:   opts.Metadata,
			BaseURL:    opts.BaseURL,
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
// deployment that offers no SAML login at all.
func (s *SAMLSet) Provider(id string) *SAML {
	if s == nil {
		return nil
	}
	return s.byID[id]
}

// All returns the providers in declaration order.
func (s *SAMLSet) All() []*SAML {
	if s == nil {
		return nil
	}
	return slices.Clone(s.ordered)
}
