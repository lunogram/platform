package v1

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
	"time"

	"github.com/lunogram/platform/internal/claim"
	"github.com/lunogram/platform/internal/config"

	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// clerkJWKS returns a config.JWKS backed by a server publishing a fresh RSA
// public key, which is what an AUTH_JWKS_URL resolves to in production. The
// clerk driver refuses to build without one, so any fixture that configures
// clerk has to supply it.
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

func TestGetAuthMethods(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	cfg := config.Node{
		Auth: config.Auth{
			Drivers: []string{"clerk"},
			JWKS:    clerkJWKS(t),
			Clerk: config.ClerkAuth{
				SecretKey: "sk_test_xxx",
			},
		},
	}

	controller, err := NewAuthController(logger, mgmt, management.NewState(mgmt), cfg, nil, nil, nil)
	require.NoError(t, err)

	type test struct {
		code int
	}

	tests := map[string]test{
		"success": {
			code: 200,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/auth/methods", nil)
			controller.GetAuthMethods(res, req)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var result []string
				err = json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Contains(t, result, "clerk")
			}
		})
	}
}

func TestAuthCallbackWithInvalidDriver(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	cfg := config.Node{
		Auth: config.Auth{
			Drivers: []string{"basic"},
			Basic:   config.BasicAuth{Email: "a@b", Password: "c"},
		},
	}

	controller, err := NewAuthController(logger, mgmt, management.NewState(mgmt), cfg, nil, nil, nil)
	require.NoError(t, err)

	type test struct {
		driver oapi.AuthCallbackParamsDriver
		code   int
	}

	tests := map[string]test{
		"invalid driver": {
			driver: "invalid",
			code:   404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/auth/callback/"+string(test.driver), nil)
			controller.AuthCallback(res, req, test.driver)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

func TestAuthWebhookWithInvalidDriver(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmt, _, _ := teststore.RunPostgreSQL(t)

	cfg := config.Node{
		Auth: config.Auth{
			Drivers: []string{"basic"},
			Basic:   config.BasicAuth{Email: "a@b", Password: "c"},
		},
	}

	controller, err := NewAuthController(logger, mgmt, management.NewState(mgmt), cfg, nil, nil, nil)
	require.NoError(t, err)

	type test struct {
		driver oapi.AuthWebhookParamsDriver
		code   int
	}

	tests := map[string]test{
		"invalid driver": {
			driver: "invalid",
			code:   404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/auth/webhook/"+string(test.driver), nil)
			controller.AuthWebhook(res, req, test.driver)

			require.Equal(t, test.code, res.Code, res.Body.String())
		})
	}
}

// consoleSignerFor builds a console signer with a throwaway key.
func consoleSignerFor(t *testing.T) *auth.ConsoleSigner {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)

	signer, err := auth.NewConsoleSigner(config.ConsoleAuth{
		SigningKey:  string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})),
		Issuer:      "https://lunogram.test/console",
		Audience:    "lunogram-console",
		IdleTTL:     8 * time.Hour,
		AbsoluteTTL: 168 * time.Hour,
	})
	require.NoError(t, err)
	return signer
}

// TestRefreshSessionDistinguishesGoneFromNotExtendable is what keeps the console
// out of a refresh loop.
//
// A session that is alive but non-refreshable -- impersonation is recorded that
// way by construction -- must not be reported the same as one that has been
// revoked. Conflating them would eject an operator from a session that is
// working perfectly well, on the first scheduled refresh.
func TestRefreshSessionDistinguishesGoneFromNotExtendable(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	state := management.NewState(mgmtDB)
	signer := consoleSignerFor(t)
	ctx := t.Context()

	orgID, err := state.CreateOrganization(ctx, "Refresh Organization")
	require.NoError(t, err)
	adminID, err := state.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID, Email: "refresh@example.com", Role: "owner",
	})
	require.NoError(t, err)

	controller, err := NewAuthController(logger, mgmtDB, state, config.Node{
		Auth: config.Auth{Drivers: []string{"basic"}, Basic: config.BasicAuth{Email: "a@b", Password: "c"}},
	}, nil, signer, nil)
	require.NoError(t, err)

	refresh := func(t *testing.T, token string) *httptest.ResponseRecorder {
		t.Helper()
		res := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/auth/refresh", nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		controller.RefreshSession(res, req)
		return res
	}

	newSession := func(t *testing.T, mutate func(*management.AdminSession)) *management.AdminSession {
		t.Helper()
		now := time.Now()
		session := management.AdminSession{
			AdminID:           adminID,
			ExpiresAt:         now.Add(time.Hour),
			AbsoluteExpiresAt: now.Add(168 * time.Hour),
			Refreshable:       true,
		}
		if mutate != nil {
			mutate(&session)
		}
		created, err := state.CreateAdminSession(ctx, session)
		require.NoError(t, err)
		return created
	}

	t.Run("a refreshable session is extended", func(t *testing.T) {
		session := newSession(t, nil)
		token, err := signer.Mint(session, []string{"basic"})
		require.NoError(t, err)

		res := refresh(t, token)
		require.Equal(t, 200, res.Code, res.Body.String())

		var body oapi.SessionRefresh
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
		require.True(t, body.ExpiresAt.After(session.ExpiresAt), "the idle window must have moved out")

		cookies := res.Result().Cookies()
		require.Len(t, cookies, 1, "the browser is handed the reissued token")
	})

	t.Run("an impersonated session answers 403, not 401", func(t *testing.T) {
		upstream := time.Now().Add(20 * time.Minute)
		session := newSession(t, func(s *management.AdminSession) {
			s.Impersonated = true
			s.ImpersonatorSubject = ptr.To("user_support_engineer")
			s.UpstreamExpiresAt = &upstream
			s.ExpiresAt = upstream
			s.AbsoluteExpiresAt = upstream
			s.Refreshable = false
		})
		token, err := signer.Mint(session, []string{"clerk"})
		require.NoError(t, err)

		res := refresh(t, token)
		require.Equal(t, 403, res.Code, "the session is alive, it just cannot be extended")
	})

	t.Run("a revoked session answers 401", func(t *testing.T) {
		session := newSession(t, nil)
		token, err := signer.Mint(session, nil)
		require.NoError(t, err)
		require.NoError(t, state.RevokeAdminSession(ctx, session.ID))

		require.Equal(t, 401, refresh(t, token).Code)
	})

	t.Run("no credential answers 401", func(t *testing.T) {
		require.Equal(t, 401, refresh(t, "").Code)
	})
}

// TestLogoutRevokesTheSession pins that logging out ends the session
// server-side rather than only clearing cookies.
func TestLogoutRevokesTheSession(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	state := management.NewState(mgmtDB)
	signer := consoleSignerFor(t)
	ctx := t.Context()

	orgID, err := state.CreateOrganization(ctx, "Logout Organization")
	require.NoError(t, err)
	adminID, err := state.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID, Email: "logout@example.com", Role: "owner",
	})
	require.NoError(t, err)

	session, err := state.CreateAdminSession(ctx, management.AdminSession{
		AdminID:           adminID,
		ExpiresAt:         time.Now().Add(time.Hour),
		AbsoluteExpiresAt: time.Now().Add(168 * time.Hour),
		Refreshable:       true,
	})
	require.NoError(t, err)

	controller, err := NewAuthController(logger, mgmtDB, state, config.Node{
		Auth: config.Auth{Drivers: []string{"basic"}, Basic: config.BasicAuth{Email: "a@b", Password: "c"}},
	}, nil, signer, nil)
	require.NoError(t, err)

	token, err := signer.Mint(session, []string{"basic"})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	controller.Logout(res, req)

	require.Equal(t, 204, res.Code, res.Body.String())

	ended, err := state.GetAdminSession(ctx, session.ID)
	require.NoError(t, err)
	require.False(t, ended.Active(time.Now()), "a copy of the token must not keep working")

	// Every cookie name a credential could have arrived in is expired, so
	// logging out does not depend on guessing which one the browser holds.
	require.Len(t, res.Result().Cookies(), 3)
	for _, cookie := range res.Result().Cookies() {
		require.Empty(t, cookie.Value)
		require.True(t, cookie.MaxAge < 0)
	}
}
