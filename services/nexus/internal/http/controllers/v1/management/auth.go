package v1

import (
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/http/auth/providers"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/http/json"
	"github.com/lunogram/platform/services/nexus/internal/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"go.uber.org/zap"
)

func NewAuthController(logger *zap.Logger, db *sqlx.DB, cfg config.Node) (*AuthController, error) {
	stores := store.NewState(db)

	provider, err := providers.NewProvider(cfg.Auth, stores, logger)
	if err != nil {
		return nil, err
	}

	return &AuthController{
		logger:   logger,
		provider: provider,
	}, nil
}

type AuthController struct {
	logger   *zap.Logger
	provider providers.Provider
	tokenGen *providers.HMACJWTGenerator
}

func (c *AuthController) GetAuthMethods(w http.ResponseWriter, r *http.Request) {
	json.Write(w, http.StatusOK, []string{c.provider.Driver()})
}

func (c *AuthController) AuthCallback(w http.ResponseWriter, r *http.Request, driver oapi.AuthCallbackParamsDriver) {
	if string(driver) != c.provider.Driver() {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("auth driver not found")))
		return
	}

	ctx := r.Context()
	ctx, err := c.provider.Authenticate(ctx, w, r)
	if err != nil {
		c.logger.Error("auth validation failed", zap.String("driver", string(driver)), zap.Error(err))
		c.writeAuthError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *AuthController) AuthWebhook(w http.ResponseWriter, r *http.Request, driver oapi.AuthWebhookParamsDriver) {
	if string(driver) != c.provider.Driver() {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("auth driver not found")))
		return
	}

	ctx := r.Context()
	err := c.provider.Webhook(ctx, r)
	if err != nil {
		c.logger.Error("webhook processing failed", zap.String("driver", string(driver)), zap.Error(err))
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
	case providers.ErrNoSession:
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("authentication token required")))
	case providers.ErrInvalidToken:
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("invalid authentication token")))
	case providers.ErrInvalidEmail:
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("user has no valid email address")))
	default:
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("authentication failed")))
	}
}
