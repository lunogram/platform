package outbound

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func strategyConfig(t *testing.T, body string) AuthConfig {
	t.Helper()
	var cfg AuthConfig
	require.NoError(t, yaml.Unmarshal([]byte(body), &cfg))
	return cfg
}

func TestAPIKeyStrategy(t *testing.T) {
	t.Parallel()

	t.Run("header", func(t *testing.T) {
		t.Parallel()
		strategy, err := BuildStrategy(strategyConfig(t, "type: api_key\nconfig:\n  in: header\n  name: X-Token\n  value: secret\n"), StrategyDeps{})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "https://example.com", nil)
		require.NoError(t, strategy.Apply(t.Context(), req))
		assert.Equal(t, "secret", req.Header.Get("X-Token"))
	})

	t.Run("cookie", func(t *testing.T) {
		t.Parallel()
		strategy, err := BuildStrategy(strategyConfig(t, "type: api_key\nconfig:\n  in: cookie\n  name: session\n  value: secret\n"), StrategyDeps{})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "https://example.com", nil)
		require.NoError(t, strategy.Apply(t.Context(), req))
		cookie, err := req.Cookie("session")
		require.NoError(t, err)
		assert.Equal(t, "secret", cookie.Value)
	})

	t.Run("defaults to header", func(t *testing.T) {
		t.Parallel()
		strategy, err := BuildStrategy(strategyConfig(t, "type: api_key\nconfig:\n  name: X-Token\n  value: secret\n"), StrategyDeps{})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "https://example.com", nil)
		require.NoError(t, strategy.Apply(t.Context(), req))
		assert.Equal(t, "secret", req.Header.Get("X-Token"))
	})

	t.Run("rejects incomplete config", func(t *testing.T) {
		t.Parallel()
		for _, body := range []string{
			"type: api_key\nconfig:\n  name: X-Token\n", // no value
			"type: api_key\nconfig:\n  value: secret\n", // no name
			"type: api_key\n", // no config block
			"type: api_key\nconfig:\n  in: query\n  name: a\n  value: b\n", // unsupported location
			"type: api_key\nconfig:\n  nmae: X-Token\n  value: b\n",        // typo, unknown field
		} {
			_, err := BuildStrategy(strategyConfig(t, body), StrategyDeps{})
			assert.Error(t, err, body)
		}
	})
}

func TestBasicAuthStrategy(t *testing.T) {
	t.Parallel()

	strategy, err := BuildStrategy(strategyConfig(t, "type: basic_auth\nconfig:\n  user: alice\n  password: hunter2\n"), StrategyDeps{})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "https://example.com", nil)
	require.NoError(t, strategy.Apply(t.Context(), req))

	user, password, ok := req.BasicAuth()
	require.True(t, ok)
	assert.Equal(t, "alice", user)
	assert.Equal(t, "hunter2", password)
}

func TestUnknownStrategy(t *testing.T) {
	t.Parallel()

	_, err := BuildStrategy(strategyConfig(t, "type: mtls\n"), StrategyDeps{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown auth type")
}

func TestNoStrategy(t *testing.T) {
	t.Parallel()

	for _, body := range []string{"type: none\n", "{}\n"} {
		strategy, err := BuildStrategy(strategyConfig(t, body), StrategyDeps{})
		require.NoError(t, err)
		assert.Nil(t, strategy)
	}
}

func TestOAuth2ClientCredentials(t *testing.T) {
	t.Parallel()

	t.Run("fetches, caches and refreshes", func(t *testing.T) {
		t.Parallel()

		var fetches atomic.Int64
		var lifetime atomic.Int64
		lifetime.Store(3600)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fetches.Add(1)
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "client_credentials", r.Form.Get("grant_type"))
			assert.Equal(t, "a b", r.Form.Get("scope"))

			user, secret, ok := r.BasicAuth()
			require.True(t, ok)
			assert.Equal(t, "id", user)
			assert.Equal(t, "shh", secret)

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "token-" + strconv.FormatInt(fetches.Load(), 10),
				"expires_in":   lifetime.Load(),
			})
		}))
		defer server.Close()

		strategy, err := BuildStrategy(AuthConfig{
			Type:   "oauth2_client_credentials",
			Config: yamlNode(t, map[string]any{"token_url": server.URL, "client_id": "id", "client_secret": "shh", "scopes": []string{"a", "b"}}),
		}, StrategyDeps{HTTPClient: server.Client()})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "https://example.com", nil)
		require.NoError(t, strategy.Apply(t.Context(), req))
		assert.Equal(t, "Bearer token-1", req.Header.Get("Authorization"))

		// A live token is reused rather than re-fetched.
		req2 := httptest.NewRequest(http.MethodPost, "https://example.com", nil)
		require.NoError(t, strategy.Apply(t.Context(), req2))
		assert.Equal(t, "Bearer token-1", req2.Header.Get("Authorization"))
		assert.EqualValues(t, 1, fetches.Load())

		// A token whose stated lifetime is inside the expiry leeway is never
		// cached, so the next call fetches again.
		oauth := strategy.(*OAuth2ClientCredentials)
		oauth.mu.Lock()
		oauth.expiresAt = time.Now().Add(-time.Second)
		oauth.mu.Unlock()

		req3 := httptest.NewRequest(http.MethodPost, "https://example.com", nil)
		require.NoError(t, strategy.Apply(t.Context(), req3))
		assert.Equal(t, "Bearer token-2", req3.Header.Get("Authorization"))
		assert.EqualValues(t, 2, fetches.Load())
	})

	t.Run("short-lived tokens are not cached", func(t *testing.T) {
		t.Parallel()

		var fetches atomic.Int64
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fetches.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "t", "expires_in": 5})
		}))
		defer server.Close()

		strategy, err := BuildStrategy(AuthConfig{
			Type:   "oauth2_client_credentials",
			Config: yamlNode(t, map[string]any{"token_url": server.URL, "client_id": "id", "client_secret": "shh"}),
		}, StrategyDeps{HTTPClient: server.Client()})
		require.NoError(t, err)

		for range 3 {
			req := httptest.NewRequest(http.MethodPost, "https://example.com", nil)
			require.NoError(t, strategy.Apply(t.Context(), req))
		}
		assert.EqualValues(t, 3, fetches.Load())
	})

	t.Run("token endpoint failure does not leak the body", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_client","client_secret":"shh"}`))
		}))
		defer server.Close()

		strategy, err := BuildStrategy(AuthConfig{
			Type:   "oauth2_client_credentials",
			Config: yamlNode(t, map[string]any{"token_url": server.URL, "client_id": "id", "client_secret": "shh"}),
		}, StrategyDeps{HTTPClient: server.Client()})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "https://example.com", nil)
		err = strategy.Apply(t.Context(), req)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "status 401")
		assert.NotContains(t, err.Error(), "shh")
	})

	t.Run("requires an http client", func(t *testing.T) {
		t.Parallel()
		_, err := BuildStrategy(AuthConfig{
			Type:   "oauth2_client_credentials",
			Config: yamlNode(t, map[string]any{"token_url": "https://idp.example.com/token", "client_id": "id", "client_secret": "shh"}),
		}, StrategyDeps{})
		require.Error(t, err)
	})
}

func TestOAuth2AppliesToRealRequest(t *testing.T) {
	t.Parallel()

	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "abc", "expires_in": 3600})
	}))
	defer idp.Close()

	var seen string
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer destination.Close()

	client, err := NewClient(Options{
		Timeout: time.Second,
		Network: Network{AllowPrivate: true, AllowHTTP: true},
		Auth: AuthConfig{
			Type:   "oauth2_client_credentials",
			Config: yamlNode(t, map[string]any{"token_url": idp.URL, "client_id": "id", "client_secret": "shh"}),
		},
	})
	require.NoError(t, err)

	_, err = client.Do(t.Context(), Request{Method: http.MethodPost, URL: destination.URL})
	require.NoError(t, err)
	assert.Equal(t, "Bearer abc", seen)
}

func yamlNode(t *testing.T, value any) yaml.Node {
	t.Helper()
	raw, err := yaml.Marshal(value)
	require.NoError(t, err)
	var node yaml.Node
	require.NoError(t, yaml.Unmarshal(raw, &node))
	require.NotEmpty(t, node.Content)
	return *node.Content[0]
}
