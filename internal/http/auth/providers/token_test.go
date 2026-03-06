package providers

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestHMACJWTGeneratorGenerate(t *testing.T) {
	t.Parallel()

	type test struct {
		secret    string
		tokenLife time.Duration
		adminID   uuid.UUID
	}

	tests := map[string]test{
		"basic token generation": {
			secret:    "test-secret-key",
			tokenLife: time.Hour,
			adminID:   uuid.New(),
		},
		"short token life": {
			secret:    "short-secret",
			tokenLife: 5 * time.Minute,
			adminID:   uuid.New(),
		},
		"long token life": {
			secret:    "long-secret",
			tokenLife: 7 * 24 * time.Hour,
			adminID:   uuid.New(),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			generator := NewHMACJWTGenerator(tc.secret, tc.tokenLife)

			token, expiresAt, err := generator.Generate(tc.adminID)
			require.NoError(t, err)
			require.NotEmpty(t, token)
			require.True(t, expiresAt.After(time.Now()))

			parsedToken, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, func(t *jwt.Token) (any, error) {
				return []byte(tc.secret), nil
			})
			require.NoError(t, err)
			require.True(t, parsedToken.Valid)

			claims, ok := parsedToken.Claims.(*jwt.RegisteredClaims)
			require.True(t, ok)

			require.Equal(t, tc.adminID.String(), claims.Subject)
			require.NotNil(t, claims.ExpiresAt)
			require.NotNil(t, claims.IssuedAt)
			require.NotNil(t, claims.NotBefore)

			expectedExpiry := time.Now().Add(tc.tokenLife)
			require.WithinDuration(t, expectedExpiry, expiresAt, 2*time.Second)
		})
	}
}

func TestHMACJWTGeneratorTokenLife(t *testing.T) {
	t.Parallel()

	type test struct {
		inputLife    time.Duration
		expectedLife time.Duration
	}

	tests := map[string]test{
		"custom token life": {
			inputLife:    2 * time.Hour,
			expectedLife: 2 * time.Hour,
		},
		"one minute": {
			inputLife:    time.Minute,
			expectedLife: time.Minute,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			generator := NewHMACJWTGenerator("secret", tc.inputLife)
			require.Equal(t, tc.expectedLife, generator.TokenLife())
		})
	}
}

func TestHMACJWTGeneratorTokenValidation(t *testing.T) {
	t.Parallel()

	secret := "test-secret-key"
	generator := NewHMACJWTGenerator(secret, time.Hour)
	adminID := uuid.New()

	token, _, err := generator.Generate(adminID)
	require.NoError(t, err)

	t.Run("valid token with correct secret", func(t *testing.T) {
		parsedToken, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
			return []byte(secret), nil
		})
		require.NoError(t, err)
		require.True(t, parsedToken.Valid)
	})

	t.Run("invalid token with wrong secret", func(t *testing.T) {
		_, err := jwt.Parse(token, func(t *jwt.Token) (any, error) {
			return []byte("wrong-secret"), nil
		})
		require.Error(t, err)
	})
}

func TestHMACJWTGeneratorKeyfunc(t *testing.T) {
	t.Parallel()

	secret := "test-secret"
	generator := NewHMACJWTGenerator(secret, time.Hour)
	adminID := uuid.New()

	token, _, err := generator.Generate(adminID)
	require.NoError(t, err)

	parsedToken, err := jwt.Parse(token, generator.Keyfunc())
	require.NoError(t, err)
	require.True(t, parsedToken.Valid)
}

func TestHMACJWTGeneratorMultipleTokens(t *testing.T) {
	t.Parallel()

	generator := NewHMACJWTGenerator("secret", time.Hour)

	adminID1 := uuid.New()
	adminID2 := uuid.New()

	token1, _, err := generator.Generate(adminID1)
	require.NoError(t, err)

	token2, _, err := generator.Generate(adminID2)
	require.NoError(t, err)

	require.NotEqual(t, token1, token2)

	for _, token := range []string{token1, token2} {
		parsedToken, err := jwt.Parse(token, generator.Keyfunc())
		require.NoError(t, err)
		require.True(t, parsedToken.Valid)
	}
}

func TestHMACJWTGeneratorClaimsTimestamps(t *testing.T) {
	t.Parallel()

	generator := NewHMACJWTGenerator("secret", time.Hour)
	adminID := uuid.New()

	beforeGeneration := time.Now()
	token, expiresAt, err := generator.Generate(adminID)

	require.NoError(t, err)

	parsedToken, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, generator.Keyfunc())
	require.NoError(t, err)

	claims := parsedToken.Claims.(*jwt.RegisteredClaims)

	require.WithinDuration(t, beforeGeneration, claims.IssuedAt.Time, 2*time.Second)
	require.Equal(t, claims.IssuedAt.Time, claims.NotBefore.Time)
	require.Equal(t, expiresAt.Unix(), claims.ExpiresAt.Unix())
}
