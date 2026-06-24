package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/jwks"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// issuerDB is a fake store.DB that resolves a single trusted_issuer row by its
// issuer. Only GetContext is exercised by GetTrustedIssuer; every other method is
// left to the embedded nil interface (and would panic if called), which keeps the
// fake to the one query the middleware actually runs.
type issuerDB struct {
	store.DB
	method *management.TrustedIssuerAuthMethod // resolved row, or nil to force a lookup miss
}

func (d *issuerDB) GetContext(_ context.Context, dest any, _ string, args ...any) error {
	out, ok := dest.(*management.TrustedIssuerAuthMethod)
	if !ok {
		return errors.New("issuerDB: unexpected destination type")
	}
	if d.method == nil {
		return errors.New("sql: no rows in result set")
	}
	// GetTrustedIssuer queries by (project_id, issuer); the issuer is the string
	// argument (project_id is a uuid.UUID). Mirror WHERE t.issuer = $2 so an
	// unknown issuer misses like the real query.
	for _, a := range args {
		if issuer, ok := a.(string); ok && issuer != d.method.Issuer {
			return errors.New("sql: no rows in result set")
		}
	}
	*out = *d.method
	return nil
}

// newIssuerState builds a management.State whose trusted-issuer lookup resolves
// method (nil yields a store miss). Caching is disabled, so lookups read
// straight through to the fake DB.
func newIssuerState(method *management.TrustedIssuerAuthMethod) *management.State {
	return management.NewState(&issuerDB{method: method})
}

// issuerMethod returns a fully-populated trusted_issuer method (RSA PEM
// verification) plus its signing key, with an identity to assert the actor
// against.
func issuerMethod(t *testing.T) (*rsa.PrivateKey, *management.TrustedIssuerAuthMethod) {
	t.Helper()
	key, method := rsaIssuer(t)
	method.ID = uuid.New()
	method.OrganizationID = uuid.New()
	method.ProjectID = uuid.New()
	method.Role = rbac.ProjectClient
	method.SubjectScope = management.SubjectScopeOwn
	return key, method
}

func TestWithTrustedIssuer(t *testing.T) {
	t.Parallel()
	cache := jwks.New(jwks.Config{}, nil, nil, nil) // PEM path: never touches the network

	t.Run("rejects an empty token", func(t *testing.T) {
		_, method := issuerMethod(t)
		_, err := WithTrustedIssuer(newIssuerState(method), cache)(clientRequestCtx(method.ProjectID.String()), "")
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("rejects an unparseable token (issuer extraction fails)", func(t *testing.T) {
		_, method := issuerMethod(t)
		_, err := WithTrustedIssuer(newIssuerState(method), cache)(clientRequestCtx(method.ProjectID.String()), "not-a-jwt")
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("rejects a token with an empty issuer", func(t *testing.T) {
		key, method := issuerMethod(t)
		token := signRS256(t, key, jwt.MapClaims{
			"sub": "user_123",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		_, err := WithTrustedIssuer(newIssuerState(method), cache)(clientRequestCtx(method.ProjectID.String()), token)
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("rejects an unknown issuer (store lookup miss)", func(t *testing.T) {
		key, method := issuerMethod(t)
		token := signRS256(t, key, jwt.MapClaims{
			"iss": "https://stranger.example",
			"sub": "user_123",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		// State resolves only method.Issuer; the token's foreign issuer misses.
		_, err := WithTrustedIssuer(newIssuerState(method), cache)(clientRequestCtx(method.ProjectID.String()), token)
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	// Failures before the issuer resolves stay a generic ErrUnauthorized (above);
	// once the token resolves to a configured issuer, the middleware surfaces a
	// precise, debuggable reason instead so an integrator can fix their token.

	t.Run("rejects when verification fails (wrong key) with a debuggable reason", func(t *testing.T) {
		_, method := issuerMethod(t)
		other, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		token := signRS256(t, other, jwt.MapClaims{
			"iss": method.Issuer,
			"sub": "user_123",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		_, err = WithTrustedIssuer(newIssuerState(method), cache)(clientRequestCtx(method.ProjectID.String()), token)
		assert.NotErrorIs(t, err, ErrUnauthorized, "a resolved issuer surfaces a reason, not a bare unauthorized")
		assert.ErrorContains(t, err, "signature could not be verified")
	})

	t.Run("rejects a missing exp claim with a debuggable reason", func(t *testing.T) {
		key, method := issuerMethod(t)
		token := signRS256(t, key, jwt.MapClaims{
			"iss": method.Issuer,
			"sub": "user_123",
			// no "exp": WithExpirationRequired rejects it
		})
		_, err := WithTrustedIssuer(newIssuerState(method), cache)(clientRequestCtx(method.ProjectID.String()), token)
		assert.NotErrorIs(t, err, ErrUnauthorized)
		assert.ErrorContains(t, err, `"exp"`, "the reason names the missing claim")
	})

	t.Run("rejects an expired token with a debuggable reason", func(t *testing.T) {
		key, method := issuerMethod(t)
		token := signRS256(t, key, jwt.MapClaims{
			"iss": method.Issuer,
			"sub": "user_123",
			"exp": time.Now().Add(-time.Hour).Unix(),
		})
		_, err := WithTrustedIssuer(newIssuerState(method), cache)(clientRequestCtx(method.ProjectID.String()), token)
		assert.NotErrorIs(t, err, ErrUnauthorized)
		assert.ErrorContains(t, err, "expired")
	})

	t.Run("rejects an empty subject claim with a debuggable reason", func(t *testing.T) {
		key, method := issuerMethod(t)
		token := signRS256(t, key, jwt.MapClaims{
			"iss": method.Issuer,
			// no "sub": subject resolves to ""
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		_, err := WithTrustedIssuer(newIssuerState(method), cache)(clientRequestCtx(method.ProjectID.String()), token)
		assert.NotErrorIs(t, err, ErrUnauthorized)
		assert.ErrorContains(t, err, `"sub"`, "the reason names the missing subject claim")
	})

	t.Run("names a custom subject claim in the rejection reason", func(t *testing.T) {
		key, method := issuerMethod(t)
		method.SubjectClaim = "user_id"
		token := signRS256(t, key, jwt.MapClaims{
			"iss": method.Issuer,
			// no "user_id": the configured subject claim is absent
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		_, err := WithTrustedIssuer(newIssuerState(method), cache)(clientRequestCtx(method.ProjectID.String()), token)
		assert.NotErrorIs(t, err, ErrUnauthorized)
		assert.ErrorContains(t, err, `"user_id"`)
	})

	t.Run("builds an end-user actor on success", func(t *testing.T) {
		key, method := issuerMethod(t)
		token := signRS256(t, key, jwt.MapClaims{
			"iss": method.Issuer,
			"sub": "user_123",
			"exp": time.Now().Add(time.Hour).Unix(),
		})

		newCtx, err := WithTrustedIssuer(newIssuerState(method), cache)(clientRequestCtx(method.ProjectID.String()), token)
		require.NoError(t, err)

		actor := rbac.FromContext(newCtx)
		require.NotNil(t, actor)
		assert.Equal(t, rbac.ActorEndUser, actor.Type)
		assert.Equal(t, method.ID.String(), actor.ID)
		assert.Equal(t, method.OrganizationID, actor.OrganizationID)
		assert.Equal(t, method.ProjectID, actor.ProjectID)
		assert.Equal(t, "user_123", actor.Subject, "verified subject is wired onto the actor")
		assert.Equal(t, method.Issuer, actor.SubjectSource, "subject source is the trusted issuer")
		assert.Equal(t, rbac.DataScopeOwn, actor.Scope, "subject scope maps to the data scope")
	})

	t.Run("maps the all subject scope", func(t *testing.T) {
		key, method := issuerMethod(t)
		method.SubjectScope = management.SubjectScopeAll
		token := signRS256(t, key, jwt.MapClaims{
			"iss": method.Issuer,
			"sub": "user_123",
			"exp": time.Now().Add(time.Hour).Unix(),
		})

		newCtx, err := WithTrustedIssuer(newIssuerState(method), cache)(clientRequestCtx(method.ProjectID.String()), token)
		require.NoError(t, err)
		actor := rbac.FromContext(newCtx)
		require.NotNil(t, actor)
		assert.Equal(t, rbac.DataScopeAll, actor.Scope)
	})

	t.Run("extracts a custom subject claim onto the actor", func(t *testing.T) {
		key, method := issuerMethod(t)
		method.SubjectClaim = "user_id"
		token := signRS256(t, key, jwt.MapClaims{
			"iss":     method.Issuer,
			"user_id": "ext-999",
			"exp":     time.Now().Add(time.Hour).Unix(),
		})

		newCtx, err := WithTrustedIssuer(newIssuerState(method), cache)(clientRequestCtx(method.ProjectID.String()), token)
		require.NoError(t, err)
		actor := rbac.FromContext(newCtx)
		require.NotNil(t, actor)
		assert.Equal(t, "ext-999", actor.Subject)
	})
}

// jwksFromKey builds a JWKS document advertising key's public key under kid.
func jwksFromKey(t *testing.T, key *rsa.PrivateKey, kid string) []byte {
	t.Helper()
	jwk, err := jwkset.NewJWKFromKey(key.Public(), jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{KID: kid},
	})
	require.NoError(t, err)

	storage := jwkset.NewMemoryStorage()
	require.NoError(t, storage.KeyWrite(context.Background(), jwk))
	raw, err := storage.JSONPublic(context.Background())
	require.NoError(t, err)
	return raw
}

func signRS256WithKID(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(key)
	require.NoError(t, err)
	return signed
}

// rotatingFetcher serves the pre-rotation JWKS on the first fetch and the
// post-rotation set on every fetch after that. It models an issuer that rotates
// in a new signing key only observable on a subsequent (forced) refresh.
type rotatingFetcher struct {
	mu      sync.Mutex
	before  []byte
	after   []byte
	fetched bool
}

func (f *rotatingFetcher) Fetch(context.Context, string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.fetched {
		f.fetched = true
		return f.before, nil
	}
	return f.after, nil
}

func TestVerifyTrustedIssuerTokenRefreshesOnUnknownKID(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	oldKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	newKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// The cache initially knows only the old key; the issuer has rotated in a new
	// one and will only advertise it after a forced refresh.
	fetcher := &rotatingFetcher{
		before: jwksFromKey(t, oldKey, "old"),
		after:  jwksFromKey(t, newKey, "new"),
	}
	cache := jwks.New(jwks.Config{}, nil, fetcher, nil)

	jwksURL := "https://idp.test/.well-known/jwks.json"
	method := &management.TrustedIssuerAuthMethod{
		Issuer:       testIssuer,
		JWKSURL:      ptr.To(jwksURL),
		SubjectClaim: "sub",
	}

	// Warm the cache with the pre-rotation key set so the first parse fails on the
	// unknown "new" kid, forcing the refresh-and-retry branch.
	_, err = cache.Keyfunc(ctx, jwksURL)
	require.NoError(t, err)

	token := signRS256WithKID(t, newKey, "new", jwt.MapClaims{
		"iss": testIssuer,
		"sub": "rotated_user",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	claims, err := verifyTrustedIssuerToken(ctx, cache, method, token)
	require.NoError(t, err, "a refresh should pick up the rotated-in key")
	assert.Equal(t, "rotated_user", claimString(claims, method.SubjectClaim))
}

func TestVerifyTrustedIssuerTokenRejectsAlgNone(t *testing.T) {
	t.Parallel()
	_, method := rsaIssuer(t)
	cache := jwks.New(jwks.Config{}, nil, nil, nil)

	// "alg":"none" is an unsigned token; the parser must reject it because only
	// asymmetric algorithms are accepted for trusted issuers.
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"iss": testIssuer,
		"sub": "user_123",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)

	_, err = verifyTrustedIssuerToken(context.Background(), cache, method, signed)
	assert.Error(t, err, "alg:none must be rejected")
}

func TestParsePublicKeyPEM(t *testing.T) {
	t.Parallel()

	t.Run("parses a PKIX public key", func(t *testing.T) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		require.NoError(t, err)
		der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
		require.NoError(t, err)
		pemData := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))

		pub, err := parsePublicKeyPEM(pemData)
		require.NoError(t, err)
		assert.IsType(t, &rsa.PublicKey{}, pub)
	})

	t.Run("parses a certificate and returns its public key", func(t *testing.T) {
		certPEM, key := selfSignedCertPEM(t)
		pub, err := parsePublicKeyPEM(certPEM)
		require.NoError(t, err)
		rsaPub, ok := pub.(*rsa.PublicKey)
		require.True(t, ok)
		assert.Equal(t, key.PublicKey.N, rsaPub.N)
	})

	t.Run("rejects input that is not PEM", func(t *testing.T) {
		_, err := parsePublicKeyPEM("definitely not pem")
		assert.Error(t, err)
	})

	t.Run("rejects a PEM block that is neither PKIX nor a certificate", func(t *testing.T) {
		junk := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte("garbage")}))
		_, err := parsePublicKeyPEM(junk)
		assert.Error(t, err)
	})
}

// selfSignedCertPEM returns a self-signed certificate in PEM form and the RSA
// key that signed it.
func selfSignedCertPEM(t *testing.T) (string, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), key
}
