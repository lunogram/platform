package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lunogram/platform/services/nexus/internal/claim"
	"github.com/lunogram/platform/services/nexus/internal/claim/rbac"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/store"
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

// Middleware is a middleware function that authenticates requests by verifying the
// authorization header. It returns a AuthenticationFunc that checks the authorization
// header and adds the authenticated session to the request context.
func Middleware(middleware ...Handler) openapi3filter.AuthenticationFunc {
	return func(ctx context.Context, filter *openapi3filter.AuthenticationInput) error {
		req := filter.RequestValidationInput.Request
		tokenString := RetrieveAuthToken(req)

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

func WithJWT(config config.Auth, stores *store.State) Handler {
	keyFunc := config.JWKS.Unwrap()
	if config.JWTSecret != "" {
		keyFunc = HMAC([]byte(config.JWTSecret))
	}

	return func(ctx context.Context, tokenString string) (context.Context, error) {
		claims := jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(tokenString, &claims, keyFunc, jwt.WithValidMethods([]string{"RS256", "HS256"}))
		if err != nil {
			return ctx, ErrUnauthorized
		}

		if !token.Valid {
			return ctx, ErrUnauthorized
		}

		session := claim.Session{
			RegisteredClaims: claims,
		}

		admin, err := stores.GetAdminBySubject(ctx, session)
		if err != nil {
			return ctx, ErrUnauthorized
		}

		ctx = claim.WithSession(ctx, session)
		return rbac.WithScope(ctx, &rbac.Scope{
			OrganizationID: admin.OrganizationID,
		}), nil
	}
}

func WithKey(stores *store.State) Handler {
	return func(ctx context.Context, tokenString string) (context.Context, error) {
		if tokenString == "" {
			return ctx, ErrUnauthorized
		}

		key, err := stores.GetAPIKeyBySecret(tokenString)
		if err != nil {
			return ctx, ErrUnauthorized
		}

		ctx = rbac.WithScope(ctx, &rbac.Scope{
			OrganizationID: key.OrganizationID,
		})

		return ctx, nil
	}
}

// ParseTokenClaims parses a JWT token and returns its claims without full validation
// This is useful when the token has already been validated and we just need to extract claims
func ParseTokenClaims(tokenString string) (*TokenClaims, error) {
	parser := jwt.NewParser()
	var claims TokenClaims
	_, _, err := parser.ParseUnverified(tokenString, &claims)
	if err != nil {
		return nil, err
	}

	return &claims, nil
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

// RetrieveAuthToken extracts the authentication token from the request.
// It checks (in priority order):
// 1. OAuth access token from 'oauth' cookie
// 2. Session token from '__session' cookie
// 3. Authorization header (with or without Bearer prefix)
func RetrieveAuthToken(r *http.Request) string {
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
