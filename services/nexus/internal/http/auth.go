package http

import (
	"context"
	"errors"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lunogram/platform/pkg/claim"
	"github.com/lunogram/platform/pkg/claim/rbac"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/http/auth"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"go.uber.org/zap"
)

func HMAC(secret []byte) jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		return secret, nil
	}
}

// Auth is a middleware function that authenticates requests by verifying the
// authorization header. It returns a AuthenticationFunc that checks the authorization
// header and adds the authenticated session to the request context.
func Auth(config config.Service, logger *zap.Logger, stores *store.Stores) openapi3filter.AuthenticationFunc {
	keyFunc := config.JWKS.Unwrap()
	if config.JWTSecret != "" {
		keyFunc = HMAC([]byte(config.JWTSecret))
	}

	return func(ctx context.Context, filter *openapi3filter.AuthenticationInput) error {
		// TODO: support other auth strategies
		req := filter.RequestValidationInput.Request
		tokenString := auth.RetrieveAuthToken(req)

		claims := jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(tokenString, &claims, keyFunc, jwt.WithValidMethods([]string{"RS256", "HS256"}))
		if err != nil {
			return err
		}

		if token.Valid {
			session := claim.Session{
				RegisteredClaims: claims,
			}

			ctx := claim.WithSession(req.Context(), session)

			// Load admin from database
			admin, err := stores.GetAdminBySubject(ctx, session)
			if err != nil {
				logger.Error("failed to get admin", zap.Error(err))
				return errors.New("unauthorized")
			}

			ctx = rbac.WithAdmin(ctx, &rbac.Admin{
				ID:             admin.ID,
				OrganizationID: admin.OrganizationID,
			})

			*req = *req.WithContext(ctx)
		}

		return nil
	}
}
