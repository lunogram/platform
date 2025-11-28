package http

import (
	"context"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/http/auth"
)

func HMAC(secret []byte) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	}
}

// Auth is a middleware function that authenticates requests by verifying the
// authorization header. It returns a AuthenticationFunc that checks the authorization
// header and adds the authenticated context to the request context.
func Auth(config config.Service) openapi3filter.AuthenticationFunc {
	keyFunc := config.JWKS.Unwrap()
	if config.JWTSecret != "" {
		keyFunc = HMAC([]byte(config.JWTSecret))
	}

	return func(ctx context.Context, filter *openapi3filter.AuthenticationInput) error {
		// TODO: support other auth strategies
		token := auth.RetrieveAuthToken(filter.RequestValidationInput.Request)
		_, err := jwt.Parse(token, keyFunc, jwt.WithValidMethods([]string{"RS256", "HS256"}))
		return err
	}
}
