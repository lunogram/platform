package providers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
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

func NewProvider(cfg config.Auth, mgmt *management.State, logger *zap.Logger) (Provider, error) {
	switch cfg.Driver {
	case "basic":
		generator := NewHMACJWTGenerator(cfg.JWTSecret, cfg.TokenLife)
		return NewBasicProvider(cfg.Basic, mgmt, generator), nil
	case "clerk":
		return NewClerkProvider(cfg.Clerk, mgmt, logger, cfg.JWKS.Unwrap())
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
