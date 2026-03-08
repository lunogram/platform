package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/rbac/access"
	"github.com/lunogram/platform/internal/store/management"
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
	mgmt      *management.State
	generator TokenGenerator
	rbac      RBACWriter
}

func NewBasicProvider(cfg config.BasicAuth, mgmt *management.State, generator TokenGenerator, rbac RBACWriter) *BasicProvider {
	return &BasicProvider{
		config:    cfg,
		mgmt:      mgmt,
		generator: generator,
		rbac:      rbac,
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
func (p *BasicProvider) findOrCreateAdmin(ctx context.Context, email string) (*management.Admin, error) {
	admin, err := p.mgmt.GetAdminByEmail(ctx, email)
	if err == nil && admin != nil {
		return admin, nil
	}

	admin = &management.Admin{
		Email: email,
		Role:  "owner",
	}

	admin.OrganizationID, err = p.mgmt.CreateOrganization(ctx, "Default Organization")
	if err != nil {
		return nil, err
	}

	admin.ID, err = p.mgmt.CreateAdmin(ctx, *admin)
	if err != nil {
		return nil, err
	}

	// Grant the new admin the owner role on the organization in the RBAC engine
	// so that subsequent permission checks (e.g. read profile, list/create
	// projects) succeed.
	if p.rbac != nil {
		for _, t := range access.OrganizationRoleTuples(admin.ID, admin.OrganizationID, admin.Role) {
			if err := p.rbac.WriteTuple(ctx, t.User, t.Relation, t.Object); err != nil {
				return nil, fmt.Errorf("failed to write RBAC tuple for new admin: %w", err)
			}
		}
	}

	return admin, nil
}

// Webhook is not supported for basic auth
func (p *BasicProvider) Webhook(ctx context.Context, r *http.Request) error {
	return errors.New("webhooks not supported for basic auth")
}
