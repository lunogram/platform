package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lunogram/platform/internal/claim"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
)

// TestResolveActiveOrganization exercises the active-organization resolution
// that scopes every authenticated request. The security-critical behaviour is
// that a stale (revoked) active organization falls back to a current membership
// and that a real DB error fails CLOSED rather than silently defaulting to the
// home org.
func TestResolveActiveOrganization(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	mgmt := management.NewState(mgmtDB)

	homeOrg, err := mgmt.CreateOrganization(ctx, "Home Org")
	require.NoError(t, err)
	otherOrg, err := mgmt.CreateOrganization(ctx, "Other Org")
	require.NoError(t, err)

	adminID, err := mgmt.CreateAdmin(ctx, management.Admin{
		OrganizationID: homeOrg,
		Email:          "switcher@example.com",
		Role:           "owner",
	})
	require.NoError(t, err)

	// The admin is a member of both organizations.
	require.NoError(t, mgmt.AddMember(ctx, homeOrg, adminID, "owner"))
	require.NoError(t, mgmt.AddMember(ctx, otherOrg, adminID, "member"))

	t.Run("valid active membership is used", func(t *testing.T) {
		require.NoError(t, mgmt.SetActiveOrganization(ctx, adminID, otherOrg))

		admin, err := mgmt.GetAdmin(ctx, adminID)
		require.NoError(t, err)

		got, err := resolveActiveOrganization(ctx, mgmt, admin)
		require.NoError(t, err)
		assert.Equal(t, otherOrg, got, "should scope to the active organization the admin still belongs to")
	})

	t.Run("stale active org falls back to home org", func(t *testing.T) {
		require.NoError(t, mgmt.SetActiveOrganization(ctx, adminID, otherOrg))
		// Revoke the membership the active org points at.
		require.NoError(t, mgmt.RemoveMember(ctx, otherOrg, adminID))

		admin, err := mgmt.GetAdmin(ctx, adminID)
		require.NoError(t, err)

		got, err := resolveActiveOrganization(ctx, mgmt, admin)
		require.NoError(t, err)
		assert.Equal(t, homeOrg, got, "a revoked active org must not leak access; fall back to a current membership")

		// restore for later subtests
		require.NoError(t, mgmt.AddMember(ctx, otherOrg, adminID, "member"))
	})

	t.Run("DB error does not fail open", func(t *testing.T) {
		admin, err := mgmt.GetAdmin(ctx, adminID)
		require.NoError(t, err)

		// Force the membership query to fail by handing it a cancelled context.
		// The resolver must propagate the error rather than silently default to
		// the home org (which would bypass the revoked-membership check).
		cancelled, cancel := context.WithCancel(ctx)
		cancel()

		_, err = resolveActiveOrganization(cancelled, mgmt, admin)
		require.Error(t, err, "a DB error must propagate, not silently default to the home org")
	})
}

// jwksServer serves a JWKS document containing rsaKey's public half and returns
// its URL. It is used to populate a config.JWKS (RS256/JWKS verification mode).
func jwksServer(t *testing.T, rsaKey *rsa.PrivateKey, kid string) string {
	t.Helper()

	jwk, err := jwkset.NewJWKFromKey(rsaKey.Public(), jwkset.JWKOptions{
		Metadata: jwkset.JWKMetadataOptions{KID: kid},
	})
	require.NoError(t, err)

	storage := jwkset.NewMemoryStorage()
	require.NoError(t, storage.KeyWrite(context.Background(), jwk))
	raw, err := storage.JSONPublic(context.Background())
	require.NoError(t, err)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(raw)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// jwksConfig returns an Auth config in JWKS/RS256 verification mode whose keys
// match rsaKey, plus that key's matching kid.
func jwksConfig(t *testing.T, rsaKey *rsa.PrivateKey) config.Auth {
	t.Helper()

	url := jwksServer(t, rsaKey, "kid-1")
	var jwks claim.JWKS
	require.NoError(t, jwks.UnmarshalText([]byte(url)))
	return config.Auth{JWKS: jwks}
}

func signHS256(t *testing.T, secret []byte, claims jwt.MapClaims) string {
	t.Helper()
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	require.NoError(t, err)
	return signed
}

func signRS256Key(t *testing.T, key *rsa.PrivateKey, kid string, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(key)
	require.NoError(t, err)
	return signed
}

// TestWithJWTAlgorithmPinning is the regression test for the alg-confusion
// hardening in WithJWT: each verification mode must accept only its own
// algorithm and reject the other, so an attacker cannot present an HS256 forgery
// to an RS256 verifier (or vice versa).
func TestWithJWTAlgorithmPinning(t *testing.T) {
	t.Parallel()

	const secret = "hmac-shared-secret"
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	claims := func() jwt.MapClaims {
		return jwt.MapClaims{
			"iss": "https://idp.example",
			"sub": "admin-1",
			"exp": time.Now().Add(time.Hour).Unix(),
		}
	}

	t.Run("RS256/JWKS mode rejects an HS256 token", func(t *testing.T) {
		t.Parallel()
		// In JWKS mode the verifier holds the RSA public key. The classic attack
		// signs an HS256 token (here even using the configured RSA key's identity)
		// hoping the public key is accepted as an HMAC secret. Method pinning
		// rejects it before the keyfunc is ever consulted.
		handler := WithJWT(jwksConfig(t, rsaKey), nil)
		token := signHS256(t, []byte(secret), claims())

		_, err := handler(context.Background(), token)
		require.ErrorIs(t, err, ErrUnauthorized, "HS256 token must be rejected in RS256/JWKS mode")
	})

	t.Run("HMAC mode rejects an RS256 token", func(t *testing.T) {
		t.Parallel()
		// In HMAC mode the verifier holds a shared secret. An RS256 token signed
		// with an RSA private key must be rejected by method pinning.
		handler := WithJWT(config.Auth{JWTSecret: secret}, nil)
		token := signRS256Key(t, rsaKey, "kid-1", claims())

		_, err := handler(context.Background(), token)
		require.ErrorIs(t, err, ErrUnauthorized, "RS256 token must be rejected in HMAC mode")
	})

	t.Run("HMAC mode rejects a token signed with the wrong secret", func(t *testing.T) {
		t.Parallel()
		handler := WithJWT(config.Auth{JWTSecret: secret}, nil)
		token := signHS256(t, []byte("a-different-secret"), claims())

		_, err := handler(context.Background(), token)
		require.ErrorIs(t, err, ErrUnauthorized, "a forged-secret HS256 token must be rejected")
	})

	t.Run("garbage token is rejected", func(t *testing.T) {
		t.Parallel()
		handler := WithJWT(config.Auth{JWTSecret: secret}, nil)
		_, err := handler(context.Background(), "not-a-jwt")
		require.ErrorIs(t, err, ErrUnauthorized)
	})
}

// TestGetSessionIgnoresOAuthCookie pins the removal of the "oauth" cookie
// intake. Nothing in the platform ever wrote that cookie, yet GetSession read
// it at the HIGHEST precedence — so anything able to set a (non-HttpOnly)
// cookie on the origin could outrank the real session. Do not reintroduce it.
func TestGetSessionIgnoresOAuthCookie(t *testing.T) {
	t.Parallel()

	// The shape the removed code accepted: base64 of an OAuth token response.
	attacker := base64.StdEncoding.EncodeToString([]byte(`{"access_token":"attacker-token"}`))

	t.Run("an oauth cookie alone yields no session", func(t *testing.T) {
		t.Parallel()

		r := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		r.AddCookie(&http.Cookie{Name: "oauth", Value: attacker})

		assert.Empty(t, GetSession(r), "the oauth cookie must not be a token intake")
	})

	t.Run("an oauth cookie cannot outrank the session cookie", func(t *testing.T) {
		t.Parallel()

		r := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		r.AddCookie(&http.Cookie{Name: "oauth", Value: attacker})
		r.AddCookie(&http.Cookie{Name: "__session", Value: "real-session-token"})

		assert.Equal(t, "real-session-token", GetSession(r))
	})

	t.Run("an oauth cookie cannot outrank the Authorization header", func(t *testing.T) {
		t.Parallel()

		r := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		r.AddCookie(&http.Cookie{Name: "oauth", Value: attacker})
		r.Header.Set("Authorization", "Bearer real-bearer-token")

		assert.Equal(t, "real-bearer-token", GetSession(r))
	})
}

// TestRequestIsSecure covers the cookie Secure-flag decision: HTTPS is detected
// from r.TLS directly, and behind a TLS-terminating proxy from the
// X-Forwarded-Proto header (case-insensitively).
func TestRequestIsSecure(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		setup func(*http.Request)
		want  bool
	}{
		"plain http, no forwarded header": {
			setup: func(*http.Request) {},
			want:  false,
		},
		"x-forwarded-proto https (proxy terminated TLS)": {
			setup: func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "https") },
			want:  true,
		},
		"x-forwarded-proto HTTPS uppercase": {
			setup: func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "HTTPS") },
			want:  true,
		},
		"x-forwarded-proto http stays insecure": {
			setup: func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "http") },
			want:  false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
			tc.setup(r)
			require.Equal(t, tc.want, requestIsSecure(r))
		})
	}
}

// TestSetSessionCookieSecure asserts the wiring from requestIsSecure into the
// cookie's Secure attribute: a proxied HTTPS request yields a Secure cookie,
// while a plain request does not.
func TestSetSessionCookieSecure(t *testing.T) {
	t.Parallel()

	secureCookie := func(setup func(*http.Request)) *http.Cookie {
		r := httptest.NewRequest(http.MethodPost, "http://example.com/login", nil)
		setup(r)
		w := httptest.NewRecorder()
		SetSessionCookie(w, r, "tok", time.Now().Add(time.Hour))
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		return cookies[0]
	}

	t.Run("proxied https marks the cookie Secure", func(t *testing.T) {
		c := secureCookie(func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "https") })
		require.True(t, c.Secure)
	})

	t.Run("plain http does not mark the cookie Secure", func(t *testing.T) {
		c := secureCookie(func(*http.Request) {})
		require.False(t, c.Secure)
	})
}
