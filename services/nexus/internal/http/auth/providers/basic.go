package providers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/store"
)

var (
	ErrMissingCredentials = errors.New("email and password are required")
	ErrInvalidCredentials = errors.New("invalid email or password")
)

// BasicProvider implements basic email/password authentication
type BasicProvider struct {
	config   config.BasicAuth
	stores   *store.State
	orgStore OrganizationStore
}

// NewBasicProvider creates a new basic auth provider
func NewBasicProvider(cfg config.BasicAuth, stores *store.State) *BasicProvider {
	return &BasicProvider{
		config:   cfg,
		stores:   stores,
		orgStore: stores,
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

// Validate validates email/password credentials
func (p *BasicProvider) Validate(ctx context.Context, r *http.Request) (*store.Admin, error) {
	var req basicAuthRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return nil, ErrMissingCredentials
	}

	if req.Email == "" || req.Password == "" {
		return nil, ErrMissingCredentials
	}

	if req.Email != p.config.Email || req.Password != p.config.Password {
		return nil, ErrInvalidCredentials
	}

	admin, err := p.findOrCreateAdmin(ctx, req.Email)
	if err != nil {
		return nil, err
	}

	return admin, nil
}

// findOrCreateAdmin finds an existing admin or creates a new one with a new organization
func (p *BasicProvider) findOrCreateAdmin(ctx context.Context, email string) (*store.Admin, error) {
	admin, err := p.stores.GetAdminByEmailGlobal(ctx, email)
	if err == nil && admin != nil {
		return admin, nil
	}

	orgID, err := p.orgStore.CreateOrganization(ctx, "Organization")
	if err != nil {
		return nil, err
	}

	newAdmin := store.Admin{
		OrganizationID: orgID,
		Email:          email,
		Role:           "owner",
	}

	newAdmin.ID, err = p.stores.CreateAdmin(ctx, newAdmin)
	if err != nil {
		return nil, err
	}

	return &newAdmin, nil
}

// Webhook is not supported for basic auth
func (p *BasicProvider) Webhook(ctx context.Context, r *http.Request) error {
	return errors.New("webhooks not supported for basic auth")
}
