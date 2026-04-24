package v1

import (
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http/auth/providers"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewAuthController(logger *zap.Logger, db *sqlx.DB, cfg config.Node, engine *rbac.Engine) (*AuthController, error) {
	stores := management.NewState(db)

	provider, err := providers.NewProvider(cfg.Auth, stores, logger, engine)
	if err != nil {
		return nil, err
	}

	return &AuthController{
		logger:                logger,
		provider:             provider,
		allowedRedirectHosts: cfg.Auth.AllowedRedirectHosts,
	}, nil
}

type AuthController struct {
	logger                *zap.Logger
	provider             providers.Provider
	allowedRedirectHosts []string
}

func (c *AuthController) GetAuthMethods(w http.ResponseWriter, r *http.Request) {
	json.Write(w, http.StatusOK, []string{c.provider.Driver()})
}

func (c *AuthController) AuthCallback(w http.ResponseWriter, r *http.Request, driver oapi.AuthCallbackParamsDriver) {
	if string(driver) != c.provider.Driver() {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("auth driver not found")))
		return
	}

	body := r.Body
	defer body.Close()

	rawBody, err := io.ReadAll(body)
	if err != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("failed to read request body")))
		return
	}

	var callbackReq oapi.AuthCallbackJSONRequestBody
	if len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &callbackReq); err != nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid JSON body")))
			return
		}

		if callbackReq.Redirect != nil && !c.validateRedirect(*callbackReq.Redirect) {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("redirect URL is not allowed")))
			return
		}
	}

	r.Body = io.NopCloser(strings.NewReader(string(rawBody)))

	ctx := r.Context()
	_, err = c.provider.Authenticate(ctx, w, r)
	if err != nil {
		c.logger.Error("auth validation failed", zap.String("driver", string(driver)), zap.Error(err))
		c.writeAuthError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (c *AuthController) validateRedirect(redirect string) bool {
	if redirect == "" {
		return true
	}

	parsed, err := url.Parse(redirect)
	if err != nil {
		return false
	}

	if parsed.Scheme == "" && parsed.Host == "" {
		return true
	}

	if parsed.Scheme == "" && parsed.Host != "" {
		return false
	}

	host := parsed.Hostname()
	for _, allowed := range c.allowedRedirectHosts {
		if host == allowed {
			return true
		}
	}

	return false
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