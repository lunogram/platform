package verifiers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"testing"

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

func TestNewVerifier(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)

	t.Run("basic stays a supported driver", func(t *testing.T) {
		// It is the documented quickstart (AUTH_BASIC_EMAIL=admin@localhost),
		// so it must keep working without being gated on anything.
		verifier, err := New(config.Auth{Driver: "basic"}, nil, logger, nil)
		require.NoError(t, err)
		assert.Equal(t, "basic", verifier.Driver())
	})

	t.Run("clerk", func(t *testing.T) {
		verifier, err := New(config.Auth{
			Driver: "clerk",
			Clerk:  config.ClerkAuth{SecretKey: "sk_test_xxx"},
		}, nil, logger, nil)
		require.NoError(t, err)
		assert.Equal(t, "clerk", verifier.Driver())
	})

	t.Run("an unknown driver is refused at startup", func(t *testing.T) {
		_, err := New(config.Auth{Driver: "magic"}, nil, logger, nil)
		require.ErrorIs(t, err, ErrUnknownDriver)
	})
}
