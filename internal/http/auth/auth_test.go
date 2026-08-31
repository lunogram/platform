package auth

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// TestConsoleSessionCookie asserts the wiring from requestIsSecure into the
// cookie: a proxied HTTPS request yields the `__Host-` prefixed Secure cookie,
// while a plain-HTTP request falls back to the unprefixed name (the `__Host-`
// prefix is only honoured in a secure context).
func TestConsoleSessionCookie(t *testing.T) {
	t.Parallel()

	secureCookie := func(setup func(*http.Request)) *http.Cookie {
		r := httptest.NewRequest(http.MethodPost, "http://example.com/login", nil)
		setup(r)
		w := httptest.NewRecorder()
		SetConsoleSessionCookie(w, r, "tok", time.Now().Add(time.Hour))
		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		return cookies[0]
	}

	t.Run("proxied https marks the cookie Secure", func(t *testing.T) {
		c := secureCookie(func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "https") })
		require.True(t, c.Secure)
		require.Equal(t, ConsoleCookieSecure, c.Name)
		require.True(t, c.HttpOnly)
		require.Equal(t, http.SameSiteLaxMode, c.SameSite)
	})

	t.Run("plain http falls back to the unprefixed name", func(t *testing.T) {
		c := secureCookie(func(*http.Request) {})
		require.False(t, c.Secure)
		require.Equal(t, ConsoleCookieInsecure, c.Name)
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

	t.Run("an oauth cookie cannot outrank the console session cookie", func(t *testing.T) {
		t.Parallel()

		r := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		r.AddCookie(&http.Cookie{Name: "oauth", Value: attacker})
		r.AddCookie(&http.Cookie{Name: ConsoleCookieInsecure, Value: "real-session-token"})

		assert.Equal(t, "real-session-token", GetSession(r))
	})

	t.Run("an oauth cookie cannot outrank the legacy session cookie", func(t *testing.T) {
		t.Parallel()

		r := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		r.AddCookie(&http.Cookie{Name: "oauth", Value: attacker})
		r.AddCookie(&http.Cookie{Name: LegacySessionCookie, Value: "legacy-session-token"})

		assert.Equal(t, "legacy-session-token", GetSession(r))
	})

	t.Run("an oauth cookie cannot outrank the Authorization header", func(t *testing.T) {
		t.Parallel()

		r := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
		r.AddCookie(&http.Cookie{Name: "oauth", Value: attacker})
		r.Header.Set("Authorization", "Bearer real-bearer-token")

		assert.Equal(t, "real-bearer-token", GetSession(r))
	})
}

// The `__Host-` prefix is what makes the binding cookie unwritable by a sibling
// subdomain. Accepting the unprefixed name as a fallback on a secure request
// would let anything on *.example.com plant a binding with Domain=... and hand
// the victim's browser the attacker's flow, which is the whole thing the
// binding exists to prevent.
func TestGetOIDCBindingReadsOnlyTheNameForThisScheme(t *testing.T) {
	t.Parallel()

	secure := func() *http.Request {
		r := httptest.NewRequest("GET", "/api/auth/oidc/default/callback", nil)
		r.Header.Set("X-Forwarded-Proto", "https")
		return r
	}

	planted := secure()
	planted.AddCookie(&http.Cookie{Name: OIDCBindingCookieInsecure, Value: "attacker-binding"})
	assert.Empty(t, GetOIDCBinding(planted),
		"a secure request must not fall back to the cookie a sibling subdomain can write")

	proper := secure()
	proper.AddCookie(&http.Cookie{Name: OIDCBindingCookieSecure, Value: "our-binding"})
	assert.Equal(t, "our-binding", GetOIDCBinding(proper))

	// Over plaintext the prefixed name is the one a browser would refuse, so
	// the unprefixed one is what gets read.
	insecure := httptest.NewRequest("GET", "/api/auth/oidc/default/callback", nil)
	insecure.AddCookie(&http.Cookie{Name: OIDCBindingCookieInsecure, Value: "dev-binding"})
	assert.Equal(t, "dev-binding", GetOIDCBinding(insecure))
}
