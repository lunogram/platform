package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
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
	delay    time.Duration

	mu    sync.Mutex
	calls int
}

func (v *legacyVerifier) Driver() string { return "clerk" }

func (v *legacyVerifier) Verify(_ context.Context, r *http.Request) (*VerifiedIdentity, error) {
	v.mu.Lock()
	v.calls++
	v.mu.Unlock()

	if v.delay > 0 {
		time.Sleep(v.delay)
	}
	if GetSession(r) != v.accept {
		return nil, ErrUnauthorized
	}
	return v.identity, nil
}

func (v *legacyVerifier) callCount() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.calls
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
		before := verifier.callCount()

		result, err := env.exchange(t, verifiedIdentity("user_established", "established@example.com"))
		require.NoError(t, err)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/admin/profile", nil)
		r.AddCookie(&http.Cookie{Name: ConsoleCookieInsecure, Value: result.Token})
		r.AddCookie(&http.Cookie{Name: LegacySessionCookie, Value: "legacy-cookie-value"})

		handler.ServeHTTP(w, r)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, before, verifier.callCount(),
			"an established session must not pay for an upstream verification on every request")
		assert.Empty(t, w.Result().Cookies(), "nothing to re-issue")
	})

	t.Run("a request with no cookie at all passes straight through", func(t *testing.T) {
		before := verifier.callCount()

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/admin/profile", nil)

		handler.ServeHTTP(w, r)

		require.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Equal(t, before, verifier.callCount())
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

// countSessions is the number of admin_sessions rows the environment holds.
func countSessions(t *testing.T, env *exchangeEnv) int {
	t.Helper()
	var n int
	require.NoError(t, env.db.GetContext(context.Background(), &n, `SELECT count(*) FROM admin_sessions`))
	return n
}

// TestUpgradeLegacySessionDoesNotStormTheSessionTable is the reason the upgrade
// reuses rather than creates.
//
// A console page load fires many requests in parallel and none of them holds the
// new cookie yet. Minting per request would grow admin_sessions with concurrency
// during the rollout window and make the table useless for showing somebody
// where they are signed in.
func TestUpgradeLegacySessionDoesNotStormTheSessionTable(t *testing.T) {
	t.Parallel()

	env := newExchangeEnv(t)

	_, err := env.exchange(t, verifiedIdentity("user_storm", "storm@example.com"))
	require.NoError(t, err)

	before := countSessions(t, env)

	verifier := &legacyVerifier{
		accept:   "legacy-cookie-value",
		identity: verifiedIdentity("user_storm", "storm@example.com"),
		// Hold the leader inside Verify so the rest of the burst really is
		// concurrent rather than serialised by scheduling luck.
		delay: 150 * time.Millisecond,
	}

	authenticate := WithAdminSession(env.mgmt, env.signer, "", zaptest.NewLogger(t))
	var served atomic.Int64
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := authenticate(withRequest(r.Context(), r), GetSession(r)); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		served.Add(1)
		w.WriteHeader(http.StatusOK)
	})

	handler := UpgradeLegacySession(verifier, env.exchanger, env.signer, zaptest.NewLogger(t))(downstream)

	const burst = 20
	var wg sync.WaitGroup
	tokens := make([]string, burst)

	for i := range burst {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/api/admin/profile", nil)
			r.AddCookie(&http.Cookie{Name: LegacySessionCookie, Value: "legacy-cookie-value"})
			handler.ServeHTTP(w, r)

			require.Equal(t, http.StatusOK, w.Code)
			cookies := w.Result().Cookies()
			require.Len(t, cookies, 1)
			tokens[i] = cookies[0].Value
		}()
	}
	wg.Wait()

	require.Equal(t, int64(burst), served.Load(), "every request in the burst must succeed")
	assert.Equal(t, before, countSessions(t, env),
		"a burst of %d requests must reuse the existing session, not open more", burst)

	// Every request came away with a token, and all of them name one session.
	sessions := map[uuid.UUID]struct{}{}
	for _, token := range tokens {
		require.NotEmpty(t, token)
		claims, err := env.signer.Verify(token)
		require.NoError(t, err)
		sessions[claims.SessionID] = struct{}{}
	}
	assert.Len(t, sessions, 1, "one browser, one session")

	assert.Equal(t, 1, verifier.callCount(),
		"the burst must collapse into a single upstream verification")
}

// TestUpgradeLegacySessionCreatesOneSessionForAFirstBurst covers the case with
// nothing to reuse: an admin whose browser holds a legacy cookie but who has no
// console session yet.
func TestUpgradeLegacySessionCreatesOneSessionForAFirstBurst(t *testing.T) {
	t.Parallel()

	env := newExchangeEnv(t)
	ctx := context.Background()

	// Provision the admin without opening a session, exactly as an admin who
	// last logged in before this release would appear.
	_, err := env.exchanger.Provision(ctx, verifiedIdentity("user_first", "first@example.com"))
	require.NoError(t, err)
	require.Zero(t, countSessions(t, env))

	verifier := &legacyVerifier{
		accept:   "legacy-cookie-value",
		identity: verifiedIdentity("user_first", "first@example.com"),
		delay:    150 * time.Millisecond,
	}
	handler := UpgradeLegacySession(verifier, env.exchanger, env.signer, zaptest.NewLogger(t))(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/api/admin/profile", nil)
			r.AddCookie(&http.Cookie{Name: LegacySessionCookie, Value: "legacy-cookie-value"})
			handler.ServeHTTP(w, r)
			require.Equal(t, http.StatusOK, w.Code)
		}()
	}
	wg.Wait()

	assert.Equal(t, 1, countSessions(t, env), "a first burst must open exactly one session")
}

// TestUpgradeLegacySessionSkipsTheLoginCallback pins that an explicit login
// mints its own session rather than being upgraded into an existing one, which
// would leave two rows for a single sign-in.
func TestUpgradeLegacySessionSkipsTheLoginCallback(t *testing.T) {
	t.Parallel()

	env := newExchangeEnv(t)

	verifier := &legacyVerifier{
		accept:   "legacy-cookie-value",
		identity: verifiedIdentity("user_callback", "callback@example.com"),
	}
	handler := UpgradeLegacySession(verifier, env.exchanger, env.signer, zaptest.NewLogger(t))(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }),
	)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login/clerk/callback", nil)
	r.AddCookie(&http.Cookie{Name: LegacySessionCookie, Value: "legacy-cookie-value"})

	handler.ServeHTTP(w, r)

	assert.Zero(t, verifier.callCount(), "the login callback does its own verification")
	assert.Empty(t, w.Result().Cookies())
	assert.Zero(t, countSessions(t, env))
}

// TestUpgradeLegacySessionNeverReusesAnImpersonatedSession pins that an
// impersonated session -- non-refreshable and clamped to the upstream session
// that authorised it -- is never handed to a credential arriving later.
func TestUpgradeLegacySessionNeverReusesAnImpersonatedSession(t *testing.T) {
	t.Parallel()

	env := newExchangeEnv(t)
	ctx := context.Background()

	_, err := env.exchange(t, verifiedIdentity("user_imp", "imp@example.com"))
	require.NoError(t, err)

	impersonated := verifiedIdentity("user_imp", "imp@example.com")
	impersonated.Actor = &VerifiedActor{Subject: "user_support_engineer"}
	impersonated.ExpiresAt = time.Now().Add(20 * time.Minute)

	result, err := env.exchanger.Upgrade(ctx, httptest.NewRequest(http.MethodGet, "/", nil), impersonated)
	require.NoError(t, err)

	assert.True(t, result.Session.Impersonated)
	assert.False(t, result.Session.Refreshable)
	assert.WithinDuration(t, impersonated.ExpiresAt, result.Session.AbsoluteExpiresAt, time.Second,
		"an impersonated upgrade must compute its own clamped lifetime, never inherit one")

	// And the ordinary session it could have reused is untouched and still
	// non-impersonated.
	var impersonatedRows int
	require.NoError(t, env.db.GetContext(ctx, &impersonatedRows,
		`SELECT count(*) FROM admin_sessions WHERE impersonated`))
	assert.Equal(t, 1, impersonatedRows)
}
