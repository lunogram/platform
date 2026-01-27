package providers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/internal/store"
)

var (
	ErrNoToken          = errors.New("no authentication token provided")
	ErrInvalidToken     = errors.New("invalid authentication token")
	ErrInvalidEmail     = errors.New("user has no email address")
	ErrWebhookDenied    = errors.New("webhook signature verification failed")
	ErrUnknownDriver    = errors.New("unknown auth driver")
	ErrProviderNotFound = errors.New("auth provider not found")
)

// AuthResult represents the result of a successful authentication
type AuthResult struct {
	Admin       *store.Admin
	AccessToken string
	ExpiresAt   time.Time
}

// Provider defines the interface for authentication providers
type Provider interface {
	// Driver returns the driver identifier (e.g., "basic", "clerk")
	Driver() string

	// Validate validates the authentication request and returns admin info
	// For basic auth: validates email/password
	// For clerk auth: validates JWT token and creates/retrieves admin
	Validate(ctx context.Context, r *http.Request) (*store.Admin, error)

	// Webhook handles webhook callbacks from external auth providers (optional)
	// Returns nil if the provider doesn't support webhooks
	Webhook(ctx context.Context, r *http.Request) error
}

// TokenGenerator generates JWT tokens for authenticated sessions
type TokenGenerator interface {
	Generate(adminID uuid.UUID) (token string, expiresAt time.Time, err error)
}

// AdminStore defines the store operations needed by auth providers
type AdminStore interface {
	GetAdminByExternalID(ctx context.Context, externalID string) (*store.Admin, error)
	GetAdminByEmail(ctx context.Context, email string, organizationID uuid.UUID) (*store.Admin, error)
	CreateAdmin(ctx context.Context, admin store.Admin) (uuid.UUID, error)
	UpdateAdmin(ctx context.Context, id uuid.UUID, update store.AdminUpdate) error
	DeleteAdmin(ctx context.Context, id uuid.UUID) error
}

// OrganizationStore defines the store operations for organizations
type OrganizationStore interface {
	CreateOrganization(ctx context.Context, name string) (uuid.UUID, error)
}
