package v1

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/http/auth/verifiers"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/jwks"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/sso"
	"github.com/lunogram/platform/internal/ssrf"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// stubProvider is an OpenID Connect provider on loopback: a discovery document,
// a JWKS, and a token endpoint that mints whatever the test asks it to. No test
// in this file reaches the network.
type stubProvider struct {
	server *httptest.Server
	key    *rsa.PrivateKey

	// claims is what the next ID token carries. A test mutates it to produce a
	// token with the wrong issuer, the wrong audience, an expiry in the past, or
	// a nonce that answers no login.
	claims jwt.MapClaims
	// codeChallenges are what /token will accept the presented verifier to hash
	// to. They are collected from the authorization URLs, so a flow that forgot
	// to send its verifier fails here exactly as a real provider would fail it.
	//
	// A set rather than one value because a browser may have several logins
	// outstanding, and a real provider keys the challenge to the code it issued
	// rather than to whichever request came last.
	codeChallenges []string
	// omitIDToken makes the token endpoint answer without an id_token.
	omitIDToken bool
	// userInfo is what /userinfo answers, and nil switches the endpoint out of
	// the discovery document. A provider that keeps the email scope there rather
	// than in the ID token is compliant and common.
	userInfo map[string]any
	// lastTokenForm records what the token exchange actually sent.
	lastTokenForm url.Values
}

func newStubProvider(t *testing.T) *stubProvider {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	provider := &stubProvider{key: key}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		document := map[string]any{
			"issuer":                                provider.issuer(),
			"authorization_endpoint":                provider.issuer() + "/authorize",
			"token_endpoint":                        provider.issuer() + "/token",
			"jwks_uri":                              provider.issuer() + "/jwks",
			"scopes_supported":                      []string{"openid", "email", "profile"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}
		if provider.userInfo != nil {
			document["userinfo_endpoint"] = provider.issuer() + "/userinfo"
		}
		writeJSON(w, document)
	})

	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer stub-access-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		writeJSON(w, provider.userInfo)
	})

	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"keys": []map[string]string{{
			"kty": "RSA",
			"alg": "RS256",
			"use": "sig",
			"kid": "stub-1",
			"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
			"e":   base64.RawURLEncoding.EncodeToString([]byte{0x01, 0x00, 0x01}),
		}}})
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		provider.lastTokenForm = r.PostForm

		if len(provider.codeChallenges) > 0 {
			sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
			if !slices.Contains(provider.codeChallenges, base64.RawURLEncoding.EncodeToString(sum[:])) {
				w.WriteHeader(http.StatusBadRequest)
				writeJSON(w, map[string]string{"error": "invalid_grant"})
				return
			}
		}

		response := map[string]any{
			"access_token": "stub-access-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		}
		if !provider.omitIDToken {
			response["id_token"] = provider.mint(t)
		}
		writeJSON(w, response)
	})

	provider.server = httptest.NewServer(mux)
	t.Cleanup(provider.server.Close)
	return provider
}

func (p *stubProvider) issuer() string { return p.server.URL }

func (p *stubProvider) mint(t *testing.T) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, p.claims)
	token.Header["kid"] = "stub-1"
	signed, err := token.SignedString(p.key)
	require.NoError(t, err)
	return signed
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

// loopbackFetcher fetches a JWKS without the outbound guard, which would
// otherwise refuse the stub provider on 127.0.0.1.
type loopbackFetcher struct{}

func (loopbackFetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	buffer := make([]byte, 0, 4096)
	chunk := make([]byte, 4096)
	for {
		n, err := response.Body.Read(chunk)
		buffer = append(buffer, chunk[:n]...)
		if err != nil {
			break
		}
	}
	return buffer, nil
}

// literalNull asks signIn for a claim emitted as JSON null, which is different
// from one a provider does not send at all.
type literalNull struct{}

// testProviderID is the id the single-provider environment form is given.
const testProviderID = config.DefaultOIDCProviderID

type oidcEnv struct {
	t        *testing.T
	auth     *AuthController
	state    *management.State
	provider *stubProvider
}

func newOIDCEnv(t *testing.T) *oidcEnv {
	t.Helper()

	logger := zaptest.NewLogger(t)
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	state := management.NewState(mgmtDB)
	engine := rbac.NewTestEngine(t)
	provider := newStubProvider(t)

	options, err := goredis.ParseURL(container.RunRedis(t))
	require.NoError(t, err)
	rdb := goredis.NewClient(options)
	t.Cleanup(func() { rdb.Close() })

	cfg := config.Node{
		PublicURL: "https://console.example.test",
		Redis:     config.Redis{KeyPrefix: "test:" + t.Name() + ":"},
		Auth: config.Auth{
			Drivers: []string{verifiers.OIDCDriver},
			OIDC: config.OIDCAuth{Provider: config.OIDCProvider{
				ID:           testProviderID,
				Issuer:       provider.issuer(),
				ClientID:     "stub-client",
				ClientSecret: "stub-client-secret",
			}},
		},
	}

	// The stub provider is on loopback over plaintext, so the tests run under
	// relaxations an operator would have to opt into explicitly.
	policy := ssrf.Policy{AllowPrivate: true, AllowHTTP: true}
	controller, err := NewAuthController(logger, mgmtDB, state, cfg, engine, consoleSignerFor(t), nil, nil, nil,
		verifiers.Deps{
			Keys:       jwks.New(jwks.Config{}, nil, loopbackFetcher{}, logger),
			Flows:      sso.NewFlowStore(rdb, cfg.Redis.KeyPrefix),
			Discovery:  sso.NewDiscovery(http.DefaultClient, policy, 0),
			HTTPClient: http.DefaultClient,
			BaseURL:    cfg.PublicBaseURL(),
		})
	require.NoError(t, err)

	return &oidcEnv{t: t, auth: controller, state: state, provider: provider}
}

// begin runs the start endpoint and returns the state the deployment minted, the
// nonce it asked the provider for, and the binding cookie it set on the browser.
func (env *oidcEnv) begin(redirect string) (state, nonce string, binding *http.Cookie) {
	return env.beginCarrying(redirect, nil)
}

// beginCarrying starts a login in a browser that already holds `carried`, which
// is what a second tab looks like.
func (env *oidcEnv) beginCarrying(redirect string, carried *http.Cookie) (state, nonce string, binding *http.Cookie) {
	env.t.Helper()

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/oidc/"+testProviderID+"/start", nil)
	if carried != nil {
		req.AddCookie(carried)
	}

	params := oapi.StartOIDCLoginParams{}
	if redirect != "" {
		params.R = &redirect
	}
	env.auth.StartOIDCLogin(res, req, testProviderID, params)
	require.Equal(env.t, http.StatusFound, res.Code, res.Body.String())

	target, err := url.Parse(res.Header().Get("Location"))
	require.NoError(env.t, err)

	query := target.Query()
	require.Equal(env.t, "S256", query.Get("code_challenge_method"))
	env.provider.codeChallenges = append(env.provider.codeChallenges, query.Get("code_challenge"))

	for _, cookie := range res.Result().Cookies() {
		if cookie.Name == auth.OIDCBindingCookieInsecure && cookie.Value != "" {
			binding = cookie
		}
	}
	if binding == nil && carried != nil {
		// Nothing new was set because the browser's existing binding was reused,
		// which is the point of carrying one.
		binding = carried
	}
	require.NotNil(env.t, binding, "the start endpoint binds the login to this browser")
	assert.True(env.t, binding.HttpOnly)

	return query.Get("state"), query.Get("nonce"), binding
}

// callback runs the callback endpoint with an authorization response, carrying
// the binding cookie the browser was given.
func (env *oidcEnv) callback(query url.Values, binding *http.Cookie) *httptest.ResponseRecorder {
	env.t.Helper()

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/oidc/"+testProviderID+"/callback?"+query.Encode(), nil)
	if binding != nil {
		req.AddCookie(binding)
	}
	env.auth.CompleteOIDCLogin(res, req, testProviderID, oapi.CompleteOIDCLoginParams{})
	return res
}

// signIn runs a whole login: start, mint an ID token carrying the given claims
// on top of a valid base, and complete.
func (env *oidcEnv) signIn(email string, overrides map[string]any) *httptest.ResponseRecorder {
	env.t.Helper()

	state, nonce, binding := env.begin("")
	env.provider.claims = env.baseClaims(email, nonce)
	// A nil override omits the claim rather than emitting a null, which is what
	// a provider that simply does not send it looks like. literalNull is how a
	// test asks for the null itself.
	for key, value := range overrides {
		switch value.(type) {
		case nil:
			delete(env.provider.claims, key)
		case literalNull:
			env.provider.claims[key] = nil
		default:
			env.provider.claims[key] = value
		}
	}

	return env.callback(url.Values{"code": {"stub-code"}, "state": {state}}, binding)
}

func (env *oidcEnv) baseClaims(email, nonce string) jwt.MapClaims {
	return jwt.MapClaims{
		"iss":            env.provider.issuer(),
		"aud":            "stub-client",
		"sub":            "subject-" + email,
		"exp":            time.Now().Add(time.Hour).Unix(),
		"iat":            time.Now().Unix(),
		"nonce":          nonce,
		"email":          email,
		"email_verified": true,
		"given_name":     "Ada",
		"family_name":    "Lovelace",
	}
}

func (env *oidcEnv) adminFor(email string) *management.Admin {
	env.t.Helper()
	admin, err := env.state.ResolveAdminByEmail(context.Background(), email)
	require.NoError(env.t, err)
	return admin
}

// assertLandedInConsole checks that a login succeeded: the browser is redirected
// into the console and holds a session cookie.
func assertLandedInConsole(t *testing.T, res *httptest.ResponseRecorder, redirect string) {
	t.Helper()
	require.Equal(t, http.StatusFound, res.Code, res.Body.String())
	assert.Equal(t, redirect, res.Header().Get("Location"))

	var session bool
	for _, cookie := range res.Result().Cookies() {
		if cookie.Name == auth.ConsoleCookieInsecure && cookie.Value != "" {
			session = true
		}
	}
	assert.True(t, session, "a successful login sets the session cookie")
}

// assertRejected checks that a login failed: back to the login view, no cookie.
func assertRejected(t *testing.T, res *httptest.ResponseRecorder, reason string) {
	t.Helper()
	require.Equal(t, http.StatusFound, res.Code, res.Body.String())
	location := res.Header().Get("Location")
	assert.True(t, strings.HasPrefix(location, "/login?"), "expected a bounce to the login view, got %q", location)
	assert.Contains(t, location, "sso_error="+reason)

	for _, cookie := range res.Result().Cookies() {
		assert.Empty(t, cookie.Value, "a rejected login must not leave a session cookie")
	}
}

func TestFederatedLogin(t *testing.T) {
	t.Parallel()
	env := newOIDCEnv(t)

	t.Run("a proved identity becomes a console session", func(t *testing.T) {
		res := env.signIn("ada@example.test", nil)
		assertLandedInConsole(t, res, "/")

		admin := env.adminFor("ada@example.test")
		identity, err := env.state.GetAdminIdentity(context.Background(), env.provider.issuer(), "subject-ada@example.test")
		require.NoError(t, err)
		assert.Equal(t, admin.ID, identity.AdminID)
		assert.Equal(t, management.IdentityProviderOIDC, identity.Provider)
	})

	t.Run("the login exchanges the code with the pkce verifier", func(t *testing.T) {
		assert.NotEmpty(t, env.provider.lastTokenForm.Get("code_verifier"),
			"the token exchange must present the verifier the challenge was derived from")
	})

	t.Run("the post-login redirect is reduced to a same-site path", func(t *testing.T) {
		state, nonce, binding := env.begin("https://evil.test/steal")
		env.provider.claims = env.baseClaims("bounce@example.test", nonce)

		res := env.callback(url.Values{"code": {"stub-code"}, "state": {state}}, binding)
		assertLandedInConsole(t, res, "/")
	})

	t.Run("a redirect the login started with is honoured", func(t *testing.T) {
		state, nonce, binding := env.begin("/campaigns?tab=sent")
		env.provider.claims = env.baseClaims("deep@example.test", nonce)

		res := env.callback(url.Values{"code": {"stub-code"}, "state": {state}}, binding)
		assertLandedInConsole(t, res, "/campaigns?tab=sent")
	})
}

// Several providers is the point of the list form: each gets its own login URL,
// its own redirect URI, and a boundary between them.
func TestFederatedLoginWithSeveralProviders(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	logger := zaptest.NewLogger(t)
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	state := management.NewState(mgmtDB)
	staff, contractors := newStubProvider(t), newStubProvider(t)

	options, err := goredis.ParseURL(container.RunRedis(t))
	require.NoError(t, err)
	rdb := goredis.NewClient(options)
	t.Cleanup(func() { rdb.Close() })

	cfg := config.Node{
		PublicURL: "https://console.example.test",
		Redis:     config.Redis{KeyPrefix: "test:" + t.Name() + ":"},
		Auth: config.Auth{
			Drivers: []string{verifiers.OIDCDriver},
			OIDC: config.OIDCAuth{Providers: []config.OIDCProvider{
				{
					ID: "staff", Name: "Staff directory",
					Issuer: staff.issuer(), ClientID: "stub-client", ClientSecret: "stub-client-secret",
				},
				{
					ID: "contractors", Name: "Contractors",
					Issuer: contractors.issuer(), ClientID: "stub-client", ClientSecret: "stub-client-secret",
					AllowedDomains: []string{"partner.test"},
				},
			}},
		},
	}

	policy := ssrf.Policy{AllowPrivate: true, AllowHTTP: true}
	controller, err := NewAuthController(logger, mgmtDB, state, cfg, rbac.NewTestEngine(t), consoleSignerFor(t), nil, nil, nil,
		verifiers.Deps{
			Keys:       jwks.New(jwks.Config{}, nil, loopbackFetcher{}, logger),
			Flows:      sso.NewFlowStore(rdb, cfg.Redis.KeyPrefix),
			Discovery:  sso.NewDiscovery(http.DefaultClient, policy, 0),
			HTTPClient: http.DefaultClient,
			BaseURL:    cfg.PublicBaseURL(),
		})
	require.NoError(t, err)

	env := &oidcEnv{t: t, auth: controller, state: state}

	begin := func(provider string, stub *stubProvider) (string, string, *http.Cookie) {
		t.Helper()
		res := httptest.NewRecorder()
		controller.StartOIDCLogin(res, httptest.NewRequest("GET", "/start", nil), provider, oapi.StartOIDCLoginParams{})
		require.Equal(t, http.StatusFound, res.Code, res.Body.String())

		target, err := url.Parse(res.Header().Get("Location"))
		require.NoError(t, err)
		query := target.Query()
		stub.codeChallenges = append(stub.codeChallenges, query.Get("code_challenge"))

		var binding *http.Cookie
		for _, cookie := range res.Result().Cookies() {
			if cookie.Name == auth.OIDCBindingCookieInsecure && cookie.Value != "" {
				binding = cookie
			}
		}
		require.NotNil(t, binding)
		return query.Get("state"), query.Get("nonce"), binding
	}

	callback := func(provider string, query url.Values, binding *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		res := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/callback?"+query.Encode(), nil)
		if binding != nil {
			req.AddCookie(binding)
		}
		controller.CompleteOIDCLogin(res, req, provider, oapi.CompleteOIDCLoginParams{})
		return res
	}

	claims := func(stub *stubProvider, email, nonce string) jwt.MapClaims {
		return jwt.MapClaims{
			"iss": stub.issuer(), "aud": "stub-client", "sub": "subject-" + email,
			"exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(),
			"nonce": nonce, "email": email, "email_verified": true,
		}
	}

	t.Run("each provider is listed with its own name", func(t *testing.T) {
		res := httptest.NewRecorder()
		controller.ListOIDCProviders(res, httptest.NewRequest("GET", "/api/auth/oidc/providers", nil))
		require.Equal(t, http.StatusOK, res.Code)

		var listed []oapi.OIDCProvider
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &listed))
		require.Len(t, listed, 2)
		assert.Equal(t, "staff", listed[0].Id, "declaration order is preserved")
		assert.Equal(t, "Staff directory", listed[0].Name)
	})

	t.Run("a provider nobody configured is absent", func(t *testing.T) {
		res := httptest.NewRecorder()
		controller.StartOIDCLogin(res, httptest.NewRequest("GET", "/start", nil), "nonexistent", oapi.StartOIDCLoginParams{})
		assert.Equal(t, http.StatusNotFound, res.Code)
	})

	t.Run("each provider signs its own people in", func(t *testing.T) {
		state, nonce, binding := begin("staff", staff)
		staff.claims = claims(staff, "ada@example.test", nonce)
		assertLandedInConsole(t, callback("staff", url.Values{"code": {"c"}, "state": {state}}, binding), "/")

		identity, err := env.state.GetAdminIdentity(ctx, staff.issuer(), "subject-ada@example.test")
		require.NoError(t, err)
		assert.Equal(t, management.IdentityProviderOIDC, identity.Provider)
	})

	// Without this the least trustworthy provider decides who can reach every
	// account: a verified address links to an existing admin whoever asserted it.
	t.Run("a provider may not assert a domain outside its allow-list", func(t *testing.T) {
		state, nonce, binding := begin("contractors", contractors)
		contractors.claims = claims(contractors, "impostor@example.test", nonce)
		assertRejected(t, callback("contractors", url.Values{"code": {"c"}, "state": {state}}, binding), "domain")

		_, err := env.state.ResolveAdminByEmail(ctx, "impostor@example.test")
		assert.Error(t, err, "no admin is provisioned for a domain the provider may not speak for")
	})

	t.Run("its own domain still signs in", func(t *testing.T) {
		state, nonce, binding := begin("contractors", contractors)
		contractors.claims = claims(contractors, "grace@partner.test", nonce)
		assertLandedInConsole(t, callback("contractors", url.Values{"code": {"c"}, "state": {state}}, binding), "/")
	})

	// The state is a value the browser carries. Redeeming one at another
	// provider's callback would prove it against that provider's issuer.
	t.Run("a state issued for one provider is refused at another", func(t *testing.T) {
		state, nonce, binding := begin("staff", staff)
		staff.claims = claims(staff, "cross@example.test", nonce)
		assertRejected(t, callback("contractors", url.Values{"code": {"c"}, "state": {state}}, binding), "expired")
	})
}

// A provider may return the email and profile scopes from UserInfo rather than
// in the ID token, which OpenID Connect permits and several implement. Without
// the fallback every one of those logins fails with "no email address".
func TestFederatedLoginReadsUserInfoWhenTheIDTokenOmitsTheAddress(t *testing.T) {
	t.Parallel()
	env := newOIDCEnv(t)

	t.Run("the profile is taken from userinfo", func(t *testing.T) {
		env.provider.userInfo = map[string]any{
			"sub":            "subject-grace@example.test",
			"email":          "grace@example.test",
			"email_verified": true,
			"given_name":     "Grace",
			"family_name":    "Hopper",
		}
		res := env.signIn("grace@example.test", map[string]any{"email": "", "given_name": ""})
		assertLandedInConsole(t, res, "/")

		admin := env.adminFor("grace@example.test")
		require.NotNil(t, admin.FirstName)
		assert.Equal(t, "Grace", *admin.FirstName)
	})

	// A provider may keep only the attestation at UserInfo, with the address in
	// the token. Skipping the fetch then leaves the login unverified, and it
	// silently never links to an account that already exists.
	t.Run("an attestation alone is fetched when the token carries only the address", func(t *testing.T) {
		env.provider.userInfo = map[string]any{
			"sub":            "subject-alan@example.test",
			"email":          "alan@example.test",
			"email_verified": true,
		}
		res := env.signIn("alan@example.test", map[string]any{"email_verified": nil})
		assertLandedInConsole(t, res, "/")

		identity, err := env.state.GetAdminIdentity(context.Background(), env.provider.issuer(), "subject-alan@example.test")
		require.NoError(t, err)
		assert.True(t, identity.EmailVerified)
	})

	// A provider may keep the profile scope at UserInfo while the ID token
	// carries a complete, attested address. Exempting the names from the check
	// would provision that admin without them, permanently.
	t.Run("names alone are fetched when the token is otherwise complete", func(t *testing.T) {
		env.provider.userInfo = map[string]any{
			"sub":         "subject-katherine@example.test",
			"email":       "katherine@example.test",
			"given_name":  "Katherine",
			"family_name": "Johnson",
		}
		res := env.signIn("katherine@example.test", map[string]any{
			"given_name": nil, "family_name": nil,
		})
		assertLandedInConsole(t, res, "/")

		admin := env.adminFor("katherine@example.test")
		require.NotNil(t, admin.FirstName)
		require.NotNil(t, admin.LastName)
		assert.Equal(t, "Katherine", *admin.FirstName)
		assert.Equal(t, "Johnson", *admin.LastName)
	})

	// A literal null is silence, not a negative answer. Reading it as one
	// fetched UserInfo and then discarded what it returned, so the login stayed
	// unverified and never linked to an account that already existed.
	t.Run("a null attestation is filled in from userinfo", func(t *testing.T) {
		env.provider.userInfo = map[string]any{
			"sub":            "subject-barbara@example.test",
			"email":          "barbara@example.test",
			"email_verified": true,
		}
		res := env.signIn("barbara@example.test", map[string]any{"email_verified": literalNull{}})
		assertLandedInConsole(t, res, "/")

		identity, err := env.state.GetAdminIdentity(context.Background(), env.provider.issuer(), "subject-barbara@example.test")
		require.NoError(t, err)
		assert.True(t, identity.EmailVerified)
	})

	// An attestation with no address attests nothing. Letting the ID token's
	// email_verified vouch for whatever UserInfo then supplies would hand an
	// unattested address to linkByEmail, which attaches logins to existing
	// admins by exactly that.
	t.Run("an attestation with no address does not vouch for the userinfo one", func(t *testing.T) {
		env.provider.userInfo = map[string]any{
			"sub":   "subject-mary@example.test",
			"email": "mary@example.test",
		}
		res := env.signIn("mary@example.test", map[string]any{"email": nil, "email_verified": true})
		assertLandedInConsole(t, res, "/")

		identity, err := env.state.GetAdminIdentity(context.Background(), env.provider.issuer(), "subject-mary@example.test")
		require.NoError(t, err)
		assert.False(t, identity.EmailVerified,
			"only userinfo can attest the address only userinfo supplied")
	})

	t.Run("userinfo supplying both halves is trusted", func(t *testing.T) {
		env.provider.userInfo = map[string]any{
			"sub":            "subject-dorothy@example.test",
			"email":          "dorothy@example.test",
			"email_verified": true,
		}
		res := env.signIn("dorothy@example.test", map[string]any{"email": nil, "email_verified": nil})
		assertLandedInConsole(t, res, "/")

		identity, err := env.state.GetAdminIdentity(context.Background(), env.provider.issuer(), "subject-dorothy@example.test")
		require.NoError(t, err)
		assert.True(t, identity.EmailVerified)
	})

	// An attestation is only about the address it accompanies.
	t.Run("an attestation for another address does not vouch for the token's", func(t *testing.T) {
		env.provider.userInfo = map[string]any{
			"sub":            "subject-edsger@example.test",
			"email":          "somebody.else@example.test",
			"email_verified": true,
		}
		res := env.signIn("edsger@example.test", map[string]any{"email_verified": nil})
		assertLandedInConsole(t, res, "/")

		identity, err := env.state.GetAdminIdentity(context.Background(), env.provider.issuer(), "subject-edsger@example.test")
		require.NoError(t, err)
		assert.False(t, identity.EmailVerified,
			"the address the login used was never attested, so it must not link by email")
	})

	// Otherwise the endpoint would be a way to attach one person's address --
	// and so their account -- to another person's authenticated subject.
	t.Run("a response describing another subject is refused", func(t *testing.T) {
		env.provider.userInfo = map[string]any{
			"sub":            "somebody-else",
			"email":          "victim@example.test",
			"email_verified": true,
		}
		assertRejected(t, env.signIn("swap@example.test", map[string]any{"email": ""}), "failed")

		_, err := env.state.ResolveAdminByEmail(context.Background(), "victim@example.test")
		assert.Error(t, err)
	})
}

// A browser may have more than one login outstanding: two tabs, or a retry
// after a back button. A binding minted per flow made the second /start
// overwrite the first, so the first callback -- from the same person, in the
// same browser -- was refused as coming from somewhere else.
func TestFederatedLoginToleratesTwoLoginsInOneBrowser(t *testing.T) {
	t.Parallel()
	env := newOIDCEnv(t)

	firstState, firstNonce, binding := env.begin("")
	secondState, secondNonce, reused := env.beginCarrying("", binding)
	assert.Equal(t, binding.Value, reused.Value, "the browser keeps one binding across its flows")

	// The older tab finishes first.
	env.provider.claims = env.baseClaims("first@example.test", firstNonce)
	assertLandedInConsole(t, env.callback(url.Values{"code": {"c"}, "state": {firstState}}, binding), "/")

	// And the newer one still can: completing the first must not have taken the
	// binding the second still needs.
	env.provider.claims = env.baseClaims("second@example.test", secondNonce)
	assertLandedInConsole(t, env.callback(url.Values{"code": {"c"}, "state": {secondState}}, binding), "/")
}

func TestFederatedLoginRejections(t *testing.T) {
	t.Parallel()
	env := newOIDCEnv(t)

	// The state is a bearer value. Without the binding, somebody could
	// authenticate as themselves, stop before following the callback, and hand
	// that URL to another person, whose browser would be given the attacker's
	// session.
	t.Run("a callback answered in another browser is refused", func(t *testing.T) {
		state, nonce, _ := env.begin("")
		env.provider.claims = env.baseClaims("victim@example.test", nonce)

		assertRejected(t, env.callback(url.Values{
			"code": {"stub-code"}, "state": {state},
		}, nil), "expired")

		_, err := env.state.ResolveAdminByEmail(context.Background(), "victim@example.test")
		assert.Error(t, err, "no session, and no admin either")
	})

	t.Run("a callback carrying somebody else's binding is refused", func(t *testing.T) {
		_, _, other := env.begin("")
		state, nonce, _ := env.begin("")
		env.provider.claims = env.baseClaims("swapped@example.test", nonce)

		assertRejected(t, env.callback(url.Values{
			"code": {"stub-code"}, "state": {state},
		}, other), "expired")
	})

	t.Run("a replayed state opens no second session", func(t *testing.T) {
		state, nonce, binding := env.begin("")
		env.provider.claims = env.baseClaims("replay@example.test", nonce)

		query := url.Values{"code": {"stub-code"}, "state": {state}}
		assertLandedInConsole(t, env.callback(query, binding), "/")

		assertRejected(t, env.callback(query, binding), "expired")
	})

	t.Run("an unknown state is refused", func(t *testing.T) {
		_, _, binding := env.begin("")
		assertRejected(t, env.callback(url.Values{
			"code": {"stub-code"}, "state": {"never-issued"},
		}, binding), "expired")
	})

	t.Run("a nonce that answers no login is refused", func(t *testing.T) {
		state, _, binding := env.begin("")
		env.provider.claims = env.baseClaims("nonce@example.test", "some-other-nonce")

		assertRejected(t, env.callback(url.Values{
			"code": {"stub-code"}, "state": {state},
		}, binding), "failed")
	})

	t.Run("a token minted for another audience is refused", func(t *testing.T) {
		assertRejected(t, env.signIn("aud@example.test", map[string]any{"aud": "somebody-elses-client"}), "failed")
	})

	t.Run("a token from another issuer is refused", func(t *testing.T) {
		assertRejected(t, env.signIn("iss@example.test", map[string]any{"iss": "https://impostor.test"}), "failed")
	})

	t.Run("an expired token is refused", func(t *testing.T) {
		assertRejected(t, env.signIn("expired@example.test", map[string]any{
			"exp": time.Now().Add(-time.Minute).Unix(),
		}), "failed")
	})

	t.Run("a token carrying no address is refused", func(t *testing.T) {
		assertRejected(t, env.signIn("noemail@example.test", map[string]any{"email": ""}), "email")
	})

	t.Run("a token response with no id_token is refused", func(t *testing.T) {
		env.provider.omitIDToken = true
		defer func() { env.provider.omitIDToken = false }()

		assertRejected(t, env.signIn("noid@example.test", nil), "failed")
	})

	t.Run("a provider that refuses the request is reported as refused", func(t *testing.T) {
		state, _, binding := env.begin("")
		assertRejected(t, env.callback(url.Values{
			"error": {"access_denied"}, "state": {state},
		}, binding), "denied")
	})

	// An authorization error carries a state too, so it is only a denial once
	// the response is one this deployment issued to this browser.
	t.Run("a denial on a state nobody issued is not reported as a denial", func(t *testing.T) {
		assertRejected(t, env.callback(url.Values{
			"error": {"access_denied"}, "state": {"never-issued"},
		}, nil), "expired")
	})

	t.Run("a denial answered in another browser is not reported as a denial", func(t *testing.T) {
		state, _, _ := env.begin("")
		assertRejected(t, env.callback(url.Values{
			"error": {"access_denied"}, "state": {state},
		}, nil), "expired")
	})
}

// An identity provider that lets a user type any address into their profile
// must not be able to inherit an existing account by claiming its address.
func TestFederatedLoginDoesNotLinkOnAnUnverifiedAddress(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	env := newOIDCEnv(t)

	orgID, err := env.state.CreateOrganization(ctx, "Federated Organization")
	require.NoError(t, err)

	// An account that already exists, held by somebody else entirely.
	existing, err := env.state.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID,
		Email:          "victim@example.test",
		Role:           rbac.OrganizationOwner,
	})
	require.NoError(t, err)

	// Provisioning is refused too, and the refusal comes from the exchange: the
	// address is already somebody's, and an unverified claim to it links to
	// nobody, so there is nothing left to do but conflict.
	assertRejected(t, env.signIn("victim@example.test", map[string]any{"email_verified": false}), "exchange")

	identities, err := env.state.ListAdminIdentities(ctx, existing)
	require.NoError(t, err)
	assert.Empty(t, identities, "an unproved address must not attach an identity to an existing account")
}

// Both federated endpoints are absent on a deployment that did not configure the
// driver, rather than half-working against settings nobody supplied.
func TestFederatedLoginIsAbsentWithoutTheDriver(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	state := management.NewState(mgmtDB)

	cfg := config.Node{
		Auth: config.Auth{Drivers: []string{"basic"}, Basic: config.BasicAuth{Email: "a@b", Password: "c"}},
		Mail: testMailConfig(),
	}
	_, dispatcher, renderer := testMailer(t, cfg)

	controller, err := NewAuthController(logger, mgmtDB, state, cfg, nil, consoleSignerFor(t), nil, dispatcher, renderer, verifiers.Deps{})
	require.NoError(t, err)

	res := httptest.NewRecorder()
	controller.StartOIDCLogin(res, httptest.NewRequest("GET", "/api/auth/oidc/x/start", nil), "x", oapi.StartOIDCLoginParams{})
	assert.Equal(t, http.StatusNotFound, res.Code)

	res = httptest.NewRecorder()
	controller.CompleteOIDCLogin(res, httptest.NewRequest("GET", "/api/auth/oidc/x/callback", nil), "x", oapi.CompleteOIDCLoginParams{})
	assert.Equal(t, http.StatusNotFound, res.Code)

	res = httptest.NewRecorder()
	controller.ListOIDCProviders(res, httptest.NewRequest("GET", "/api/auth/oidc/providers", nil))
	assert.Equal(t, http.StatusNotFound, res.Code)

	methodsRes := httptest.NewRecorder()
	controller.GetAuthMethods(methodsRes, httptest.NewRequest("GET", "/api/auth/methods", nil))
	var drivers []string
	require.NoError(t, json.Unmarshal(methodsRes.Body.Bytes(), &drivers))
	assert.NotContains(t, drivers, verifiers.OIDCDriver)
}

// AUTH_DRIVER naming the driver without the settings behind it is refused at
// boot: the failure is a variable nobody set, and it should read that way rather
// than as a broken login.
func TestFederatedLoginRefusesToStartUnconfigured(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)

	_, err := NewAuthController(logger, mgmtDB, management.NewState(mgmtDB), config.Node{
		Auth: config.Auth{Drivers: []string{verifiers.OIDCDriver}},
	}, nil, consoleSignerFor(t), nil, nil, nil, verifiers.Deps{})
	require.ErrorIs(t, err, verifiers.ErrOIDCNotConfigured)
}
