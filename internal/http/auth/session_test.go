package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sessionKey = "test-session-signing-key"

// parseSession verifies a session token the same way WithSession does (minus
// the store lookup), returning its claims.
func parseSession(t *testing.T, token string) jwt.MapClaims {
	t.Helper()
	claims := jwt.MapClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims,
		func(*jwt.Token) (any, error) { return []byte(sessionKey), nil },
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(sessionIssuer),
		jwt.WithExpirationRequired(),
	)
	require.NoError(t, err)
	require.True(t, parsed.Valid)
	return claims
}

func TestMintSession(t *testing.T) {
	t.Parallel()
	methodID := uuid.New()

	t.Run("mints a verifiable token with the expected claims", func(t *testing.T) {
		token, expiresAt, err := MintSession(sessionKey, methodID, "user_123", time.Hour)
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now().Add(time.Hour), expiresAt, time.Minute)

		claims := parseSession(t, token)
		assert.Equal(t, "user_123", claimString(claims, "sub"))
		assert.Equal(t, methodID.String(), claimString(claims, sessionMethodClaim))
	})

	t.Run("requires a signing key", func(t *testing.T) {
		_, _, err := MintSession("", methodID, "user_123", time.Hour)
		assert.Error(t, err)
	})

	t.Run("a token signed with a different key does not verify", func(t *testing.T) {
		token, _, err := MintSession("another-key", methodID, "user_123", time.Hour)
		require.NoError(t, err)

		claims := jwt.MapClaims{}
		_, err = jwt.ParseWithClaims(token, claims,
			func(*jwt.Token) (any, error) { return []byte(sessionKey), nil },
			jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(sessionIssuer))
		assert.Error(t, err)
	})

	t.Run("an expired token does not verify", func(t *testing.T) {
		token, _, err := MintSession(sessionKey, methodID, "user_123", -time.Hour)
		require.NoError(t, err)

		claims := jwt.MapClaims{}
		_, err = jwt.ParseWithClaims(token, claims,
			func(*jwt.Token) (any, error) { return []byte(sessionKey), nil },
			jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(sessionIssuer), jwt.WithExpirationRequired())
		assert.Error(t, err)
	})
}
