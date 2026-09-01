package sso

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lunogram/platform/internal/ssrf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The stub provider is an httptest server on loopback, so the tests run under
// the relaxations an operator would have to opt into explicitly.
var testPolicy = ssrf.Policy{AllowPrivate: true, AllowHTTP: true}

func discoveryServer(t *testing.T, document func(issuer string) map[string]any) *httptest.Server {
	t.Helper()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(document(server.URL))
	}))
	t.Cleanup(server.Close)
	return server
}

func completeDocument(issuer string) map[string]any {
	return map[string]any{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/authorize",
		"token_endpoint":                        issuer + "/token",
		"jwks_uri":                              issuer + "/jwks",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	}
}

func TestDiscoveryMetadata(t *testing.T) {
	t.Parallel()

	t.Run("reads the endpoints an issuer publishes", func(t *testing.T) {
		server := discoveryServer(t, completeDocument)
		discovery := NewDiscovery(server.Client(), testPolicy, time.Minute)

		metadata, err := discovery.Metadata(context.Background(), DefaultDiscoveryURL(server.URL), server.URL)
		require.NoError(t, err)
		assert.Equal(t, server.URL+"/token", metadata.TokenEndpoint)
		assert.Equal(t, server.URL+"/jwks", metadata.JWKSURI)
	})

	// Whoever chooses the discovery URL chooses the token endpoint and the JWKS,
	// which is to say they can mint an ID token for any issuer they like. Both
	// bindings are what stop that.
	t.Run("a document naming another issuer is refused", func(t *testing.T) {
		server := discoveryServer(t, func(string) map[string]any {
			return completeDocument("https://impostor.test")
		})
		discovery := NewDiscovery(server.Client(), testPolicy, time.Minute)

		_, err := discovery.Metadata(context.Background(), DefaultDiscoveryURL(server.URL), server.URL)
		assert.ErrorIs(t, err, ErrIssuerMismatch)
	})

	t.Run("a document served by another origin is refused", func(t *testing.T) {
		server := discoveryServer(t, completeDocument)
		discovery := NewDiscovery(server.Client(), testPolicy, time.Minute)

		_, err := discovery.Metadata(context.Background(),
			"https://elsewhere.test/.well-known/openid-configuration", server.URL)
		assert.ErrorIs(t, err, ErrDiscoveryOrigin)
	})

	// The document names the token endpoint and the JWKS, so an endpoint it
	// advertises is as sensitive as one an operator configured and is held to
	// the same outbound policy.
	t.Run("a document advertising an endpoint the policy refuses is refused", func(t *testing.T) {
		server := discoveryServer(t, func(issuer string) map[string]any {
			document := completeDocument(issuer)
			document["jwks_uri"] = "http://169.254.169.254/latest/meta-data/"
			return document
		})
		discovery := NewDiscovery(server.Client(), testPolicy, time.Minute)

		_, err := discovery.Metadata(context.Background(), DefaultDiscoveryURL(server.URL), server.URL)
		assert.ErrorIs(t, err, ErrDiscoveryInsecure)
	})

	// Two issuers may publish from one origin, and a document cached for one of
	// them has only ever been checked against that one.
	t.Run("a cached document is not served to another issuer", func(t *testing.T) {
		server := discoveryServer(t, completeDocument)
		discovery := NewDiscovery(server.Client(), testPolicy, time.Minute)

		_, err := discovery.Metadata(context.Background(), DefaultDiscoveryURL(server.URL), server.URL)
		require.NoError(t, err)

		_, err = discovery.Metadata(context.Background(), DefaultDiscoveryURL(server.URL), server.URL+"/tenant")
		assert.ErrorIs(t, err, ErrIssuerMismatch)
	})

	t.Run("a document missing an endpoint the flow needs is refused", func(t *testing.T) {
		server := discoveryServer(t, func(issuer string) map[string]any {
			document := completeDocument(issuer)
			delete(document, "jwks_uri")
			return document
		})
		discovery := NewDiscovery(server.Client(), testPolicy, time.Minute)

		_, err := discovery.Metadata(context.Background(), DefaultDiscoveryURL(server.URL), server.URL)
		assert.ErrorIs(t, err, ErrDiscoveryIncomplete)
	})
}

func TestValidateDiscoveryURL(t *testing.T) {
	t.Parallel()

	strict := NewDiscovery(http.DefaultClient, ssrf.Policy{}, 0)

	assert.NoError(t, strict.ValidateDiscoveryURL(
		"https://idp.test/.well-known/openid-configuration", "https://idp.test"))
	assert.NoError(t, strict.ValidateDiscoveryURL(
		"https://idp.test/tenant/openid-configuration", "https://idp.test/tenant"))
	assert.ErrorIs(t, strict.ValidateDiscoveryURL(
		"https://elsewhere.test/openid-configuration", "https://idp.test"), ErrDiscoveryOrigin)

	// The dialer decides which hosts may be reached and says nothing about the
	// scheme, so plaintext metadata has to be refused here. Whoever can rewrite
	// that response chooses the token endpoint and the JWKS.
	assert.ErrorIs(t, strict.ValidateDiscoveryURL(
		"http://idp.test/openid-configuration", "http://idp.test"), ErrDiscoveryInsecure)
	assert.ErrorIs(t, strict.ValidateDiscoveryURL(
		"https://127.0.0.1/openid-configuration", "https://127.0.0.1"), ErrDiscoveryInsecure)

	// A scheme that differs from the issuer's is still an origin mismatch.
	assert.ErrorIs(t, SameOrigin(
		"http://idp.test/openid-configuration", "https://idp.test"), ErrDiscoveryOrigin)
}

// Spelling out the default port is unusual but legal, and the two URLs are the
// same origin. Refusing them would be refusing a correct configuration.
func TestValidateDiscoveryURLNormalisesTheDefaultPort(t *testing.T) {
	t.Parallel()

	strict := NewDiscovery(http.DefaultClient, ssrf.Policy{}, 0)

	assert.NoError(t, strict.ValidateDiscoveryURL(
		"https://idp.test:443/.well-known/openid-configuration", "https://idp.test"))
	assert.NoError(t, strict.ValidateDiscoveryURL(
		"https://idp.test/.well-known/openid-configuration", "https://idp.test:443"))
	assert.NoError(t, strict.ValidateDiscoveryURL(
		"https://IDP.test/.well-known/openid-configuration", "https://idp.TEST"))

	// A different port is a different origin, default or not.
	assert.ErrorIs(t, strict.ValidateDiscoveryURL(
		"https://idp.test:8443/.well-known/openid-configuration", "https://idp.test"), ErrDiscoveryOrigin)
}

// Whichever login happens to lead the singleflight may have its browser closed
// mid-flight. With the leader's context that cancellation failed every other
// login that had coalesced behind it.
func TestDiscoveryFetchOutlivesTheCallerThatLedIt(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(completeDocument(server.URL))
	}))
	t.Cleanup(server.Close)

	discovery := NewDiscovery(server.Client(), testPolicy, time.Minute)

	leader, abandon := context.WithCancel(context.Background())
	leaderDone := make(chan error, 1)
	go func() {
		_, err := discovery.Metadata(leader, DefaultDiscoveryURL(server.URL), server.URL)
		leaderDone <- err
	}()

	// The leader gives up while the fetch is in flight; the document is only
	// then allowed to arrive.
	abandon()
	assert.ErrorIs(t, <-leaderDone, context.Canceled, "the caller stops waiting on its own context")
	close(release)

	metadata, err := discovery.Metadata(context.Background(), DefaultDiscoveryURL(server.URL), server.URL)
	require.NoError(t, err, "an abandoned leader must not poison the shared fetch")
	assert.Equal(t, server.URL+"/token", metadata.TokenEndpoint)
}

func TestDefaultDiscoveryURL(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "https://idp.test/.well-known/openid-configuration", DefaultDiscoveryURL("https://idp.test"))
	assert.Equal(t, "https://idp.test/.well-known/openid-configuration", DefaultDiscoveryURL("https://idp.test/"))
}
