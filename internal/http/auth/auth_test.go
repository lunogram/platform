package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lunogram/platform/internal/claim"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/rbac"
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

// strongSecret is an HS256 signing secret that passes [validateJWTSecret]:
// private to the test, and comfortably over the minimum length.
const strongSecret = "R4TQ8yb1ZK0mfPuLd93sXvHnE7WqAjCt"

// clerkConfig returns an Auth config for the clerk driver (RS256 verified
// against a JWKS serving rsaKey's public half).
func clerkConfig(t *testing.T, rsaKey *rsa.PrivateKey) config.Auth {
	t.Helper()

	url := jwksServer(t, rsaKey, "kid-1")
	var jwks claim.JWKS
	require.NoError(t, jwks.UnmarshalText([]byte(url)))
	return config.Auth{Driver: "clerk", JWKS: jwks}
}

// basicConfig returns an Auth config for the basic driver (HS256 verified
// against the local signing secret).
func basicConfig(secret string) config.Auth {
	return config.Auth{Driver: "basic", JWTSecret: secret}
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

// TestWithJWTDriverBoundAlgorithms is the regression test for the admin-session
// forgery this release fixes. The accepted signature algorithms follow the
// CONFIGURED DRIVER, never the mere presence of a JWT secret:
//
//   - clerk  -> RS256 verified against the provider JWKS, nothing else
//   - basic  -> HS256 verified against the local signing secret, nothing else
//
// Previously the HS256 branch was enabled by any non-empty AUTH_JWT_SECRET,
// including on a clerk deployment that never issues HS256 tokens. A secret the
// attacker knows (the placeholder that shipped as a docker-compose default was
// published in the repository) was therefore enough to mint an admin session,
// because HS256 was a fully trusted verification path regardless of driver.
func TestWithJWTDriverBoundAlgorithms(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	mgmt := management.NewState(mgmtDB)

	orgID, err := mgmt.CreateOrganization(ctx, "Target Org")
	require.NoError(t, err)
	adminID, err := mgmt.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "target-admin@example.com",
		Role:           "owner",
	})
	require.NoError(t, err)
	require.NoError(t, mgmt.AddMember(ctx, orgID, adminID, "owner"))

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	// Claims naming a real admin — exactly what an attacker would forge. The
	// subject is that admin's UUID, so everything downstream of signature
	// verification resolves and authorizes them.
	adminClaims := func() jwt.MapClaims {
		return jwt.MapClaims{
			"sub": adminID.String(),
			"exp": time.Now().Add(time.Hour).Unix(),
		}
	}

	// The forgery: an HS256 token signed with the deployment's JWT secret.
	forged := signHS256(t, []byte(strongSecret), adminClaims())

	t.Run("control: the forged token authenticates where HS256 is the driver's algorithm", func(t *testing.T) {
		t.Parallel()
		// Without this control the rejections below would prove nothing — a
		// malformed token would fail the same way. On a basic-driver deployment
		// this byte-identical token is a legitimate session and authenticates as
		// the admin it names.
		handler, err := WithJWT(basicConfig(strongSecret), mgmt)
		require.NoError(t, err)

		authed, err := handler(ctx, forged)
		require.NoError(t, err, "an HS256 token signed with the basic driver's secret is a valid session")

		actor := rbac.FromContext(authed)
		require.NotNil(t, actor)
		assert.Equal(t, adminID.String(), actor.ID, "the token authenticates as the admin named in sub")
	})

	t.Run("clerk driver rejects that same token even though the secret signing it is configured", func(t *testing.T) {
		t.Parallel()
		// THIS IS THE VULNERABILITY. The deployment authenticates through Clerk
		// but also carries AUTH_JWT_SECRET (a leftover, or the published
		// docker-compose default that everyone can read). The attacker knows the
		// secret and signs their own admin token with it.
		cfg := clerkConfig(t, rsaKey)
		cfg.JWTSecret = strongSecret

		handler, err := WithJWT(cfg, mgmt)
		require.NoError(t, err)

		_, err = handler(ctx, forged)
		require.ErrorIs(t, err, ErrUnauthorized,
			"a clerk deployment must accept RS256/JWKS only; a configured JWT secret must not enable an HS256 verification path")
	})

	t.Run("clerk driver rejects an HS256 token when no secret is configured", func(t *testing.T) {
		t.Parallel()
		handler, err := WithJWT(clerkConfig(t, rsaKey), mgmt)
		require.NoError(t, err)

		_, err = handler(ctx, forged)
		require.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("basic driver rejects an RS256 token", func(t *testing.T) {
		t.Parallel()
		// The mirror image: an HS256 deployment must not accept an asymmetric
		// token, whoever signed it.
		handler, err := WithJWT(basicConfig(strongSecret), mgmt)
		require.NoError(t, err)

		_, err = handler(ctx, signRS256Key(t, rsaKey, "kid-1", adminClaims()))
		require.ErrorIs(t, err, ErrUnauthorized, "RS256 token must be rejected under the basic driver")
	})

	t.Run("basic driver rejects a token signed with the wrong secret", func(t *testing.T) {
		t.Parallel()
		handler, err := WithJWT(basicConfig(strongSecret), mgmt)
		require.NoError(t, err)

		_, err = handler(ctx, signHS256(t, []byte("a-different-secret-of-sufficient-length"), adminClaims()))
		require.ErrorIs(t, err, ErrUnauthorized, "a forged-secret HS256 token must be rejected")
	})

	t.Run("garbage token is rejected", func(t *testing.T) {
		t.Parallel()
		handler, err := WithJWT(basicConfig(strongSecret), mgmt)
		require.NoError(t, err)

		_, err = handler(ctx, "not-a-jwt")
		require.ErrorIs(t, err, ErrUnauthorized)
	})
}

// TestWithJWTRejectsWeakSecret pins the fail-fast check on the admin signing
// secret. A deployment whose driver issues HS256 tokens must refuse to START on
// a secret that cannot keep those tokens private, rather than serving requests
// and accepting forgeries until someone notices. The published docker-compose
// placeholder is called out by value: it is public knowledge, so length alone
// would not disqualify it.
func TestWithJWTRejectsWeakSecret(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		secret  string
		wantErr bool
	}{
		"empty secret": {
			secret:  "",
			wantErr: true,
		},
		"the default published in the repository": {
			secret:  "dev-secret-change-in-production",
			wantErr: true,
		},
		"too short to resist offline guessing": {
			secret:  "short-secret",
			wantErr: true,
		},
		"one byte under the minimum": {
			secret:  strings.Repeat("k", minJWTSecretBytes-1),
			wantErr: true,
		},
		"exactly the minimum length": {
			secret:  strings.Repeat("k", minJWTSecretBytes),
			wantErr: false,
		},
		"a strong secret": {
			secret:  strongSecret,
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, err := WithJWT(basicConfig(tc.secret), nil)
			if !tc.wantErr {
				require.NoError(t, err)
				require.NotNil(t, handler)
				return
			}

			require.ErrorIs(t, err, ErrInsecureJWTSecret)
			require.Nil(t, handler, "no handler may be returned for a secret that cannot be trusted")
			assert.Contains(t, err.Error(), "AUTH_JWT_SECRET", "the error must name the variable to fix")
			assert.Contains(t, err.Error(), "openssl rand", "the error must show how to generate a good value")
		})
	}
}

// TestWithJWTSecretIsIrrelevantToClerk asserts the other half of the driver
// binding at construction: a clerk deployment neither needs nor is weakened by
// AUTH_JWT_SECRET, so a weak one must not block startup — it simply is not key
// material for that driver.
func TestWithJWTSecretIsIrrelevantToClerk(t *testing.T) {
	t.Parallel()

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	cfg := clerkConfig(t, rsaKey)
	cfg.JWTSecret = "dev-secret-change-in-production"

	handler, err := WithJWT(cfg, nil)
	require.NoError(t, err)
	require.NotNil(t, handler)
}

// TestWithJWTUnknownDriver asserts construction fails closed on a driver it
// cannot map to an algorithm, instead of silently defaulting to some accepted
// set.
func TestWithJWTUnknownDriver(t *testing.T) {
	t.Parallel()

	for _, driver := range []string{"", "oauth2", "none"} {
		t.Run("driver "+driver, func(t *testing.T) {
			t.Parallel()

			handler, err := WithJWT(config.Auth{Driver: driver, JWTSecret: strongSecret}, nil)
			require.Error(t, err)
			require.Nil(t, handler)
		})
	}
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
