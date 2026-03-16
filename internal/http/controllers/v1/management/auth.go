package v1

import (
	"bytes"
	"errors"
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

var ErrInvalidRedirect = errors.New("redirect host not allowed")

func NewAuthController(logger *zap.Logger, db *sqlx.DB, cfg config.Node, engine *rbac.Engine) (*AuthController, error) {
	stores := management.NewState(db)

	provider, err := providers.NewProvider(cfg.Auth, stores, logger, engine)
	if err != nil {
		return nil, err
	}

	return &AuthController{
		logger:               logger,
		provider:             provider,
		allowedRedirectHosts: cfg.Auth.AllowedRedirectHosts,
	}, nil
}

type AuthController struct {
	logger               *zap.Logger
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

	// Decode the body once to validate the redirect parameter, then re-encode
	// it so the provider can read it without consuming the body a second time.
	var req oapi.AuthCallbackRequest
	if err := json.Decode(r.Body, &req); err != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("failed to read request body")))
		return
	}

	if req.Redirect != nil {
		if err := c.validateRedirect(*req.Redirect); err != nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid redirect URL")))
			return
		}
	}

	encoded, err := json.Marshal(req)
	if err != nil {
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("authentication failed")))
		return
	}

	r.Body = io.NopCloser(bytes.NewReader(encoded))

	ctx := r.Context()
	_, err = c.provider.Authenticate(ctx, w, r)
	if err != nil {
		c.logger.Error("auth validation failed", zap.String("driver", string(driver)), zap.Error(err))
		c.writeAuthError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// validateRedirect checks that the redirect URL is safe to redirect to.
// Relative URLs (starting with "/" but not "//") are always allowed.
// Absolute URLs are only allowed if their hostname appears in the
// configured AllowedRedirectHosts list.
func (c *AuthController) validateRedirect(redirect string) error {
	// Allow relative URLs that start with "/" but not "//" (protocol-relative).
	// Protocol-relative URLs such as "//evil.com/path" are treated as absolute
	// by browsers and would redirect to an external host.
	if strings.HasPrefix(redirect, "/") && !strings.HasPrefix(redirect, "//") {
		return nil
	}

	parsed, err := url.Parse(redirect)
	if err != nil {
		return ErrInvalidRedirect
	}

	host := parsed.Hostname()
	for _, allowed := range c.allowedRedirectHosts {
		if host == allowed {
			return nil
		}
	}

	return ErrInvalidRedirect
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
