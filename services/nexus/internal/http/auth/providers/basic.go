package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/http/auth"
	"github.com/lunogram/platform/services/nexus/internal/store"
)

var (
	ErrMissingCredentials = errors.New("email and password are required")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// BasicProvider is a static authentication provider that validates credentials
// against configured email/password values and signs its own tokens.
// It is primarily intended for simple setups and testing purposes.
type BasicProvider struct {
	config    config.BasicAuth
	stores    *store.State
	generator TokenGenerator
}

func NewBasicProvider(cfg config.BasicAuth, stores *store.State, generator TokenGenerator) *BasicProvider {
	return &BasicProvider{
		config:    cfg,
		stores:    stores,
		generator: generator,
	}
}

// Driver returns the driver identifier
func (p *BasicProvider) Driver() string {
	return "basic"
}

// basicAuthRequest represents the expected request body for basic auth
type basicAuthRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Authenticate validates the provided email and password credentials against
// the configured values. If valid, it finds or creates an admin user,
// generates a session token, sets the session cookie, and returns the admin.
func (provider *BasicProvider) Authenticate(ctx context.Context, w http.ResponseWriter, r *http.Request) (context.Context, error) {
	var req basicAuthRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return ctx, ErrMissingCredentials
	}

	if req.Email == "" || req.Password == "" {
		return ctx, ErrMissingCredentials
	}

	if req.Email != provider.config.Email || req.Password != provider.config.Password {
		return ctx, ErrInvalidCredentials
	}

	admin, err := provider.findOrCreateAdmin(ctx, req.Email)
	if err != nil {
		return ctx, err
	}

	token, expiresAt, err := provider.generator.Generate(admin.ID)
	if err != nil {
		return ctx, err
	}

	auth.SetSessionCookie(w, r, token, expiresAt)
	return ctx, nil
}

// findOrCreateAdmin finds an existing admin or creates a new one with a new organization
func (p *BasicProvider) findOrCreateAdmin(ctx context.Context, email string) (*store.Admin, error) {
	admin, err := p.stores.GetAdminByEmail(ctx, email)
	if err == nil && admin != nil {
		return admin, nil
	}

	admin = &store.Admin{
		Email: email,
		Role:  "owner",
	}

	admin.OrganizationID, err = p.stores.CreateOrganization(ctx, "Default Organization")
	if err != nil {
		return nil, err
	}

	admin.ID, err = p.stores.CreateAdmin(ctx, *admin)
	if err != nil {
		return nil, err
	}

	return admin, nil
}

// Webhook is not supported for basic auth
func (p *BasicProvider) Webhook(ctx context.Context, r *http.Request) error {
	return errors.New("webhooks not supported for basic auth")
}
