package verifiers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lunogram/platform/internal/claim"
	"github.com/lunogram/platform/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// testECKey returns a fresh P-256 key as a SEC1 PEM block.
func testECKey(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
}

// clerkJWKS returns a config.JWKS pointing at a server that publishes a fresh
// RSA public key, which is what an AUTH_JWKS_URL resolves to in production.
func clerkJWKS(t *testing.T) claim.JWKS {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	document, err := json.Marshal(map[string]any{"keys": []map[string]string{{
		"kty": "RSA",
		"alg": "RS256",
		"use": "sig",
		"kid": "kid-1",
		"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01}),
	}}})
	require.NoError(t, err)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(document)
	}))
	t.Cleanup(server.Close)

	var jwks claim.JWKS
	require.NoError(t, jwks.UnmarshalText([]byte(server.URL)))
	return jwks
}

func TestNewVerifier(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)

	t.Run("basic stays a supported driver", func(t *testing.T) {
		// It is the documented quickstart (AUTH_BASIC_EMAIL=admin@localhost),
		// so it must keep working without being gated on anything.
		verifier, err := New(BasicDriver, config.Auth{}, nil, logger, nil)
		require.NoError(t, err)
		assert.Equal(t, BasicDriver, verifier.Driver())
	})

	t.Run("password", func(t *testing.T) {
		verifier, err := New(PasswordDriver, config.Auth{}, nil, logger, nil)
		require.NoError(t, err)
		assert.Equal(t, PasswordDriver, verifier.Driver())
	})

	t.Run("clerk", func(t *testing.T) {
		verifier, err := New(ClerkDriver, config.Auth{
			JWKS:  clerkJWKS(t),
			Clerk: config.ClerkAuth{SecretKey: "sk_test_xxx"},
		}, nil, logger, nil)
		require.NoError(t, err)
		assert.Equal(t, ClerkDriver, verifier.Driver())
	})

	t.Run("clerk without a JWKS is refused at startup", func(t *testing.T) {
		// Starting would leave the verifier with no key at all: every login
		// would fail closed but silently, looking like a broken Clerk instance
		// rather than an unset variable.
		_, err := New(config.Auth{
			Driver: "clerk",
			Clerk:  config.ClerkAuth{SecretKey: "sk_test_xxx"},
		}, nil, logger, nil)
		require.ErrorIs(t, err, ErrMissingJWKS)
		assert.Contains(t, err.Error(), "AUTH_JWKS_URL")
	})

	t.Run("an unknown driver is refused at startup", func(t *testing.T) {
		_, err := New("magic", config.Auth{}, nil, logger, nil)
		require.ErrorIs(t, err, ErrUnknownDriver)
	})
}

// A deployment may run several drivers at once -- passwords alongside SSO while
// an organization migrates -- so the set is built from configuration rather than
// collapsed to whichever one came first.
func TestBuildVerifiers(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)

	t.Run("builds every configured driver", func(t *testing.T) {
		built, err := Build(config.Auth{
			Drivers: []string{"password", " CLERK "},
			Clerk:   config.ClerkAuth{SecretKey: "sk_test_xxx"},
		}, nil, logger, nil)
		require.NoError(t, err)
		require.Len(t, built, 2)
		assert.Equal(t, PasswordDriver, built[PasswordDriver].Driver())
		assert.Equal(t, ClerkDriver, built[ClerkDriver].Driver())
	})

	t.Run("no configured driver builds nothing", func(t *testing.T) {
		built, err := Build(config.Auth{}, nil, logger, nil)
		require.NoError(t, err)
		assert.Empty(t, built)
	})

	t.Run("one unknown driver fails the whole build", func(t *testing.T) {
		// Startup is the only moment a typo in AUTH_DRIVER can be caught; a
		// deployment silently offering fewer login methods than it was told to
		// is how people get locked out.
		_, err := Build(config.Auth{Drivers: []string{"basic", "magic"}}, nil, logger, nil)
		require.ErrorIs(t, err, ErrUnknownDriver)
	})
}
