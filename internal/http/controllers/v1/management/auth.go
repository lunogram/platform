package v1

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/http/auth/verifiers"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/ratelimit"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

// NewAuthController wires the login callbacks: one verifier per configured
// driver, and one exchanger that turns whatever any of them proves into a
// Lunogram console session.
//
// mgmt must be the same (Redis-backed) store the authentication middleware
// reads through. Building a separate one here would leave logout unable to
// invalidate the shared session cache, so a revoked session would keep
// authenticating on other replicas until the cache TTL elapsed.
func NewAuthController(logger *zap.Logger, db *sqlx.DB, mgmt *management.State, cfg config.Node, engine *rbac.Engine, signer *auth.ConsoleSigner, limiter *ratelimit.Limiter) (*AuthController, error) {
	controller := &AuthController{
		logger:   logger,
		mgmt:     mgmt,
		db:       db,
		signer:   signer,
		throttle: newThrottle(limiter, cfg.RateLimit.TrustedProxyHops),
	}

	controller.exchanger = auth.NewExchanger(db, mgmt, engine, signer, nil, logger, cfg.Auth.LegacyIdentityAdoption)

	built, err := verifiers.Build(cfg.Auth, mgmt, logger, controller.exchanger)
	if err != nil {
		return nil, err
	}
	controller.verifiers = built

	// Kept in configuration order rather than map order so the console offers
	// the drivers in the order the operator listed them, deterministically.
	for _, driver := range cfg.Auth.Drivers {
		driver = strings.ToLower(strings.TrimSpace(driver))
		if _, ok := controller.verifiers[driver]; ok && !slices.Contains(controller.drivers, driver) {
			controller.drivers = append(controller.drivers, driver)
		}
	}

	if err := controller.initPasswordAuth(cfg); err != nil {
		return nil, err
	}

	// The configured pair is written into an account rather than compared
	// against, so it has to exist before anybody can sign in with it. Doing it
	// here means a fresh deployment is reachable the moment it starts, and an
	// upgrading one keeps signing in with the credential it already used.
	if cfg.Auth.Enabled(verifiers.BasicDriver) {
		seeder := auth.NewSeeder(controller.exchanger, mgmt, logger.Named("seed"))
		if err := seeder.Seed(context.Background(), cfg.Auth.Basic.Email, cfg.Auth.Basic.Password); err != nil {
			return nil, err
		}
	}

	return controller, nil
}

type AuthController struct {
	logger *zap.Logger
	mgmt   *management.State
	// db is the pool the credential flows start their transactions on. The
	// cache-backed mgmt above cannot: a store built over a transaction has no
	// cache attached, so the two are used together — writes through the
	// transaction, cache invalidation through mgmt once it commits.
	db        *sqlx.DB
	signer    *auth.ConsoleSigner
	verifiers map[string]auth.Verifier
	drivers   []string
	exchanger *auth.Exchanger
	throttle  *throttle

	password passwordAuth
}

// Close releases what the controller owns beyond the request path.
//
// The mail dispatcher is the only such thing: it holds queued messages and its
// own workers, and draining it is what stops a shutdown from swallowing a
// verification link somebody is waiting on. Nothing called it before, so that
// promise was only ever kept in tests.
func (c *AuthController) Close() {
	c.password.mail.Close()
}

// Verifier returns the verifier for one driver, or nil when that driver is not
// configured.
func (c *AuthController) Verifier(driver string) auth.Verifier { return c.verifiers[driver] }

// LegacyVerifier is the verifier for the transitional legacy-cookie upgrade.
//
// It is specifically Clerk's: Clerk's browser SDK is the only thing that ever
// wrote the `__session` cookie the upgrade reads, so no other driver could
// prove one. It is exposed so the middleware reuses the configured verifier
// rather than constructing a second one.
func (c *AuthController) LegacyVerifier() auth.Verifier {
	return c.verifiers[verifiers.ClerkDriver]
}

// Exchanger is the single path from a verified identity to a console session.
func (c *AuthController) Exchanger() *auth.Exchanger { return c.exchanger }

// GetAuthMethods lists every configured driver. A deployment may offer several
// at once -- passwords alongside SSO during a migration, say -- and the console
// picks between them.
func (c *AuthController) GetAuthMethods(w http.ResponseWriter, r *http.Request) {
	drivers := c.drivers
	if drivers == nil {
		drivers = []string{}
	}
	json.Write(w, http.StatusOK, drivers)
}

// AuthCallback completes a login: the driver proves the credential and the
// exchange turns that proof into a Lunogram console session. The verifier is
// never handed the ResponseWriter, so no driver can set a cookie or mint a
// token of its own.
func (c *AuthController) AuthCallback(w http.ResponseWriter, r *http.Request, driver oapi.AuthCallbackParamsDriver) {
	verifier := c.verifiers[string(driver)]
	if verifier == nil {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("auth driver not found")))
		return
	}

	ctx := r.Context()

	// A credential submitted as an email and a password is guessable, so it is
	// throttled per account and per source. A token-bearing driver is not: the
	// upstream owns the guessing problem, and there is no account to key on.
	budgets, throttled := c.loginBudgets(r, verifier)
	if throttled {
		if tripped, retryAfter := c.throttle.exceeded(ctx, budgets); tripped {
			c.logger.Warn("refusing a login attempt: too many recent failures", zap.String("driver", string(driver)))
			writeTooManyRequests(w, retryAfter)
			return
		}
	}

	identity, err := verifier.Verify(ctx, r)
	if err != nil {
		if throttled {
			// Only failures spend budget, so signing in correctly on many
			// devices never counts against the account.
			c.throttle.spend(ctx, budgets)
		}
		c.logger.Warn("auth validation failed", zap.String("driver", string(driver)), zap.Error(err))
		c.writeAuthError(w, err)
		return
	}

	if _, err := c.exchanger.Exchange(ctx, w, r, identity); err != nil {
		c.logger.Error("session exchange failed", zap.String("driver", string(driver)), zap.Error(err))
		c.writeAuthError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// loginBudgets derives the throttling keys for a login attempt, buffering the
// request body so the verifier still sees it intact.
//
// The account key comes from the address the caller submitted rather than from
// an account we looked up, so an address with no account is throttled exactly
// like one that has an account.
func (c *AuthController) loginBudgets(r *http.Request, verifier auth.Verifier) (map[budget]string, bool) {
	switch verifier.Driver() {
	case verifiers.BasicDriver:
	default:
		return nil, false
	}

	email, err := peekCredentialEmail(r)
	if err != nil {
		c.logger.Warn("failed to read the credential body for throttling", zap.Error(err))
	}

	return map[budget]string{
		loginAccountBudget: accountKey(email),
		loginSourceBudget:  c.throttle.sourceKey(r),
	}, true
}

func (c *AuthController) AuthWebhook(w http.ResponseWriter, r *http.Request, driver oapi.AuthWebhookParamsDriver) {
	verifier := c.verifiers[string(driver)]
	if verifier == nil {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("auth driver not found")))
		return
	}

	webhooks, ok := verifier.(auth.WebhookVerifier)
	if !ok {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("this auth driver has no webhook")))
		return
	}

	if err := webhooks.Webhook(r.Context(), r); err != nil {
		c.logger.Error("webhook processing failed", zap.String("driver", string(driver)), zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("webhook processing failed")))
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Logout revokes the server-side session and clears every cookie a credential
// could have arrived in. Revoking server-side is the point of having a session
// table at all: clearing a cookie alone leaves a still-valid bearer token in
// whatever copied it.
func (c *AuthController) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Cleared FIRST, and deliberately not deferred: a Set-Cookie header written
	// after the status line is silently dropped, so a deferred clear would leave
	// the browser holding every cookie it arrived with. Clearing up front also
	// means a browser presenting a credential we cannot read, or a revoke that
	// then fails, is still not left stuck with it.
	auth.ClearConsoleSessionCookies(w, r)

	claims, ok := c.currentSession(r)
	if !ok {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := c.mgmt.RevokeAdminSession(ctx, claims.SessionID); err != nil {
		c.logger.Error("failed to revoke session", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to end the session")))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RefreshSession extends the idle window of the caller's own session and issues
// a token for the new expiry.
//
// The session is re-read and re-checked in the database rather than trusted from
// the presented token, so a revoked, expired or non-refreshable session cannot
// be extended.
//
// Those two failures are told apart deliberately. A session that is gone answers
// 401, and the caller should send its holder to the login page. A session that
// is alive but cannot be extended -- impersonation is recorded non-refreshable
// by construction -- answers 403: nothing is wrong, there is simply nothing to
// do, and treating that as a logout would eject an operator from a session that
// is working perfectly well.
func (c *AuthController) RefreshSession(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	claims, ok := c.currentSession(r)
	if !ok {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	current, err := c.mgmt.GetAdminSession(ctx, claims.SessionID)
	if err != nil || !current.Active(time.Now()) {
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	if !current.Refreshable {
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("this session cannot be extended")))
		return
	}

	session, err := c.mgmt.RefreshAdminSession(ctx, claims.SessionID, time.Now().Add(c.signer.IdleTTL()))
	if err != nil {
		// It was live a moment ago, so it has just been revoked or has expired
		// between the two statements.
		oapi.WriteProblem(w, problem.ErrUnauthorized())
		return
	}

	token, err := c.signer.Mint(session, claims.Methods)
	if err != nil {
		c.logger.Error("failed to mint refreshed session", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	auth.SetConsoleSessionCookie(w, r, token, session.ExpiresAt)
	json.Write(w, http.StatusOK, oapi.SessionRefresh{ExpiresAt: session.ExpiresAt})
}

// currentSession verifies the credential presented on the request without
// consulting the database. It only says which session the caller is claiming;
// anything that acts on that session must re-read it.
func (c *AuthController) currentSession(r *http.Request) (*auth.ConsoleClaims, bool) {
	if c.signer == nil {
		return nil, false
	}
	token := auth.GetSession(r)
	if token == "" {
		return nil, false
	}
	claims, err := c.signer.Verify(token)
	if err != nil {
		return nil, false
	}
	return claims, true
}

func (c *AuthController) writeAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, verifiers.ErrMissingCredentials):
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("email and password are required")))
	case errors.Is(err, verifiers.ErrInvalidCredentials):
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("invalid email or password")))
	case errors.Is(err, verifiers.ErrNoSession):
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("authentication token required")))
	case errors.Is(err, verifiers.ErrInvalidToken):
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("invalid authentication token")))
	case errors.Is(err, verifiers.ErrInvalidEmail):
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("user has no valid email address")))
	default:
		// An error raised by the exchange already carries the status it deserves
		// (403 when impersonation would have provisioned, 409 on a contested
		// email address); anything else falls through to a plain 500 without
		// leaking its text.
		oapi.WriteProblem(w, err)
	}
}
