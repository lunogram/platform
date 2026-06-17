package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/golang-jwt/jwt/v5"
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

func HMAC(secret []byte) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		return secret, nil
	}
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
		tokenString := GetSession(req)
		ctx = withRequest(ctx, req)

		for _, m := range middleware {
			ctx, err := m(ctx, tokenString)
			if err == nil {
				*req = *req.WithContext(ctx)
				return nil
			}

			if errors.Is(err, ErrUnauthorized) {
				continue
			}

			return err
		}

		return ErrUnauthorized
	}
}

func WithJWT(config config.Auth, mgmt *management.State) Handler {
	keyFunc := config.JWKS.Unwrap()
	if config.JWTSecret != "" {
		keyFunc = HMAC([]byte(config.JWTSecret))
	}

	return func(ctx context.Context, value string) (context.Context, error) {
		claims := jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(value, &claims, keyFunc, jwt.WithValidMethods([]string{"RS256", "HS256"}))
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

		actor := rbac.NewActor(
			rbac.ActorAdmin,
			admin.ID.String(),
			rbac.WithOrganizationID(admin.OrganizationID),
		)

		return rbac.WithActor(ctx, actor), nil
	}
}

// Scope values recorded on an API key. A missing scope is treated as secret
// (the conservative default for keys created before scopes were enforced).
const (
	ScopePublic = "public"
	ScopeSecret = "secret"
)

// WithKey authenticates an API-key request for the given surface and enforces
// the key's scope:
//
//   - public-scoped keys are rejected on the management surface — they exist
//     only for the client API;
//   - secret-scoped keys are rejected on the client surface when the request is
//     browser-originated (carries an Origin header), since secret keys must
//     never be embedded in client-side code.
func WithKey(mgmt *management.State, surface Surface) Handler {
	return func(ctx context.Context, tokenString string) (context.Context, error) {
		if tokenString == "" {
			return ctx, ErrUnauthorized
		}

		key, err := mgmt.GetAPIKeyBySecret(tokenString)
		if err != nil {
			return ctx, ErrUnauthorized
		}

		if err := enforceScope(ctx, surface, key); err != nil {
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

// enforceScope applies the per-surface scope rules. It returns ErrUnauthorized
// (rather than a distinct error) so callers cannot use the response to probe
// whether a given key exists.
func enforceScope(ctx context.Context, surface Surface, key *management.APIKey) error {
	scope := ScopeSecret
	if key.Scope != nil && *key.Scope != "" {
		scope = *key.Scope
	}

	switch surface {
	case SurfaceManagement:
		if scope == ScopePublic {
			return ErrUnauthorized
		}
	case SurfaceClient:
		if scope == ScopeSecret && browserOriginated(ctx) {
			return ErrUnauthorized
		}
	}

	return nil
}

// browserOriginated reports whether the request carries an Origin header, which
// browsers attach to cross-origin (and most same-origin non-GET) requests.
func browserOriginated(ctx context.Context) bool {
	r := RequestFromContext(ctx)
	return r != nil && r.Header.Get("Origin") != ""
}

// OAuthResponse represents the OAuth token response stored in cookies
type OAuthResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// getCookieOAuthToken retrieves and parses the OAuth token from the 'oauth' cookie
func getCookieOAuthToken(r *http.Request) *OAuthResponse {
	cookie, err := r.Cookie("oauth")
	if err != nil {
		return nil
	}

	decoded, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil
	}

	var res OAuthResponse
	if err := json.Unmarshal(decoded, &res); err != nil {
		return nil
	}

	return &res
}

// GetSession extracts the authentication token from the request.
// It checks (in priority order):
// 1. OAuth access token from 'oauth' cookie
// 2. Session token from '__session' cookie
// 3. Authorization header (with or without Bearer prefix)
func GetSession(r *http.Request) string {
	if oauthToken := getCookieOAuthToken(r); oauthToken != nil && oauthToken.AccessToken != "" {
		return oauthToken.AccessToken
	}

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
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}
