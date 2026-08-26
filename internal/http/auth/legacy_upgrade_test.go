package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lunogram/platform/internal/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// legacyVerifier stands in for the upstream verifier: it accepts one opaque
// cookie value and proves the identity behind it.
type legacyVerifier struct {
	accept   string
	identity *VerifiedIdentity
	calls    int
}

func (v *legacyVerifier) Driver() string { return "clerk" }

func (v *legacyVerifier) Verify(_ context.Context, r *http.Request) (*VerifiedIdentity, error) {
	v.calls++
	if GetSession(r) != v.accept {
		return nil, ErrUnauthorized
	}
	return v.identity, nil
}

// TestUpgradeLegacySession is the "nobody gets logged out by this deploy" test.
// A browser arriving with only the upstream's `__session` cookie must come away
// with a console session AND a successful response to the very request that
// carried the old cookie.
func TestUpgradeLegacySession(t *testing.T) {
	t.Parallel()

	env := newExchangeEnv(t)
	logger := zaptest.NewLogger(t)
	ctx := context.Background()

	// An admin who was provisioned before this release: they exist, and their
	// upstream subject is the identity the legacy cookie proves.
	_, err := env.exchange(t, verifiedIdentity("user_established", "established@example.com"))
	require.NoError(t, err)

	verifier := &legacyVerifier{
		accept:   "legacy-cookie-value",
		identity: verifiedIdentity("user_established", "established@example.com"),
	}

	// The downstream handler authenticates exactly as the real stack does, so a
	// 200 here means the in-flight request really was upgraded rather than
	// merely queued behind a new cookie.
	authenticate := WithAdminSession(env.mgmt, env.signer, "", zaptest.NewLogger(t))
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := authenticate(withRequest(r.Context(), r), GetSession(r)); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	handler := UpgradeLegacySession(verifier, env.exchanger, env.signer, logger)(downstream)

	t.Run("a legacy cookie yields a console cookie and a 200 on the same request", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/admin/profile", nil)
		r.AddCookie(&http.Cookie{Name: LegacySessionCookie, Value: "legacy-cookie-value"})

		handler.ServeHTTP(w, r)

		require.Equal(t, http.StatusOK, w.Code, "the request that carried the legacy cookie must succeed")

		cookies := w.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, ConsoleCookieInsecure, cookies[0].Name)

		claims, err := env.signer.Verify(cookies[0].Value)
		require.NoError(t, err)

		session, err := env.mgmt.GetAdminSession(ctx, claims.SessionID)
		require.NoError(t, err)
		assert.True(t, session.Active(session.IssuedAt))
	})

	t.Run("a request already holding a console session is left alone", func(t *testing.T) {
		before := verifier.calls

		result, err := env.exchange(t, verifiedIdentity("user_established", "established@example.com"))
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/admin/profile", nil)
		r.AddCookie(&http.Cookie{Name: ConsoleCookieInsecure, Value: result.Token})
		r.AddCookie(&http.Cookie{Name: LegacySessionCookie, Value: "legacy-cookie-value"})

		handler.ServeHTTP(w, r)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, before, verifier.calls,
			"an established session must not pay for an upstream verification on every request")
		assert.Empty(t, w.Result().Cookies(), "nothing to re-issue")
	})

	t.Run("a request with no cookie at all passes straight through", func(t *testing.T) {
		before := verifier.calls

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/admin/profile", nil)

		handler.ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, before, verifier.calls)
	})

	t.Run("an unverifiable legacy cookie is left for the normal 401", func(t *testing.T) {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/admin/profile", nil)
		r.AddCookie(&http.Cookie{Name: LegacySessionCookie, Value: "not-a-real-cookie"})

		handler.ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Empty(t, w.Result().Cookies(),
			"a rejected credential must not be answered with a session cookie")
	})

	t.Run("the upgrade is inert when the console signer is not configured", func(t *testing.T) {
		inert := UpgradeLegacySession(verifier, env.exchanger, nil, logger)(downstream)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/admin/profile", nil)
		r.AddCookie(&http.Cookie{Name: LegacySessionCookie, Value: "legacy-cookie-value"})

		inert.ServeHTTP(w, r)
		require.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// TestUpgradeLegacySessionScopesTheActor confirms the upgraded request is
// authenticated as the right admin, not merely as somebody.
func TestUpgradeLegacySessionScopesTheActor(t *testing.T) {
	t.Parallel()

	env := newExchangeEnv(t)
	ctx := context.Background()

	result, err := env.exchange(t, verifiedIdentity("user_scoped", "scoped@example.com"))
	require.NoError(t, err)
	expected := result.Session.AdminID

	admin, err := env.mgmt.GetAdmin(ctx, expected)
	require.NoError(t, err)

	verifier := &legacyVerifier{
		accept:   "legacy-cookie-value",
		identity: verifiedIdentity("user_scoped", "scoped@example.com"),
	}

	var actor *rbac.Actor
	authenticate := WithAdminSession(env.mgmt, env.signer, "", zaptest.NewLogger(t))
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, err := authenticate(withRequest(r.Context(), r), GetSession(r))
		require.NoError(t, err)
		actor = rbac.FromContext(got)
	})

	handler := UpgradeLegacySession(verifier, env.exchanger, env.signer, zaptest.NewLogger(t))(downstream)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/admin/profile", nil)
	r.AddCookie(&http.Cookie{Name: LegacySessionCookie, Value: "legacy-cookie-value"})
	handler.ServeHTTP(w, r)

	require.NotNil(t, actor)
	assert.Equal(t, expected.String(), actor.ID)
	assert.Equal(t, admin.OrganizationID, actor.OrganizationID)
}
