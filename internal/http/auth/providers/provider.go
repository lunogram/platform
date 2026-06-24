package providers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/rbac/access"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

var (
	ErrNoSession        = errors.New("no session token provided")
	ErrInvalidToken     = errors.New("invalid authentication token")
	ErrInvalidEmail     = errors.New("user has no email address")
	ErrWebhookDenied    = errors.New("webhook signature verification failed")
	ErrUnknownDriver    = errors.New("unknown auth driver")
	ErrProviderNotFound = errors.New("auth provider not found")
)

// AuthResult represents the result of a successful authentication
type AuthResult struct {
	Admin       *management.Admin
	AccessToken string
	ExpiresAt   time.Time
}

// RBACWriter is the subset of the RBAC engine needed by auth providers to
// grant organization-level roles when a new admin is provisioned. Accepting
// an interface instead of the concrete *rbac.Engine avoids an import cycle
// and makes unit-testing straightforward (callers can supply a no-op or a
// recording implementation).
type RBACWriter interface {
	// WriteTuple adds a single relationship tuple to the authorization store.
	// For admin provisioning this is typically:
	//   WriteTuple(ctx, "user:<admin-uuid>", "owner", "organization:<org-uuid>")
	WriteTuple(ctx context.Context, user, relation, object string) error
}

// provisionMembership records a freshly provisioned admin's membership in its
// home organization: the organization_members row plus the matching RBAC owner
// tuples. Every admin-provisioning path calls this so the two representations
// stay in sync (an admin that is a member but has no tuples cannot pass
// permission checks, and vice versa).
func provisionMembership(ctx context.Context, mgmt *management.State, writer RBACWriter, adminID, organizationID uuid.UUID, role string) error {
	if err := mgmt.AddMember(ctx, organizationID, adminID, role); err != nil {
		return fmt.Errorf("failed to add organization membership for new admin: %w", err)
	}

	if writer != nil {
		for _, t := range access.OrganizationRoleTuples(adminID, organizationID, role) {
			if err := writer.WriteTuple(ctx, t.User, t.Relation, t.Object); err != nil {
				return fmt.Errorf("failed to write RBAC tuple for new admin: %w", err)
			}
		}
	}
	return nil
}

func NewProvider(cfg config.Auth, mgmt *management.State, logger *zap.Logger, rbac RBACWriter) (Provider, error) {
	switch cfg.Driver {
	case "basic":
		generator := NewHMACJWTGenerator(cfg.JWTSecret, cfg.TokenLife)
		return NewBasicProvider(cfg.Basic, mgmt, generator, rbac), nil
	case "clerk":
		return NewClerkProvider(cfg.Clerk, mgmt, logger, cfg.JWKS.Unwrap(), rbac)
	default:
		return nil, ErrUnknownDriver
	}
}

// Provider defines the interface for authentication providers
type Provider interface {
	// Driver returns the unique driver identifier
	Driver() string

	// Authenticate the authentication credentials from the HTTP request.
	// It extracts and verifies the token or credentials from the request,
	// and returns the associated Admin if authentication is successful.
	// Returns an error if the credentials are missing, invalid, or expired.
	Authenticate(ctx context.Context, w http.ResponseWriter, r *http.Request) (context.Context, error)

	// Webhook handles webhook callbacks from external auth providers (optional)
	// Returns nil if the provider doesn't support webhooks
	Webhook(ctx context.Context, r *http.Request) error
}

// TokenGenerator generates JWT tokens for authenticated sessions
type TokenGenerator interface {
	Generate(sub uuid.UUID) (token string, expiresAt time.Time, err error)
}
