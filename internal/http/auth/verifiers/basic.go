package verifiers

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/store/management"
)

// BasicIssuer namespaces identities proved by the static credential. It is a
// URN rather than a URL because there is no upstream to dereference: the
// "issuer" is this deployment's own configuration.
const BasicIssuer = "urn:lunogram:basic"

// Basic verifies the single email/password pair configured through
// AUTH_BASIC_EMAIL / AUTH_BASIC_PASSWORD. It is the documented quickstart, so it
// is a first-class verifier rather than a development shortcut -- it just proves
// a credential and stops, like every other one.
type Basic struct {
	config config.BasicAuth
}

func NewBasic(cfg config.BasicAuth) *Basic {
	return &Basic{config: cfg}
}

func (b *Basic) Driver() string { return "basic" }

type basicCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Verify checks the submitted credentials against the configured pair.
//
// The identity it returns is keyed on the CONFIGURED address, not the submitted
// one, so a case-variant submission cannot mint a second identity for the same
// account. The address counts as verified because the credential is the address:
// there is no self-service profile here for someone to type another person's
// email into.
func (b *Basic) Verify(_ context.Context, r *http.Request) (*auth.VerifiedIdentity, error) {
	if b.config.Email == "" || b.config.Password == "" {
		return nil, ErrInvalidCredentials
	}

	var credentials basicCredentials
	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		return nil, ErrMissingCredentials
	}

	if credentials.Email == "" || credentials.Password == "" {
		return nil, ErrMissingCredentials
	}

	// Constant-time comparison so a wrong password cannot be recovered a byte at
	// a time from response timing.
	emailMatch := subtle.ConstantTimeCompare([]byte(strings.ToLower(credentials.Email)), []byte(strings.ToLower(b.config.Email)))
	passwordMatch := subtle.ConstantTimeCompare([]byte(credentials.Password), []byte(b.config.Password))
	if emailMatch != 1 || passwordMatch != 1 {
		return nil, ErrInvalidCredentials
	}

	email := strings.ToLower(b.config.Email)
	return &auth.VerifiedIdentity{
		Issuer:        BasicIssuer,
		Subject:       email,
		Provider:      management.IdentityProviderBasic,
		Email:         email,
		EmailVerified: true,
	}, nil
}
