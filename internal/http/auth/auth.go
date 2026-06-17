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

func HMAC(secret []byte) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		return secret, nil
	}
}

// multiKeyfunc returns a keyfunc that dispatches to jwks for RS256 tokens and
// hmac for HS256 tokens, allowing both Clerk (RS256) and basic auth (HS256) to
// coexist when both are configured.
func multiKeyfunc(jwks, hmac jwt.Keyfunc) jwt.Keyfunc {
	return func(token *jwt.Token) (any, error) {
		switch token.Method.(type) {
		case *jwt.SigningMethodRSA:
			if jwks != nil {
				return jwks(token)
			}
		case *jwt.SigningMethodHMAC:
			if hmac != nil {
				return hmac(token)
			}
		}
		return nil, jwt.ErrTokenSignatureInvalid
	}
}

type Handler func(ctx context.Context, token string) (context.Context, error)

// Middleware is a middleware function that authenticates requests by verifying the
// authorization header. It returns a AuthenticationFunc that checks the authorization
// header and adds the authenticated session to the request context.
func Middleware(middleware ...Handler) openapi3filter.AuthenticationFunc {
	return func(ctx context.Context, filter *openapi3filter.AuthenticationInput) error {
		req := filter.RequestValidationInput.Request
		tokenString := GetSession(req)

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
	var hmacFunc jwt.Keyfunc
	if config.JWTSecret != "" {
		hmacFunc = HMAC([]byte(config.JWTSecret))
	}
	keyFunc := multiKeyfunc(config.JWKS.Unwrap(), hmacFunc)

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
			rbac.WithOrganizationID(resolveActiveOrganization(ctx, mgmt, admin)),
		)

		return rbac.WithActor(ctx, actor), nil
	}
}

// resolveActiveOrganization determines which organization scopes the request.
// An admin may belong to several organizations; the session is scoped to their
// active organization. The stored active organization is validated against
// current membership on every request so that revoking a membership (or a stale
// active_organization_id) cannot leak access to an organization the admin no
// longer belongs to. It falls back to the home organization, then to any
// remaining membership.
func resolveActiveOrganization(ctx context.Context, mgmt *management.State, admin *management.Admin) uuid.UUID {
	active := admin.OrganizationID
	if admin.ActiveOrganizationID != nil {
		active = *admin.ActiveOrganizationID
	}

	if ok, err := mgmt.IsMember(ctx, active, admin.ID); err == nil && ok {
		return active
	}

	if ok, err := mgmt.IsMember(ctx, admin.OrganizationID, admin.ID); err == nil && ok {
		return admin.OrganizationID
	}

	if orgs, err := mgmt.ListOrganizationsForAdmin(ctx, admin.ID); err == nil && len(orgs) > 0 {
		return orgs[0].ID
	}

	return admin.OrganizationID
}

func WithKey(mgmt *management.State) Handler {
	return func(ctx context.Context, tokenString string) (context.Context, error) {
		if tokenString == "" {
			return ctx, ErrUnauthorized
		}

		key, err := mgmt.GetAPIKeyBySecret(tokenString)
		if err != nil {
			return ctx, ErrUnauthorized
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
