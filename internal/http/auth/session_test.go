package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSigner builds a SessionSigner with a fresh EC P-256 key and the given
// issuer (blank uses the default).
func testSigner(t *testing.T, issuer string) *SessionSigner {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))

	signer, err := NewSessionSigner(pemKey, issuer)
	require.NoError(t, err)
	require.NotNil(t, signer)
	return signer
}

func TestSessionSigner(t *testing.T) {
	t.Parallel()
	methodID := uuid.New()

	t.Run("mints a verifiable ES256 token with the expected claims", func(t *testing.T) {
		signer := testSigner(t, "")
		token, expiresAt, err := signer.Mint(methodID, "user_123", time.Hour)
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now().Add(time.Hour), expiresAt, time.Minute)

		claims := jwt.MapClaims{}
		parsed, err := jwt.ParseWithClaims(token, claims,
			func(*jwt.Token) (any, error) { return &signer.key.PublicKey, nil },
			jwt.WithValidMethods([]string{"ES256"}),
			jwt.WithIssuer(defaultSessionIssuer),
			jwt.WithExpirationRequired(),
		)
		require.NoError(t, err)
		require.True(t, parsed.Valid)
		assert.Equal(t, "user_123", claimString(claims, "sub"))
		assert.Equal(t, methodID.String(), claimString(claims, sessionMethodClaim))
	})

	t.Run("defaults the issuer and honours an override", func(t *testing.T) {
		assert.Equal(t, defaultSessionIssuer, testSigner(t, "").issuer)
		assert.Equal(t, "https://acme.example", testSigner(t, "https://acme.example").issuer)
	})

	t.Run("an empty key disables sessions; a non-PEM key errors", func(t *testing.T) {
		s, err := NewSessionSigner("", "")
		require.NoError(t, err)
		assert.Nil(t, s)

		_, err = NewSessionSigner("not a pem key", "")
		assert.Error(t, err)
	})

	t.Run("a token signed by a different key does not verify", func(t *testing.T) {
		token, _, err := testSigner(t, "").Mint(methodID, "user_123", time.Hour)
		require.NoError(t, err)

		other := testSigner(t, "")
		claims := jwt.MapClaims{}
		_, err = jwt.ParseWithClaims(token, claims,
			func(*jwt.Token) (any, error) { return &other.key.PublicKey, nil },
			jwt.WithValidMethods([]string{"ES256"}), jwt.WithIssuer(defaultSessionIssuer))
		assert.Error(t, err)
	})

	t.Run("an expired token does not verify", func(t *testing.T) {
		signer := testSigner(t, "")
		token, _, err := signer.Mint(methodID, "user_123", -time.Hour)
		require.NoError(t, err)

		claims := jwt.MapClaims{}
		_, err = jwt.ParseWithClaims(token, claims,
			func(*jwt.Token) (any, error) { return &signer.key.PublicKey, nil },
			jwt.WithValidMethods([]string{"ES256"}), jwt.WithIssuer(defaultSessionIssuer), jwt.WithExpirationRequired())
		assert.Error(t, err)
	})

	t.Run("an HMAC token is rejected (algorithm pinning)", func(t *testing.T) {
		signer := testSigner(t, "")
		forged, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"iss": defaultSessionIssuer,
			"sub": "user_123",
			"exp": time.Now().Add(time.Hour).Unix(),
		}).SignedString([]byte("anything"))
		require.NoError(t, err)

		claims := jwt.MapClaims{}
		_, err = jwt.ParseWithClaims(forged, claims,
			func(*jwt.Token) (any, error) { return &signer.key.PublicKey, nil },
			jwt.WithValidMethods([]string{"ES256"}))
		assert.Error(t, err)
	})
}
