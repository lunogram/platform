package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
)

// ErrUnauthorized is returned when the authentication fails.
var ErrUnauthorized = errors.New("unauthorized")

// CrossOriginTitle is the problem title [ErrCrossOrigin] carries. It is a wire
// contract: the console matches on it to tell a rejected origin apart from an
// ordinary permission denial, so that the one failure a user cannot act on
// without being told about it says so on screen. See
// console/src/oapi/client.ts.
const CrossOriginTitle = "cross-origin request refused"

// ErrCrossOrigin is returned when a cookie-borne write arrives from an origin
// that is not the console's own. Unlike [ErrUnauthorized] it does not hand the
// request to the next verifier: a browser write refused for its provenance is
// refused, not offered a second credential to try.
var ErrCrossOrigin = problem.ErrorFunc(problem.WithStatus(problem.NewError(CrossOriginTitle,
	"this request did not come from the console itself, so it was refused. "+
		"if you are running a self-hosted deployment, PUBLIC_URL must match the address the console is served from"),
	http.StatusForbidden))

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

// Console session cookie names. The console cookie deliberately is NOT
// `__session`: Clerk's browser SDK owns that name on our own origin and
// rewrites it on every token refresh, so a token minted into it would be
// overwritten out from under us.
//
// `__Host-` is not decoration. The prefix is browser-enforced: the cookie is
// only accepted when it is Secure, Path=/ and carries no Domain attribute,
// which means a sibling subdomain cannot set it. That guarantee is unavailable
// over plain HTTP, so local development falls back to the unprefixed name.
const (
	ConsoleCookieSecure   = "__Host-lunogram_session"
	ConsoleCookieInsecure = "lunogram_session"

	// LegacySessionCookie is the cookie name the Clerk SDK writes. It is still
	// READ so that admins holding one at deploy time are not logged out; see
	// [UpgradeLegacySession]. Nothing writes it any more.
	LegacySessionCookie = "__session"
)

// consoleCookieName picks the cookie name the browser will actually accept for
// this request: the `__Host-` prefixed one requires a secure context.
func consoleCookieName(r *http.Request) string {
	if requestIsSecure(r) {
		return ConsoleCookieSecure
	}
	return ConsoleCookieInsecure
}

// GetSession extracts the authentication credential from the request, in
// priority order: the console session cookie, the legacy Clerk cookie, then the
// Authorization header.
//
// There is deliberately no `oauth` cookie intake. It used to sit at the HIGHEST
// precedence here while nothing in the platform ever wrote it, which made it a
// cookie-forcing hazard: anything able to set a cookie on the origin could
// override the credential every other path had already agreed on.
func GetSession(r *http.Request) string {
	if token, ok := cookieCredential(r); ok {
		return token
	}

	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		token := strings.TrimSpace(authHeader)
		token = strings.TrimPrefix(token, "Bearer ")
		return token
	}

	return ""
}

// cookieCredential returns the credential carried by a cookie, and whether one
// was present at all. The second result is what lets [WithAdminSession] apply
// CSRF hardening only to cookie-borne credentials -- an Authorization header is
// not attached by the browser automatically and so is not forgeable across
// origins in the same way.
func cookieCredential(r *http.Request) (string, bool) {
	for _, name := range []string{ConsoleCookieSecure, ConsoleCookieInsecure, LegacySessionCookie} {
		if cookie, err := r.Cookie(name); err == nil && cookie.Value != "" {
			return cookie.Value, true
		}
	}
	return "", false
}

// SetConsoleSessionCookie stores a minted console session token. HttpOnly keeps
// it out of reach of page scripts, SameSite=Lax keeps it off cross-site
// sub-requests while still surviving ordinary navigation, and Secure follows
// [requestIsSecure].
func SetConsoleSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     consoleCookieName(r),
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearConsoleSessionCookies expires every cookie name a credential could have
// arrived in, including the legacy one. Logging out must not depend on guessing
// which name the browser happens to hold.
func ClearConsoleSessionCookies(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{ConsoleCookieSecure, ConsoleCookieInsecure, LegacySessionCookie} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   requestIsSecure(r),
			SameSite: http.SameSiteLaxMode,
		})
	}
}

// OIDC binding cookie names. The `__Host-` prefixed one is used wherever the
// browser will accept it, exactly as the session cookie is.
const (
	OIDCBindingCookieSecure   = "__Host-lunogram_oidc_binding"
	OIDCBindingCookieInsecure = "lunogram_oidc_binding"
)

// SetOIDCBindingCookie ties an authorization request to the browser that
// started it. The value's twin is held server-side with the flow, and the
// callback refuses a response the two do not agree on.
//
// SameSite=Lax rather than Strict: the browser arrives back from the identity
// provider by top-level navigation, which Lax allows and Strict does not, and
// the cookie would then be missing from exactly the request that needs it.
//
// It is refreshed rather than replaced -- see [BrowserBinding] -- so that two
// logins started in one browser do not invalidate each other.
func SetOIDCBindingCookie(w http.ResponseWriter, r *http.Request, binding string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     oidcBindingCookieName(r),
		Value:    binding,
		Path:     "/",
		Expires:  time.Now().Add(ttl),
		HttpOnly: true,
		Secure:   requestIsSecure(r),
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearOIDCBindingCookie expires both names. It is NOT called when a login
// completes: a browser may have more than one flow outstanding -- two tabs, or a
// retry after a back button -- and they share one binding, so clearing it on the
// first callback would make the others uncompletable. The cookie lapses with the
// flows it belongs to instead.
func ClearOIDCBindingCookie(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{OIDCBindingCookieSecure, OIDCBindingCookieInsecure} {
		http.SetCookie(w, &http.Cookie{
			Name:     name,
			Value:    "",
			Path:     "/",
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
			HttpOnly: true,
			Secure:   requestIsSecure(r),
			SameSite: http.SameSiteLaxMode,
		})
	}
}

// GetOIDCBinding returns the browser binding presented on a callback, or "".
//
// Only the name this request's scheme would have been given is read, and the
// unprefixed one is deliberately NOT accepted as a fallback on a secure
// request. `__Host-` is what makes the cookie unwritable by a sibling
// subdomain; reading the unprefixed name too would let anything on
// *.example.com plant a binding of its own choosing and hand the victim's
// browser the attacker's flow, which is the whole thing the binding prevents.
func GetOIDCBinding(r *http.Request) string {
	cookie, err := r.Cookie(oidcBindingCookieName(r))
	if err != nil || cookie.Value == "" {
		return ""
	}
	return cookie.Value
}

// BrowserBinding returns the binding to record against a new flow: the one this
// browser already carries, or a fresh one when it carries none.
//
// Reusing it is what lets a browser hold several flows at once. A value per
// flow would mean the second /start overwrites the first, and the first
// callback -- perfectly legitimate, from the same person, in the same browser --
// would then be refused as coming from somewhere else.
//
// Sharing costs nothing the per-flow value was buying. The binding answers "is
// this the browser that started the login", which is a property of the browser,
// not of the flow; what makes a response unrepeatable is the state, which is
// single-use server-side.
func BrowserBinding(r *http.Request, mint func() (string, error)) (string, error) {
	if existing := GetOIDCBinding(r); existing != "" {
		return existing, nil
	}
	return mint()
}

func oidcBindingCookieName(r *http.Request) string {
	if requestIsSecure(r) {
		return OIDCBindingCookieSecure
	}
	return OIDCBindingCookieInsecure
}

// SAMLBindingCookie ties a SAML authentication request to the browser that
// started it, as [OIDCBindingCookieSecure] does for OpenID Connect.
//
// There is only the `__Host-` name and no plaintext fallback, because this
// cookie has to be SameSite=None and browsers refuse a SameSite=None cookie
// that is not Secure. A deployment whose public URL is not https is refused the
// SAML driver at boot rather than handed a binding that silently never arrives.
const SAMLBindingCookie = "__Host-lunogram_saml_binding"

// SetSAMLBindingCookie stores the browser's half of the binding.
//
// SameSite=None, where the OpenID Connect twin is Lax, and the difference is
// forced by the protocol rather than chosen. A browser returns from an OpenID
// Connect provider by GET, which Lax allows. It returns from a SAML identity
// provider by the HTTP-POST binding -- a cross-site top-level form POST, which
// Lax does NOT send cookies on. A Lax cookie here would be missing from every
// assertion this deployment ever received, so the binding would be dead code
// that failed open or refused every login depending on how it was read.
//
// Secure is set unconditionally rather than from [requestIsSecure]. The cookie
// is invalid without it and the deployment is already known to be https, so a
// reverse proxy that forwards no X-Forwarded-Proto must not be able to turn the
// binding off.
func SetSAMLBindingCookie(w http.ResponseWriter, binding string, ttl time.Duration) {
	http.SetCookie(w, &http.Cookie{
		Name:     SAMLBindingCookie,
		Value:    binding,
		Path:     "/",
		Expires:  time.Now().Add(ttl),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	})
}

// GetSAMLBinding returns the browser binding presented at the assertion
// consumer service, or "".
func GetSAMLBinding(r *http.Request) string {
	cookie, err := r.Cookie(SAMLBindingCookie)
	if err != nil || cookie.Value == "" {
		return ""
	}
	return cookie.Value
}

// SAMLBrowserBinding returns the binding to record against a new flow: the one
// this browser already carries, or a fresh one. It is shared across a browser's
// outstanding flows for the reason [BrowserBinding] is.
func SAMLBrowserBinding(r *http.Request, mint func() (string, error)) (string, error) {
	if existing := GetSAMLBinding(r); existing != "" {
		return existing, nil
	}
	return mint()
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
