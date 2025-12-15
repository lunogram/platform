package claim

import (
	"context"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

type JWKS struct {
	keyfunc jwt.Keyfunc
}

func (jwks *JWKS) UnmarshalText(server []byte) error {
	store, err := keyfunc.NewDefault([]string{string(server)})
	if err != nil {
		return err
	}

	jwks.keyfunc = store.Keyfunc
	return nil
}

func (jwks JWKS) Unwrap() jwt.Keyfunc {
	return jwks.keyfunc
}

// Session represents the authenticated session from a JWT token
type Session struct {
	jwt.RegisteredClaims
}

type contextKey string

const sessionKey contextKey = "session"

// WithSession stores the session object in the context
func WithSession(ctx context.Context, session Session) context.Context {
	return context.WithValue(ctx, sessionKey, session)
}

// FromContext retrieves the session object from the context
func FromContext(ctx context.Context) (Session, bool) {
	session, ok := ctx.Value(sessionKey).(Session)
	return session, ok
}
