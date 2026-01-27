package v1

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/http/auth"
	"github.com/lunogram/platform/services/nexus/internal/http/auth/providers"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"go.uber.org/zap"
)

// OAuthCookieData represents the data stored in the OAuth cookie
type OAuthCookieData struct {
	AccessToken string    `json:"access_token"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func NewAuthController(logger *zap.Logger, db *sqlx.DB, cfg config.Node) (*AuthController, error) {
	stores := store.NewState(db)
	tokenGen := providers.NewJWTGeneratorWithSecret(cfg.Auth.JWTSecret, cfg.PlatformURL, cfg.Auth.TokenLife)

	driver := cfg.Auth.Driver
	if driver == "" {
		return nil, providers.ErrUnknownDriver
	}

	provider, err := NewProvider(driver, cfg, stores, logger)
	if err != nil {
		return nil, err
	}

	return &AuthController{
		logger:   logger,
		driver:   driver,
		provider: provider,
		tokenGen: tokenGen,
	}, nil
}

func NewProvider(driver string, cfg config.Node, stores *store.State, logger *zap.Logger) (providers.Provider, error) {
	switch driver {
	case "basic":
		return providers.NewBasicProvider(cfg.Auth.Basic, stores), nil
	case "clerk":
		return providers.NewClerkProvider(cfg.Auth.Clerk, stores, logger, auth.WithJWT(cfg.Auth, stores))
	default:
		return nil, providers.ErrUnknownDriver
	}
}

type AuthController struct {
	logger   *zap.Logger
	driver   string
	provider providers.Provider
	tokenGen *providers.HMACJWTGenerator
}

func (c *AuthController) GetAuthMethods(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode([]string{c.driver})
}

func (c *AuthController) AuthCallback(w http.ResponseWriter, r *http.Request, driver oapi.AuthCallbackParamsDriver) {
	if string(driver) != c.driver {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("auth driver not found")))
		return
	}

	admin, err := c.provider.Validate(r.Context(), r)
	if err != nil {
		c.logger.Error("auth validation failed", zap.String("driver", string(driver)), zap.Error(err))
		c.writeAuthError(w, err)
		return
	}

	token, expiresAt, err := c.tokenGen.Generate(admin.ID)
	if err != nil {
		c.logger.Error("failed to generate token", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to generate authentication token")))
		return
	}

	c.setOAuthCookie(w, r, token, expiresAt)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(oapi.AuthResponse{
		AccessToken: token,
		ExpiresAt:   expiresAt,
	})
}

func (c *AuthController) AuthWebhook(w http.ResponseWriter, r *http.Request, driver oapi.AuthWebhookParamsDriver) {
	if string(driver) != c.driver {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("auth driver not found")))
		return
	}

	if err := c.provider.Webhook(r.Context(), r); err != nil {
		c.logger.Error("webhook processing failed", zap.String("driver", string(driver)), zap.Error(err))
		if err == providers.ErrWebhookDenied {
			oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("webhook signature verification failed")))
			return
		}
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("webhook processing failed")))
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *AuthController) writeAuthError(w http.ResponseWriter, err error) {
	switch err {
	case providers.ErrMissingCredentials:
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("email and password are required")))
	case providers.ErrInvalidCredentials:
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("invalid email or password")))
	case providers.ErrNoToken:
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("authentication token required")))
	case providers.ErrInvalidToken:
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("invalid authentication token")))
	case providers.ErrInvalidEmail:
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("user has no valid email address")))
	default:
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("authentication failed")))
	}
}

func (c *AuthController) setOAuthCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	oauthData := OAuthCookieData{
		AccessToken: token,
		ExpiresAt:   expiresAt,
	}

	value, _ := json.Marshal(oauthData) //nolint:errcheck

	http.SetCookie(w, &http.Cookie{
		Name:     "oauth",
		Value:    base64.StdEncoding.EncodeToString(value),
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}
