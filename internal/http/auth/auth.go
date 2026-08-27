package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
)

// TokenClaims represents the claims from a JWT token
type TokenClaims struct {
	jwt.RegisteredClaims
}

// Subject returns the subject claim
func (c *TokenClaims) Subject() string {
	return c.RegisteredClaims.Subject
}

// Issuer returns the issuer claim
func (c *TokenClaims) Issuer() string {
	return c.RegisteredClaims.Issuer
}

// ErrUnauthorized is returned when the authentication fails.
var ErrUnauthorized = errors.New("unauthorized")

// ErrInsecureJWTSecret is returned at construction when the configured admin
// signing secret cannot be trusted to keep admin sessions private.
var ErrInsecureJWTSecret = errors.New("insecure AUTH_JWT_SECRET")

// ErrMissingJWKS is returned at construction when the clerk driver has no JWKS
// to verify admin sessions against.
var ErrMissingJWKS = errors.New("missing AUTH_JWKS_URL")

const (
	driverBasic = "basic"
	driverClerk = "clerk"

	// minJWTSecretBytes is the shortest admin signing secret accepted. HS256
	// keys below the 32-byte HMAC-SHA256 block size leave no margin against
	// offline guessing of a captured token.
	minJWTSecretBytes = 32

	// generateJWTSecret is the command suggested to operators whose secret is
	// rejected.
	generateJWTSecret = "openssl rand -base64 48"
)

// publishedJWTSecrets are values that have appeared in this repository, its
// compose files or its documentation, including ones the working tree no longer
// contains — git history is as public as a checkout, and a deployment stood up
// from an older compose file is still running the value it shipped with.
// Anyone who has read the repo knows them, so none may serve as admin signing
// key material — a copied example is exactly how a deployment ends up with a
// secret its operator believes is private. Extend this list whenever an example
// secret is added anywhere public.
var publishedJWTSecrets = map[string]struct{}{
	"dev-secret-change-in-production": {},
	"never-gonna-give-you-up":         {},
}

func HMAC(secret []byte) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		return secret, nil
	}
}

// adminTokenVerifier binds both the accepted signature algorithms and the key
// material to the configured driver: a clerk deployment verifies RS256 against
// the provider's JWKS, a basic deployment verifies HS256 against the local
// signing secret. Neither mode is widened by the mere presence of the other's
// configuration.
//
// Binding the algorithm set to key-material *presence* instead ("a secret
// exists, therefore also accept HS256") is not safe. It is true that the two
// algorithms never share key material, so the classic RS256→HS256 confusion
// attack cannot happen — but that reasoning only covers confusion between two
// trusted keys. A secret that an attacker knows (a leftover or copied example
// value on a deployment that authenticates through Clerk) is enough on its own
// to mint an admin session, because the HS256 branch is a fully trusted
// verification path. Selecting on the driver keeps exactly one algorithm and
// one key source live, so an unused secret grants nothing.
func adminTokenVerifier(cfg config.Auth) ([]string, jwt.Keyfunc, error) {
	switch cfg.Driver {
	case driverBasic:
		if err := validateJWTSecret(cfg.JWTSecret); err != nil {
			return nil, nil, err
		}
		return []string{"HS256"}, HMAC([]byte(cfg.JWTSecret)), nil
	case driverClerk:
		// Symmetry with the basic branch: refuse to start on key material that
		// cannot verify anything. Without a JWKS the parse below has no key and
		// every admin login fails — fail closed, but silently, and it presents
		// as "Clerk login is broken" rather than as a missing variable.
		jwks := cfg.JWKS.Unwrap()
		if jwks == nil {
			return nil, nil, fmt.Errorf("%w: the %q auth driver verifies admin sessions against the provider's JWKS; set AUTH_JWKS_URL to your Clerk instance's JWKS endpoint (https://<your-instance>/.well-known/jwks.json)", ErrMissingJWKS, driverClerk)
		}
		return []string{"RS256"}, jwks, nil
	default:
		return nil, nil, fmt.Errorf("unsupported auth driver %q: set AUTH_DRIVER to %q or %q", cfg.Driver, driverBasic, driverClerk)
	}
}

// validateJWTSecret rejects admin signing secrets that cannot keep a session
// private: absent, publicly known, or short enough to guess offline. It runs
// at construction so a deployment that would hand out forgeable admin sessions
// refuses to start instead of serving them.
func validateJWTSecret(secret string) error {
	switch {
	case secret == "":
		return fmt.Errorf("%w: not set, but the %q auth driver signs admin sessions with it; generate one with `%s`", ErrInsecureJWTSecret, driverBasic, generateJWTSecret)
	case isPublishedJWTSecret(secret):
		return fmt.Errorf("%w: set to the example value published in the repository, so anyone can mint admin sessions with it; generate a private one with `%s`", ErrInsecureJWTSecret, generateJWTSecret)
	case len(secret) < minJWTSecretBytes:
		return fmt.Errorf("%w: %d bytes, at least %d are required; generate one with `%s`", ErrInsecureJWTSecret, len(secret), minJWTSecretBytes, generateJWTSecret)
	}
	return nil
}

// isPublishedJWTSecret reports whether the secret is one this project has made
// public. The comparison is case-insensitive and ignores surrounding whitespace
// so that a value copied out of a guide is still recognised.
func isPublishedJWTSecret(secret string) bool {
	_, published := publishedJWTSecrets[strings.ToLower(strings.TrimSpace(secret))]
	return published
}

type Handler func(ctx context.Context, token string) (context.Context, error)

// Surface identifies which API an API-key request is authenticating against.
// Key scope is enforced differently per surface; see [WithKey].
type Surface int

const (
	SurfaceManagement Surface = iota
	SurfaceClient
)

type requestContextKey struct{}

// withRequest stores the in-flight request on the context so authentication
// handlers can inspect request headers (e.g. Origin) during scope enforcement.
func withRequest(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, requestContextKey{}, r)
}

// RequestFromContext returns the request stored by the authentication
// middleware, or nil if none is present.
func RequestFromContext(ctx context.Context) *http.Request {
	r, _ := ctx.Value(requestContextKey{}).(*http.Request)
	return r
}

// Middleware is a middleware function that authenticates requests by verifying the
// authorization header. It returns a AuthenticationFunc that checks the authorization
// header and adds the authenticated session to the request context.
func Middleware(middleware ...Handler) openapi3filter.AuthenticationFunc {
	return func(ctx context.Context, filter *openapi3filter.AuthenticationInput) error {
		req := filter.RequestValidationInput.Request

		authenticated, err := authenticate(ctx, req, middleware)
		if err != nil {
			return err
		}

		*req = *req.WithContext(authenticated)
		return nil
	}
}

// authenticate walks the handler chain until one accepts the request's
// credential, and returns the context that handler produced. A handler that
// rejects the credential (ErrUnauthorized) hands over to the next one; any other
// error aborts the walk, so a failure to *evaluate* a credential is never read
// as "this credential does not apply" and retried against a weaker one.
//
// Every handler is given the incoming context rather than the previous
// handler's, so a rejected attempt leaves nothing behind on the context that
// follows it.
//
// It is shared by [Middleware], which authenticates the OpenAPI surfaces, and
// [Require], which authenticates routes mounted outside the validator, so both
// resolve credentials identically.
func authenticate(ctx context.Context, r *http.Request, handlers []Handler) (context.Context, error) {
	token := GetSession(r)
	ctx = withRequest(ctx, r)

	for _, handler := range handlers {
		authenticated, err := handler(ctx, token)
		if err == nil {
			return authenticated, nil
		}

		if errors.Is(err, ErrUnauthorized) {
			continue
		}

		return ctx, err
	}

	return ctx, ErrUnauthorized
}

// WithJWT authenticates an admin session token. The accepted algorithms and the
// verification key are selected by the configured driver; see
// [adminTokenVerifier]. Construction fails when the driver issues HS256 tokens
// but its signing secret is weak, so the process refuses to start rather than
// accepting forgeable admin sessions.
func WithJWT(config config.Auth, mgmt *management.State) (Handler, error) {
	methods, keyFunc, err := adminTokenVerifier(config)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context, value string) (context.Context, error) {
		claims := jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(value, &claims, keyFunc,
			jwt.WithValidMethods(methods),
			jwt.WithExpirationRequired(),
		)
		if err != nil {
			return ctx, ErrUnauthorized
		}

		if !token.Valid {
			return ctx, ErrUnauthorized
		}

		admin, err := mgmt.GetAdminBySubject(ctx, claims.Issuer, claims.Subject)
		if err != nil {
			return ctx, ErrUnauthorized
		}

		orgID, err := resolveActiveOrganization(ctx, mgmt, admin)
		if err != nil {
			// A real failure resolving the active organization (e.g. a transient
			// DB error) must NOT fail open onto the home org — that would bypass
			// the revoked-membership check on a hot security path. Surface the
			// error so the request fails instead of silently granting scope.
			return ctx, err
		}

		actor := rbac.NewActor(
			rbac.ActorAdmin,
			admin.ID.String(),
			rbac.WithOrganizationID(orgID),
		)

		return rbac.WithActor(ctx, actor), nil
	}, nil
}

// resolveActiveOrganization determines which organization scopes the request.
// An admin may belong to several organizations; the session is scoped to their
// active organization. The stored active organization is validated against
// current membership on every request so that revoking a membership (or a stale
// active_organization_id) cannot leak access to an organization the admin no
// longer belongs to. It falls back to the home organization, then to any
// remaining membership.
//
// This runs on every authenticated request and gates the revoked-membership
// check, so a DB error must be propagated, not swallowed. Swallowing it would
// fail OPEN — defaulting to the home org and bypassing the membership check on
// a transient failure. Only a clean "not a member" result (no error) advances
// to the next fallback.
func resolveActiveOrganization(ctx context.Context, mgmt *management.State, admin *management.Admin) (uuid.UUID, error) {
	active := admin.OrganizationID
	if admin.ActiveOrganizationID != nil {
		active = *admin.ActiveOrganizationID
	}

	ok, err := mgmt.IsMember(ctx, active, admin.ID)
	if err != nil {
		return uuid.Nil, err
	}
	if ok {
		return active, nil
	}

	// The active org is stale (membership revoked). Try the home org, but only
	// if it differs from the active org we already checked.
	if admin.OrganizationID != active {
		ok, err := mgmt.IsMember(ctx, admin.OrganizationID, admin.ID)
		if err != nil {
			return uuid.Nil, err
		}
		if ok {
			return admin.OrganizationID, nil
		}
	}

	// Neither the active nor home org is a current membership; fall back to any
	// remaining membership so the admin can still reach an org they belong to.
	orgs, err := mgmt.ListOrganizationsForAdmin(ctx, admin.ID)
	if err != nil {
		return uuid.Nil, err
	}
	if len(orgs) > 0 {
		return orgs[0].ID, nil
	}

	// The admin belongs to no organization. Scope to the home org as a last
	// resort; org-scoped permission checks will deny access since there is no
	// membership tuple, so this does not leak access.
	return admin.OrganizationID, nil
}

// WithKey authenticates an API-key request for the given surface. API keys are
// private (backend-only) credentials, so on the client surface a key is rejected
// when the request is browser-originated (carries an Origin header); browser and
// mobile clients authenticate via a trusted issuer or a short-lived session
// instead. The management surface accepts any valid key.
//
// On the client surface the request URL names the project it acts on. A key is
// rejected (closed) when the URL project cannot be resolved or does not match the
// key's project, so a credential can never act on a project it is not scoped to.
func WithKey(mgmt *management.State, surface Surface) Handler {
	return func(ctx context.Context, tokenString string) (context.Context, error) {
		if tokenString == "" {
			return ctx, ErrUnauthorized
		}

		key, err := mgmt.GetAPIKeyBySecret(ctx, tokenString)
		if err != nil {
			return ctx, ErrUnauthorized
		}

		if err := enforceSurface(ctx, surface); err != nil {
			return ctx, err
		}

		if err := enforceURLProject(ctx, surface, key.ProjectID); err != nil {
			return ctx, err
		}

		actor := rbac.NewActor(
			rbac.ActorAPIKey,
			key.ID,
			rbac.WithOrganizationID(key.OrganizationID),
			rbac.WithProjectID(key.ProjectID),
		)

		return rbac.WithActor(ctx, actor), nil
	}
}

// enforceSurface rejects API-key auth where it must not be used: a private key
// presented to the browser-facing client API (a request carrying an Origin
// header). It returns ErrUnauthorized (rather than a distinct error) so callers
// cannot use the response to probe whether a given key exists.
func enforceSurface(ctx context.Context, surface Surface) error {
	if surface == SurfaceClient && browserOriginated(ctx) {
		return ErrUnauthorized
	}
	return nil
}

// browserOriginated reports whether the request carries an Origin header, which
// browsers attach to cross-origin (and most same-origin non-GET) requests.
//
// This is a best-effort, defense-in-depth signal that an honest browser is
// calling with a private key — not a security boundary. Browsers omit Origin on
// same-origin GETs, and a non-browser attacker holding a leaked key simply does
// not send the header. Private keys must be treated as secrets (rotate on
// exposure, never ship to a browser); this check only discourages accidental
// in-browser key use, it does not authenticate origin.
func browserOriginated(ctx context.Context) bool {
	r := RequestFromContext(ctx)
	return r != nil && r.Header.Get("Origin") != ""
}

// enforceURLProject binds a resolved credential to the project named in the
// request URL. On the client surface every route is mounted under
// /api/client/projects/{projectID}; the credential may act only on that project.
// It fails closed (ErrUnauthorized) when the URL project cannot be resolved or
// does not match the credential's project. The management surface carries no
// project in its URL, so the check is a no-op there.
func enforceURLProject(ctx context.Context, surface Surface, credentialProject uuid.UUID) error {
	if surface != SurfaceClient {
		return nil
	}
	urlProject, ok := projectFromRequest(ctx)
	if !ok || urlProject != credentialProject {
		return ErrUnauthorized
	}
	return nil
}

// projectFromRequest resolves the {projectID} path parameter of the in-flight
// request. Authentication runs inside the OpenAPI validator middleware, after chi
// has matched the route, so the chi route context carries the parameter; it falls
// back to scanning the URL path so the resolution is robust to middleware
// ordering. ok is false when no valid project UUID is present.
func projectFromRequest(ctx context.Context) (uuid.UUID, bool) {
	r := RequestFromContext(ctx)
	if r == nil {
		return uuid.Nil, false
	}
	raw := ""
	if rc := chi.RouteContext(r.Context()); rc != nil {
		raw = rc.URLParam("projectID")
	}
	if raw == "" {
		raw = projectIDFromPath(r.URL.Path)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// projectIDFromPath extracts the path segment following /api/client/projects/.
// It is the fallback for when the chi route context is not populated.
func projectIDFromPath(path string) string {
	const prefix = "/api/client/projects/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	rest := path[len(prefix):]
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return rest[:i]
	}
	return rest
}

// GetSession extracts the authentication token from the request.
// It checks (in priority order):
// 1. Session token from the HttpOnly '__session' cookie
// 2. Authorization header (with or without Bearer prefix)
//
// Only these two intakes are honoured. An 'oauth' cookie is deliberately NOT
// read: nothing in the platform ever writes one, so accepting it only offered
// script running on the origin a non-HttpOnly channel for outranking the real
// session cookie. Do not reintroduce it.
func GetSession(r *http.Request) string {
	if sessionCookie, err := r.Cookie("__session"); err == nil && sessionCookie.Value != "" {
		return sessionCookie.Value
	}

	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		// Remove "Bearer " prefix if present
		token := strings.TrimSpace(authHeader)
		token = strings.TrimPrefix(token, "Bearer ")
		return token
	}

	return ""
}

// SetSessionCookie stores the session token in a secure HTTP cookie. It sets
// the token directly as the "__session" cookie value. The cookie is configured
// with HttpOnly flag for security, and Secure flag is set based on whether the
// request was made over TLS. SameSite is set to Lax mode to prevent CSRF while
// allowing normal navigation.
func SetSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     "__session",
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// requestIsSecure reports whether the original client request reached us over
// HTTPS. In the common deployment a reverse proxy terminates TLS and forwards
// plaintext, so r.TLS is nil; we then trust X-Forwarded-Proto. Trusting that
// header can only ever cause the cookie to be marked Secure (never the reverse),
// so a spoofed value is harmless here.
func requestIsSecure(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
