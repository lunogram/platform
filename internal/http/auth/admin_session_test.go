package auth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newECKeyPEM returns a fresh P-256 key as a SEC1 PEM block.
func newECKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))
}

func testConsoleConfig(t *testing.T) config.ConsoleAuth {
	t.Helper()
	return config.ConsoleAuth{
		SigningKey:  newECKeyPEM(t),
		Issuer:      "https://lunogram.test/console",
		Audience:    "lunogram-console",
		IdleTTL:     8 * time.Hour,
		AbsoluteTTL: 168 * time.Hour,
	}
}

func testConsoleSigner(t *testing.T) *ConsoleSigner {
	t.Helper()
	signer, err := NewConsoleSigner(testConsoleConfig(t))
	require.NoError(t, err)
	require.NotNil(t, signer)
	return signer
}

func testSession(t *testing.T) *management.AdminSession {
	t.Helper()
	return &management.AdminSession{
		ID:                uuid.New(),
		AdminID:           uuid.New(),
		ExpiresAt:         time.Now().Add(time.Hour),
		AbsoluteExpiresAt: time.Now().Add(24 * time.Hour),
		Refreshable:       true,
	}
}

// TestAdminSessionAlgorithmPinning is the regression test for the properties a
// console token must never be able to negotiate for itself. Each subtest forges
// a token that differs from a valid one in exactly one respect, and every one
// must be rejected.
func TestAdminSessionAlgorithmPinning(t *testing.T) {
	t.Parallel()

	signer := testConsoleSigner(t)
	session := testSession(t)

	validClaims := func() jwt.MapClaims {
		now := time.Now()
		return jwt.MapClaims{
			"iss": "https://lunogram.test/console",
			"aud": "lunogram-console",
			"sub": session.AdminID.String(),
			"sid": session.ID.String(),
			"iat": now.Unix(),
			"exp": now.Add(time.Hour).Unix(),
		}
	}

	t.Run("a token minted by this signer verifies", func(t *testing.T) {
		token, err := signer.Mint(session, []string{"clerk"})
		require.NoError(t, err)

		claims, err := signer.Verify(token)
		require.NoError(t, err)
		assert.Equal(t, session.AdminID, claims.AdminID)
		assert.Equal(t, session.ID, claims.SessionID)
		assert.Equal(t, []string{"clerk"}, claims.Methods)
		assert.False(t, claims.Impersonated())
	})

	t.Run("HS256 is rejected", func(t *testing.T) {
		// The classic algorithm-confusion attack: sign symmetrically and hope
		// the verifier hands its public key over as an HMAC secret. Method
		// pinning rejects the token before the keyfunc is ever consulted.
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims())
		token.Header["kid"] = signer.keyring.ActiveKID()
		signed, err := token.SignedString([]byte("not-the-key"))
		require.NoError(t, err)

		_, err = signer.Verify(signed)
		require.Error(t, err)
	})

	t.Run("alg none is rejected", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims())
		token.Header["kid"] = signer.keyring.ActiveKID()
		signed, err := token.SignedString(jwt.UnsafeAllowNoneSignatureType)
		require.NoError(t, err)

		_, err = signer.Verify(signed)
		require.Error(t, err)
	})

	t.Run("a token signed by a different key is rejected", func(t *testing.T) {
		other := testConsoleSigner(t)
		signed, err := other.Mint(session, nil)
		require.NoError(t, err)

		_, err = signer.Verify(signed)
		require.Error(t, err, "a foreign key must not verify even with the right claims")
	})

	t.Run("an unknown kid is rejected rather than tried against every key", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodES256, validClaims())
		token.Header["kid"] = "not-a-known-key"
		signed, err := token.SignedString(signer.keyring.active)
		require.NoError(t, err)

		_, err = signer.Verify(signed)
		require.Error(t, err)
	})

	t.Run("a token with no kid is rejected", func(t *testing.T) {
		signed, err := jwt.NewWithClaims(jwt.SigningMethodES256, validClaims()).SignedString(signer.keyring.active)
		require.NoError(t, err)

		_, err = signer.Verify(signed)
		require.Error(t, err)
	})

	forged := map[string]func(jwt.MapClaims){
		"wrong issuer":   func(c jwt.MapClaims) { c["iss"] = "https://attacker.example" },
		"wrong audience": func(c jwt.MapClaims) { c["aud"] = "lunogram-client" },
		"no audience":    func(c jwt.MapClaims) { delete(c, "aud") },
		"no expiry":      func(c jwt.MapClaims) { delete(c, "exp") },
		"expired":        func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() },
		"no session id":  func(c jwt.MapClaims) { delete(c, "sid") },
		"subject is not an admin id": func(c jwt.MapClaims) {
			c["sub"] = "user_2abc123def"
		},
	}

	for name, mutate := range forged {
		t.Run(name+" is rejected", func(t *testing.T) {
			claims := validClaims()
			mutate(claims)
			signed, err := signer.keyring.Sign(claims)
			require.NoError(t, err)

			_, err = signer.Verify(signed)
			require.Error(t, err)
		})
	}

	t.Run("a client session token is rejected", func(t *testing.T) {
		// The client and console signers hold DIFFERENT keys, so a client token
		// fails at the signature -- before a single claim is read. That is the
		// point of not sharing one key between the two populations.
		client := testSigner(t, "https://lunogram.test")
		token, _, err := client.Mint(uuid.New(), "end-user-subject", time.Hour)
		require.NoError(t, err)

		_, err = signer.Verify(token)
		require.Error(t, err)
	})

	t.Run("a console token is rejected by the client verifier", func(t *testing.T) {
		client := testSigner(t, "https://lunogram.test")
		token, err := signer.Mint(session, nil)
		require.NoError(t, err)

		parsed, err := jwt.Parse(token,
			func(*jwt.Token) (any, error) { return &client.key.PublicKey, nil },
			jwt.WithValidMethods([]string{"ES256"}),
		)
		require.Error(t, err)
		require.False(t, parsed.Valid)
	})
}

// TestAdminSessionImpersonationClaim covers the `act` claim, which is
// attribution only and whose absence must never fail a login.
func TestAdminSessionImpersonationClaim(t *testing.T) {
	t.Parallel()

	signer := testConsoleSigner(t)

	t.Run("an impersonated session carries act.sub", func(t *testing.T) {
		session := testSession(t)
		session.Impersonated = true
		subject := "user_support_engineer"
		session.ImpersonatorSubject = &subject

		token, err := signer.Mint(session, []string{"clerk"})
		require.NoError(t, err)

		claims, err := signer.Verify(token)
		require.NoError(t, err)
		assert.True(t, claims.Impersonated())
		assert.Equal(t, subject, claims.ImpersonatorSubject)
	})

	t.Run("a malformed act claim means not impersonated, not a failure", func(t *testing.T) {
		claims := jwt.MapClaims{
			"iss": "https://lunogram.test/console",
			"aud": "lunogram-console",
			"sub": uuid.New().String(),
			"sid": uuid.New().String(),
			"exp": time.Now().Add(time.Hour).Unix(),
			"act": "not-an-object",
		}
		signed, err := signer.keyring.Sign(claims)
		require.NoError(t, err)

		verified, err := signer.Verify(signed)
		require.NoError(t, err)
		assert.False(t, verified.Impersonated())
	})
}

// TestConsoleTokenCarriesNoAuthorization pins the decision that authorization
// inputs stay out of the bearer credential: a demoted admin must lose their old
// role at once, not when their token expires.
func TestConsoleTokenCarriesNoAuthorization(t *testing.T) {
	t.Parallel()

	signer := testConsoleSigner(t)
	token, err := signer.Mint(testSession(t), []string{"clerk"})
	require.NoError(t, err)

	claims := jwt.MapClaims{}
	_, _, err = jwt.NewParser().ParseUnverified(token, claims)
	require.NoError(t, err)

	for _, forbidden := range []string{"email", "role", "org", "organization", "organization_id", "permissions"} {
		assert.NotContains(t, claims, forbidden, "authorization input %q must not be cached in a bearer credential", forbidden)
	}
}

// TestConsoleSignerDisabled covers the "no key configured" case. The signer is
// nil rather than error, and it is the caller (startup) that decides whether
// that is a disabled feature or a fatal misconfiguration -- see
// [ErrConsoleSigningKeyMissing].
func TestConsoleSignerDisabled(t *testing.T) {
	t.Parallel()

	signer, err := NewConsoleSigner(config.ConsoleAuth{})
	require.NoError(t, err)
	assert.Nil(t, signer)
}

func TestKeyring(t *testing.T) {
	t.Parallel()

	t.Run("no active key disables the keyring", func(t *testing.T) {
		ring, err := NewKeyring("", nil)
		require.NoError(t, err)
		assert.Nil(t, ring)
	})

	t.Run("a retired key still verifies", func(t *testing.T) {
		retiring := newECKeyPEM(t)

		before, err := NewConsoleSigner(config.ConsoleAuth{
			SigningKey: retiring, Issuer: "https://lunogram.test/console",
			Audience: "lunogram-console", IdleTTL: time.Hour, AbsoluteTTL: time.Hour,
		})
		require.NoError(t, err)

		session := testSession(t)
		token, err := before.Mint(session, nil)
		require.NoError(t, err)

		// Rotate: a new active key, the old one kept for verification only.
		after, err := NewConsoleSigner(config.ConsoleAuth{
			SigningKey: newECKeyPEM(t), PreviousSigningKeys: []string{retiring},
			Issuer: "https://lunogram.test/console", Audience: "lunogram-console",
			IdleTTL: time.Hour, AbsoluteTTL: time.Hour,
		})
		require.NoError(t, err)

		claims, err := after.Verify(token)
		require.NoError(t, err, "a rotation must not log everyone out")
		assert.Equal(t, session.ID, claims.SessionID)

		// Once the retired key is dropped, its tokens stop verifying.
		without, err := NewConsoleSigner(config.ConsoleAuth{
			SigningKey: newECKeyPEM(t), Issuer: "https://lunogram.test/console",
			Audience: "lunogram-console", IdleTTL: time.Hour, AbsoluteTTL: time.Hour,
		})
		require.NoError(t, err)
		_, err = without.Verify(token)
		require.Error(t, err)
	})

	t.Run("key ids are derived, not configured", func(t *testing.T) {
		pemKey := newECKeyPEM(t)

		first, err := NewKeyring(pemKey, nil)
		require.NoError(t, err)
		second, err := NewKeyring(pemKey, nil)
		require.NoError(t, err)

		assert.Equal(t, first.ActiveKID(), second.ActiveKID(),
			"two replicas holding the same key must agree on its id without coordinating")
		assert.Len(t, first.ActiveKID(), keyIDLength)
	})

	t.Run("rejects a key that is not PEM", func(t *testing.T) {
		_, err := NewKeyring("not a pem block", nil)
		require.Error(t, err)
	})

	t.Run("accepts a key whose newlines are escaped", func(t *testing.T) {
		// How a multi-line PEM actually reaches the process from a compose file,
		// a Kubernetes env var, or a PaaS dashboard.
		pemKey := newECKeyPEM(t)
		escaped := strings.ReplaceAll(strings.TrimSpace(pemKey), "\n", `\n`)

		ring, err := NewKeyring(escaped, nil)
		require.NoError(t, err)
		require.NotNil(t, ring)

		direct, err := NewKeyring(pemKey, nil)
		require.NoError(t, err)
		assert.Equal(t, direct.ActiveKID(), ring.ActiveKID(), "the same key however it was transported")
	})
}
