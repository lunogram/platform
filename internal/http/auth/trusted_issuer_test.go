package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lunogram/platform/internal/jwks"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testIssuer = "https://idp.test"

// rsaIssuer generates an RSA key and a trusted_issuer method that verifies
// tokens with that key's public PEM.
func rsaIssuer(t *testing.T) (*rsa.PrivateKey, *management.TrustedIssuerAuthMethod) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})

	return key, &management.TrustedIssuerAuthMethod{
		Issuer:       testIssuer,
		PublicCert:   ptr.To(string(pubPEM)),
		SubjectClaim: "sub",
	}
}

func signRS256(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodRS256, claims).SignedString(key)
	require.NoError(t, err)
	return signed
}

func TestVerifyTrustedIssuerToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cache := jwks.New(jwks.Config{}, nil, nil, nil) // unused for the PEM path

	t.Run("accepts a valid token and returns claims", func(t *testing.T) {
		key, method := rsaIssuer(t)
		token := signRS256(t, key, jwt.MapClaims{
			"iss": testIssuer,
			"sub": "user_123",
			"exp": time.Now().Add(time.Hour).Unix(),
		})

		claims, err := verifyTrustedIssuerToken(ctx, cache, method, token)
		require.NoError(t, err)
		assert.Equal(t, "user_123", claimString(claims, method.SubjectClaim))
	})

	t.Run("rejects an expired token", func(t *testing.T) {
		key, method := rsaIssuer(t)
		token := signRS256(t, key, jwt.MapClaims{
			"iss": testIssuer,
			"sub": "user_123",
			"exp": time.Now().Add(-time.Hour).Unix(),
		})
		_, err := verifyTrustedIssuerToken(ctx, cache, method, token)
		assert.Error(t, err)
	})

	t.Run("rejects a mismatched issuer", func(t *testing.T) {
		key, method := rsaIssuer(t)
		token := signRS256(t, key, jwt.MapClaims{
			"iss": "https://evil.example",
			"sub": "user_123",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		_, err := verifyTrustedIssuerToken(ctx, cache, method, token)
		assert.Error(t, err)
	})

	t.Run("rejects a token signed by a different key", func(t *testing.T) {
		_, method := rsaIssuer(t)
		other, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		token := signRS256(t, other, jwt.MapClaims{
			"iss": testIssuer,
			"sub": "user_123",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		_, err = verifyTrustedIssuerToken(ctx, cache, method, token)
		assert.Error(t, err)
	})

	t.Run("rejects an HMAC-signed token (algorithm pinning)", func(t *testing.T) {
		_, method := rsaIssuer(t)
		signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"iss": testIssuer,
			"sub": "user_123",
			"exp": time.Now().Add(time.Hour).Unix(),
		}).SignedString([]byte("shared-secret"))
		require.NoError(t, err)
		_, err = verifyTrustedIssuerToken(ctx, cache, method, signed)
		assert.Error(t, err, "HMAC must be rejected for trusted issuers")
	})

	t.Run("enforces audience when configured", func(t *testing.T) {
		key, method := rsaIssuer(t)
		method.Audience = ptr.To("lunogram")

		ok := signRS256(t, key, jwt.MapClaims{"iss": testIssuer, "sub": "u", "aud": "lunogram", "exp": time.Now().Add(time.Hour).Unix()})
		_, err := verifyTrustedIssuerToken(ctx, cache, method, ok)
		require.NoError(t, err)

		bad := signRS256(t, key, jwt.MapClaims{"iss": testIssuer, "sub": "u", "aud": "someone-else", "exp": time.Now().Add(time.Hour).Unix()})
		_, err = verifyTrustedIssuerToken(ctx, cache, method, bad)
		assert.Error(t, err)
	})

	t.Run("extracts a custom subject claim", func(t *testing.T) {
		key, method := rsaIssuer(t)
		method.SubjectClaim = "user_id"
		token := signRS256(t, key, jwt.MapClaims{
			"iss":     testIssuer,
			"user_id": "ext-999",
			"exp":     time.Now().Add(time.Hour).Unix(),
		})
		claims, err := verifyTrustedIssuerToken(ctx, cache, method, token)
		require.NoError(t, err)
		assert.Equal(t, "ext-999", claimString(claims, method.SubjectClaim))
	})
}

func TestUnverifiedIssuer(t *testing.T) {
	t.Parallel()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	token := signRS256(t, key, jwt.MapClaims{"iss": testIssuer, "sub": "u", "exp": time.Now().Add(time.Hour).Unix()})

	iss, err := unverifiedIssuer(token)
	require.NoError(t, err)
	assert.Equal(t, testIssuer, iss)
}
