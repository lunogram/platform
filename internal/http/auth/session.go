package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
)

const (
	// sessionIssuer is the `iss` of Lunogram-minted session tokens.
	sessionIssuer = "lunogram-session"
	// sessionMethodClaim carries the session auth method (policy) id, which
	// defines the authorization the session confers.
	sessionMethodClaim = "amid"
)

// MintSession creates a signed, short-lived session token for the given subject
// under a session policy (auth method). The token's authorization derives from
// the policy; the subject identifies the end user. signingKey must be non-empty.
func MintSession(signingKey string, methodID uuid.UUID, subject string, ttl time.Duration) (token string, expiresAt time.Time, err error) {
	if signingKey == "" {
		return "", time.Time{}, errors.New("auth: session signing key is not configured")
	}

	now := time.Now()
	expiresAt = now.Add(ttl)
	claims := jwt.MapClaims{
		"iss":              sessionIssuer,
		"sub":              subject,
		sessionMethodClaim: methodID.String(),
		"iat":              now.Unix(),
		"exp":              expiresAt.Unix(),
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(signingKey))
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, expiresAt, nil
}

// SessionSubjectSource is the external-ID source for a session's subject. Like
// trusted issuers, session subjects are namespaced by their policy so they do
// not collide with other identifier sources.
func SessionSubjectSource(methodID uuid.UUID) string {
	return "session:" + methodID.String()
}

// WithSession authenticates a Lunogram-minted session token: it verifies the
// HMAC signature and standard claims, resolves the session policy the token was
// minted under, and builds an end-user actor scoped by that policy and carrying
// the token's subject. It declines when no signing key is configured.
func WithSession(mgmt *management.State, signingKey string) Handler {
	return func(ctx context.Context, tokenString string) (context.Context, error) {
		if tokenString == "" || signingKey == "" {
			return ctx, ErrUnauthorized
		}

		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims,
			func(*jwt.Token) (any, error) { return []byte(signingKey), nil },
			jwt.WithValidMethods([]string{"HS256"}),
			jwt.WithIssuer(sessionIssuer),
			jwt.WithExpirationRequired(),
		)
		if err != nil || !token.Valid {
			return ctx, ErrUnauthorized
		}

		methodID, err := uuid.Parse(claimString(claims, sessionMethodClaim))
		if err != nil {
			return ctx, ErrUnauthorized
		}
		subject := claimString(claims, "sub")
		if subject == "" {
			return ctx, ErrUnauthorized
		}

		method, err := mgmt.GetSessionAuthMethod(methodID)
		if err != nil {
			return ctx, ErrUnauthorized
		}

		actor := rbac.NewActor(
			rbac.ActorEndUser,
			method.ID.String(),
			rbac.WithOrganizationID(method.OrganizationID),
			rbac.WithProjectID(method.ProjectID),
			rbac.WithSubject(subject, SessionSubjectSource(method.ID)),
			rbac.WithScope(rbac.DataScope(method.SubjectScope)),
		)
		return rbac.WithActor(ctx, actor), nil
	}
}
