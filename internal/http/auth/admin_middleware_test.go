package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const testPublicBaseURL = "https://console.lunogram.test"

type middlewareEnv struct {
	mgmt    *management.State
	signer  *ConsoleSigner
	handler Handler
	orgID   uuid.UUID
	adminID uuid.UUID
}

func newMiddlewareEnv(t *testing.T) *middlewareEnv {
	t.Helper()

	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	mgmt := management.NewState(mgmtDB)
	signer := testConsoleSigner(t)
	ctx := context.Background()

	orgID, err := mgmt.CreateOrganization(ctx, "Middleware Org")
	require.NoError(t, err)
	adminID, err := mgmt.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID, Email: "middleware@example.com", Role: "owner",
	})
	require.NoError(t, err)
	require.NoError(t, mgmt.AddMember(ctx, orgID, adminID, "owner"))

	return &middlewareEnv{
		mgmt:    mgmt,
		signer:  signer,
		handler: WithAdminSession(mgmt, signer, testPublicBaseURL, zaptest.NewLogger(t)),
		orgID:   orgID,
		adminID: adminID,
	}
}

// session records a live session for the environment's admin, applying any
// mutations first.
func (e *middlewareEnv) session(t *testing.T, mutate ...func(*management.AdminSession)) *management.AdminSession {
	t.Helper()
	now := time.Now()
	session := management.AdminSession{
		AdminID:           e.adminID,
		ExpiresAt:         now.Add(8 * time.Hour),
		AbsoluteExpiresAt: now.Add(168 * time.Hour),
		Refreshable:       true,
	}
	for _, m := range mutate {
		m(&session)
	}
	created, err := e.mgmt.CreateAdminSession(context.Background(), session)
	require.NoError(t, err)
	return created
}

// authenticate runs the middleware against a bare request carrying the token in
// an Authorization header.
func (e *middlewareEnv) authenticate(t *testing.T, token string) (context.Context, error) {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, testPublicBaseURL+"/api/admin/profile", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	return e.handler(withRequest(context.Background(), r), token)
}

func TestWithAdminSession(t *testing.T) {
	t.Parallel()
	env := newMiddlewareEnv(t)
	ctx := context.Background()

	t.Run("a live session authenticates", func(t *testing.T) {
		session := env.session(t)
		token, err := env.signer.Mint(session, []string{"clerk"})
		require.NoError(t, err)

		got, err := env.authenticate(t, token)
		require.NoError(t, err)

		actor := rbac.FromContext(got)
		require.NotNil(t, actor)
		assert.Equal(t, rbac.ActorAdmin, actor.Type)
		assert.Equal(t, env.adminID.String(), actor.ID, "the subject is always the admin UUID")
		assert.Equal(t, env.orgID, actor.OrganizationID)
		assert.False(t, actor.Impersonated)
	})

	t.Run("a revoked session is rejected", func(t *testing.T) {
		session := env.session(t)
		token, err := env.signer.Mint(session, nil)
		require.NoError(t, err)
		require.NoError(t, env.mgmt.RevokeAdminSession(ctx, session.ID))

		_, err = env.authenticate(t, token)
		require.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("an expired session is rejected even with a token that is still valid", func(t *testing.T) {
		// The session row expires before the token does, so only the database
		// lookup can catch this. That is the whole reason the row exists.
		session := env.session(t, func(s *management.AdminSession) {
			s.ExpiresAt = time.Now().Add(-time.Minute)
			s.AbsoluteExpiresAt = time.Now().Add(-time.Minute)
		})

		claims := jwt.MapClaims{
			"iss": env.signer.issuer,
			"aud": env.signer.audience,
			"sub": session.AdminID.String(),
			"sid": session.ID.String(),
			"exp": time.Now().Add(time.Hour).Unix(),
		}
		token, err := env.signer.keyring.Sign(claims)
		require.NoError(t, err)

		_, err = env.authenticate(t, token)
		require.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("a session past its absolute lifetime is rejected", func(t *testing.T) {
		// The table's CHECK (expires_at <= absolute_expires_at) makes "idle
		// window still open but absolute lifetime exceeded" unrepresentable, so
		// the absolute bound can only ever be reached with the idle one.
		session := env.session(t, func(s *management.AdminSession) {
			s.ExpiresAt = time.Now().Add(-time.Hour)
			s.AbsoluteExpiresAt = time.Now().Add(-time.Hour)
		})
		assert.False(t, session.Active(time.Now()))

		token, err := env.signer.keyring.Sign(jwt.MapClaims{
			"iss": env.signer.issuer, "aud": env.signer.audience,
			"sub": session.AdminID.String(), "sid": session.ID.String(),
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		require.NoError(t, err)

		_, err = env.authenticate(t, token)
		require.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("a token whose subject is not the session's admin is rejected", func(t *testing.T) {
		session := env.session(t)

		other, err := env.mgmt.CreateAdmin(ctx, management.Admin{
			OrganizationID: env.orgID, Email: "other@example.com", Role: "owner",
		})
		require.NoError(t, err)

		token, err := env.signer.keyring.Sign(jwt.MapClaims{
			"iss": env.signer.issuer, "aud": env.signer.audience,
			"sub": other.String(),
			"sid": session.ID.String(),
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		require.NoError(t, err)

		_, err = env.authenticate(t, token)
		require.ErrorIs(t, err, ErrUnauthorized, "a token must not be able to name someone else's session")
	})

	t.Run("an unknown session is rejected", func(t *testing.T) {
		token, err := env.signer.keyring.Sign(jwt.MapClaims{
			"iss": env.signer.issuer, "aud": env.signer.audience,
			"sub": env.adminID.String(), "sid": uuid.New().String(),
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		require.NoError(t, err)

		_, err = env.authenticate(t, token)
		require.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("an empty credential is declined", func(t *testing.T) {
		_, err := env.authenticate(t, "")
		require.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("an impersonated session is attributed but not privileged", func(t *testing.T) {
		upstream := time.Now().Add(20 * time.Minute)
		session := env.session(t, func(s *management.AdminSession) {
			s.Impersonated = true
			s.ImpersonatorSubject = &[]string{"user_support_engineer"}[0]
			s.UpstreamExpiresAt = &upstream
			s.ExpiresAt = upstream
			s.AbsoluteExpiresAt = upstream
			s.Refreshable = false
		})

		token, err := env.signer.Mint(session, []string{"clerk"})
		require.NoError(t, err)

		got, err := env.authenticate(t, token)
		require.NoError(t, err)

		actor := rbac.FromContext(got)
		require.NotNil(t, actor)
		assert.True(t, actor.Impersonated)
		assert.Equal(t, env.adminID.String(), actor.ID,
			"authorization is evaluated as the impersonated admin, so the user key is theirs")
		assert.Equal(t, "user:"+env.adminID.String(), actor.UserKey())
	})

	t.Run("a deleted admin loses access immediately", func(t *testing.T) {
		doomed, err := env.mgmt.CreateAdmin(ctx, management.Admin{
			OrganizationID: env.orgID, Email: "doomed-mw@example.com", Role: "owner",
		})
		require.NoError(t, err)

		session, err := env.mgmt.CreateAdminSession(ctx, management.AdminSession{
			AdminID:           doomed,
			ExpiresAt:         time.Now().Add(8 * time.Hour),
			AbsoluteExpiresAt: time.Now().Add(168 * time.Hour),
			Refreshable:       true,
		})
		require.NoError(t, err)

		token, err := env.signer.Mint(session, nil)
		require.NoError(t, err)
		require.NoError(t, env.mgmt.DeleteAdmin(ctx, doomed))

		_, err = env.authenticate(t, token)
		require.ErrorIs(t, err, ErrUnauthorized)
	})
}

// TestWithAdminSessionOriginCheck covers the CSRF hardening: a cookie is
// attached by the browser automatically, so an unsafe request bearing one must
// come from our own origin.
func TestWithAdminSessionOriginCheck(t *testing.T) {
	t.Parallel()
	env := newMiddlewareEnv(t)

	session := env.session(t)
	token, err := env.signer.Mint(session, nil)
	require.NoError(t, err)

	authenticate := func(t *testing.T, method, origin string, useCookie bool) error {
		t.Helper()
		r := httptest.NewRequest(method, testPublicBaseURL+"/api/admin/profile", nil)
		if useCookie {
			r.AddCookie(&http.Cookie{Name: ConsoleCookieInsecure, Value: token})
		} else {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		_, err := env.handler(withRequest(context.Background(), r), token)
		return err
	}

	tests := map[string]struct {
		method    string
		origin    string
		useCookie bool
		wantErr   bool
	}{
		"cookie, unsafe method, foreign origin": {
			method: http.MethodPost, origin: "https://evil.example", useCookie: true, wantErr: true,
		},
		"cookie, unsafe method, our origin": {
			method: http.MethodPost, origin: testPublicBaseURL, useCookie: true,
		},
		"cookie, unsafe method, no origin": {
			// Browsers omit Origin on same-origin navigations and non-browser
			// clients are not subject to CSRF, so absence is not a rejection.
			method: http.MethodPost, useCookie: true,
		},
		"cookie, safe method, foreign origin": {
			method: http.MethodGet, origin: "https://evil.example", useCookie: true,
		},
		"header credential, unsafe method, foreign origin": {
			// An Authorization header is never attached automatically, so it
			// cannot be replayed cross-origin the way a cookie can.
			method: http.MethodPost, origin: "https://evil.example",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := authenticate(t, tc.method, tc.origin, tc.useCookie)
			if tc.wantErr {
				require.ErrorIs(t, err, ErrUnauthorized)
				return
			}
			require.NoError(t, err)
		})
	}
}

// TestWithAdminSessionOriginFallsBackToTheRequestOrigin covers a PUBLIC_URL that
// does not match how the console is actually reached. That must degrade to
// ordinary same-origin enforcement rather than rejecting every write the console
// makes, and it is safe because the browser sets Origin to the ATTACKER's page
// on a cross-site request while the URL stays ours -- so the two can only agree
// when the request really is same-origin.
func TestWithAdminSessionOriginFallsBackToTheRequestOrigin(t *testing.T) {
	t.Parallel()
	env := newMiddlewareEnv(t)

	session := env.session(t)
	token, err := env.signer.Mint(session, nil)
	require.NoError(t, err)

	misconfigured := WithAdminSession(env.mgmt, env.signer, "https://stale.example", zaptest.NewLogger(t))

	authenticate := func(origin string) error {
		r := httptest.NewRequest(http.MethodPost, testPublicBaseURL+"/api/admin/profile", nil)
		r.AddCookie(&http.Cookie{Name: ConsoleCookieInsecure, Value: token})
		r.Header.Set("Origin", origin)
		_, err := misconfigured(withRequest(context.Background(), r), token)
		return err
	}

	require.NoError(t, authenticate(testPublicBaseURL), "a same-origin write must still be accepted")
	require.ErrorIs(t, authenticate("https://evil.example"), ErrUnauthorized,
		"a cross-origin write must still be rejected")
}

// TestWithAdminSessionDoesNotFailOpen pins the rule that a backing-store failure
// is never a pass.
func TestWithAdminSessionDoesNotFailOpen(t *testing.T) {
	t.Parallel()
	env := newMiddlewareEnv(t)

	session := env.session(t)
	token, err := env.signer.Mint(session, nil)
	require.NoError(t, err)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	r := httptest.NewRequest(http.MethodGet, testPublicBaseURL+"/api/admin/profile", nil)
	_, err = env.handler(withRequest(cancelled, r), token)
	require.Error(t, err, "a database failure must never authenticate a request")
}
