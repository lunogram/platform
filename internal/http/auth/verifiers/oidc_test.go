package verifiers

import (
	"net/http"
	"testing"

	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/jwks"
	"github.com/lunogram/platform/internal/sso"
	"github.com/lunogram/platform/internal/ssrf"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/oauth2"
)

// openid is what the token response carries an id_token because of, so it is
// added rather than left to an operator who listed scopes and forgot it.
func TestWithOpenIDScope(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"openid"}, withOpenIDScope(nil))
	assert.Equal(t, []string{"openid", "email"}, withOpenIDScope([]string{"email"}))
	assert.Equal(t, []string{"email", "openid"}, withOpenIDScope([]string{"email", "openid", "email"}),
		"a listed openid keeps its place and duplicates are dropped")
}

// A deployment that names the driver without configuring it is refused at boot,
// so the failure reads as a variable nobody set rather than as a broken login.
func TestNewOIDCRequiresItsSettings(t *testing.T) {
	t.Parallel()

	_, err := NewOIDC(testOIDCOptions(t, config.OIDCProvider{ID: "idp", Issuer: "https://idp.test"}))
	require.ErrorIs(t, err, ErrOIDCNotConfigured)
}

// The redirect_uri is derived from the deployment's public URL and never from a
// request parameter, and it is what the operator registers with the provider.
func TestNewOIDCDerivesItsRedirectURI(t *testing.T) {
	t.Parallel()

	verifier, err := NewOIDC(testOIDCOptions(t, config.OIDCProvider{
		ID:           "okta",
		Issuer:       "https://idp.test/",
		ClientID:     "client",
		ClientSecret: "secret",
	}))
	require.NoError(t, err)
	assert.Equal(t, OIDCDriver, verifier.Driver())
	assert.Equal(t, "https://console.example.test/api/auth/oidc/okta/callback", verifier.RedirectURI(),
		"the id is part of the redirect URI the operator registers")
	assert.Equal(t, "https://idp.test/", verifier.config.Issuer,
		"an issuer identifier is compared exactly, trailing slash included")
	assert.Equal(t, "https://idp.test/.well-known/openid-configuration", verifier.config.DiscoveryURL)
}

func testOIDCOptions(t *testing.T, settings config.OIDCProvider) OIDCOptions {
	t.Helper()

	logger := zaptest.NewLogger(t)
	return OIDCOptions{
		Config: settings,
		// Never dialled: these tests only construct the verifier. A nil client
		// would give a nil store, which is what the driver refuses to build on.
		Flows:      sso.NewFlowStore(goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:0"}), "test:"),
		Discovery:  sso.NewDiscovery(http.DefaultClient, ssrf.Policy{}, 0),
		Keys:       jwks.New(jwks.Config{}, nil, nil, logger),
		BaseURL:    "https://console.example.test",
		HTTPClient: http.DefaultClient,
		Logger:     logger,
	}
}

// Whoever chooses the discovery URL chooses the token endpoint and the JWKS, so
// it has to be served by the issuer's own origin.
func TestNewOIDCRefusesAForeignDiscoveryURL(t *testing.T) {
	t.Parallel()

	_, err := NewOIDC(testOIDCOptions(t, config.OIDCProvider{
		ID:           "idp",
		Issuer:       "https://idp.test",
		ClientID:     "client",
		ClientSecret: "secret",
		DiscoveryURL: "https://elsewhere.test/.well-known/openid-configuration",
	}))
	require.ErrorIs(t, err, sso.ErrDiscoveryOrigin)
}

// A provider that only implements client_secret_basic must not have every
// exchange fail because the client pinned client_secret_post. Discovery says an
// absent list means basic, so that is the default rather than an assumption in
// the other direction.
func TestTokenAuthStyle(t *testing.T) {
	t.Parallel()

	assert.Equal(t, oauth2.AuthStyleInHeader, tokenAuthStyle(nil))
	assert.Equal(t, oauth2.AuthStyleInHeader, tokenAuthStyle([]string{"client_secret_basic", "client_secret_post"}))
	assert.Equal(t, oauth2.AuthStyleInParams, tokenAuthStyle([]string{"client_secret_post"}))
	assert.Equal(t, oauth2.AuthStyleAutoDetect, tokenAuthStyle([]string{"private_key_jwt"}))
}

// email_verified attests the standard email claim. An operator who points the
// address somewhere editable -- preferred_username, upn -- must not inherit that
// attestation, because the exchange links accounts by the address it is handed.
func TestNewOIDCPairsVerificationWithTheAddressClaim(t *testing.T) {
	t.Parallel()

	base := config.OIDCProvider{ID: "idp", Issuer: "https://idp.test", ClientID: "client", ClientSecret: "secret"}

	standard := base
	standard.EmailClaim = "email"
	verifier, err := NewOIDC(testOIDCOptions(t, standard))
	require.NoError(t, err)
	assert.Equal(t, "email_verified", verifier.config.EmailVerifiedClaim)

	custom := base
	custom.EmailClaim = "preferred_username"
	verifier, err = NewOIDC(testOIDCOptions(t, custom))
	require.NoError(t, err)
	assert.Empty(t, verifier.config.EmailVerifiedClaim,
		"an address from a non-standard claim is unverified until a verification claim names what attests it")

	paired := base
	paired.EmailClaim = "upn"
	paired.EmailVerifiedClaim = "upn_verified"
	verifier, err = NewOIDC(testOIDCOptions(t, paired))
	require.NoError(t, err)
	assert.Equal(t, "upn_verified", verifier.config.EmailVerifiedClaim)
}

// An allow-list that normalises away would read as "any domain", which is the
// opposite of what somebody who configured one meant.
func TestNewOIDCRefusesAnAllowListThatNamesNothing(t *testing.T) {
	t.Parallel()

	base := config.OIDCProvider{ID: "idp", Issuer: "https://idp.test", ClientID: "c", ClientSecret: "s"}

	blank := base
	blank.AllowedDomains = []string{"", "   "}
	_, err := NewOIDC(testOIDCOptions(t, blank))
	require.ErrorIs(t, err, ErrOIDCNotConfigured)
	assert.Contains(t, err.Error(), "allowed_domains")

	// A stray blank beside a real domain is just a trailing comma.
	tolerant := base
	tolerant.AllowedDomains = []string{"", "Example.TEST"}
	verifier, err := NewOIDC(testOIDCOptions(t, tolerant))
	require.NoError(t, err)
	assert.Equal(t, []string{"example.test"}, verifier.domains)

	// No allow-list at all still means any domain, which is right for a
	// deployment with one provider.
	unbounded, err := NewOIDC(testOIDCOptions(t, base))
	require.NoError(t, err)
	assert.Empty(t, unbounded.domains)
}

// A wiring mistake should point at the collaborator that is actually absent.
func TestNewOIDCNamesTheMissingCollaborator(t *testing.T) {
	t.Parallel()

	opts := testOIDCOptions(t, config.OIDCProvider{
		ID: "idp", Issuer: "https://idp.test", ClientID: "client", ClientSecret: "secret",
	})
	opts.Flows = nil

	_, err := NewOIDC(opts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Redis")
	assert.NotContains(t, err.Error(), "JWKS", "only the absent collaborator is named")
}

func TestSafeRedirectPath(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		redirect string
		want     string
	}{
		{name: "a path", redirect: "/projects/1", want: "/projects/1"},
		{name: "a path with a query", redirect: "/projects?tab=two", want: "/projects?tab=two"},
		// The console preserves the fragment when it builds the redirect, so
		// dropping it here would land somebody on the wrong part of the page --
		// and only when they signed in with this driver.
		{name: "a path with a fragment", redirect: "/settings#keys", want: "/settings#keys"},
		{name: "a path with a query and a fragment", redirect: "/settings?tab=two#keys", want: "/settings?tab=two#keys"},
		{name: "empty", redirect: "", want: "/"},
		{name: "an absolute url", redirect: "https://evil.test/steal", want: "/"},
		{name: "a protocol-relative url", redirect: "//evil.test/steal", want: "/"},
		{name: "a scheme with no slashes", redirect: "javascript:alert(1)", want: "/"},
		{name: "a bare path fragment", redirect: "projects", want: "/"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, SafeRedirectPath(test.redirect))
		})
	}
}

// An issuer advertising a symmetric or "none" algorithm must not talk the
// platform into verifying an ID token against something that is not a signature.
func TestAllowedSigningAlgorithms(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"RS256"}, allowedSigningAlgorithms(nil),
		"the mandatory algorithm is the fallback")
	assert.Equal(t, []string{"RS256"}, allowedSigningAlgorithms([]string{"HS256", "none", "RS256"}))
	assert.Equal(t, []string{"RS256"}, allowedSigningAlgorithms([]string{"HS256", "none"}),
		"a provider advertising nothing usable falls back rather than accepting its list")
	assert.Equal(t, []string{"ES256", "PS512"}, allowedSigningAlgorithms([]string{"ES256", "PS512"}))
}
