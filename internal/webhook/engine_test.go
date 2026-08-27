package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// testEvent is registered once for the engine tests so they do not depend on
// the payload shape of a real product event.
var testEvent = MustRegister(Definition{Name: "test.event", Version: "v1"})

func engineFor(t *testing.T, yamlConfig string) *Engine {
	t.Helper()
	cfg, err := ParseConfig([]byte(yamlConfig), "")
	require.NoError(t, err)
	engine, err := New(cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	return engine
}

// hookYAML builds a one-hook config for url with the given extra settings.
func hookYAML(url string, extra string) string {
	return bodyHookYAML(url, `function(ctx) { event: ctx.event, payload: ctx.payload }`, extra)
}

func bodyHookYAML(url, body, extra string) string {
	return fmt.Sprintf(`version: v1
defaults:
  timeout: 2s
  network: {allow_private: true, allow_http: true}
  retry: {max_attempts: 2, initial_interval: 1ms, max_interval: 2ms}
hooks:
  test.event:
    - id: main
      url: %s
      body: '%s'
%s`, url, body, extra)
}

func TestDispatchDeliversRenderedBody(t *testing.T) {
	t.Parallel()

	var body []byte
	var event string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = readAll(r)
		event = r.Header.Get("X-Webhook-Event")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	engine := engineFor(t, hookYAML(server.URL, ""))
	results, err := engine.Dispatch(t.Context(), testEvent.Occurred(map[string]any{"n": 1}))
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, http.StatusOK, results[0].StatusCode)
	assert.Equal(t, "test.event", event)
	assert.JSONEq(t, `{"event":"test.event","payload":{"n":1}}`, string(body))
}

func TestDispatchIncludesActor(t *testing.T) {
	t.Parallel()

	orgID := uuid.New()
	var body []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = readAll(r)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	engine := engineFor(t, bodyHookYAML(server.URL, "function(ctx) { actor: ctx.actor }", ""))

	actor := rbac.NewActor(rbac.ActorAdmin, "admin-7", rbac.WithOrganizationID(orgID))
	ctx := rbac.WithActor(t.Context(), actor)

	_, err := engine.Dispatch(ctx, testEvent.Occurred(map[string]any{}))
	require.NoError(t, err)

	var got struct {
		Actor Actor `json:"actor"`
	}
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, "admin", got.Actor.Type)
	assert.Equal(t, "admin-7", got.Actor.ID)
	assert.Equal(t, orgID.String(), got.Actor.OrganizationID)
	assert.Empty(t, got.Actor.ProjectID, "the nil project id renders as an empty string, not a nil uuid")
}

func TestCanInterrupt(t *testing.T) {
	t.Parallel()

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer failing.Close()

	t.Run("true fails the dispatch", func(t *testing.T) {
		engine := engineFor(t, hookYAML(failing.URL, "      can_interrupt: true\n"))
		results, err := engine.Dispatch(t.Context(), testEvent.Occurred(map[string]any{}))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "test.event/main")
		require.Len(t, results, 1)
		assert.Equal(t, http.StatusInternalServerError, results[0].StatusCode)
	})

	t.Run("false swallows the failure", func(t *testing.T) {
		engine := engineFor(t, hookYAML(failing.URL, "      can_interrupt: false\n"))
		results, err := engine.Dispatch(t.Context(), testEvent.Occurred(map[string]any{}))
		require.NoError(t, err)
		require.Len(t, results, 1)
		require.Error(t, results[0].Err, "the failure is still reported in the results")
	})
}

func TestHooksRunSequentiallyAndStopAtAnInterrupt(t *testing.T) {
	t.Parallel()

	var order atomic.Value
	order.Store([]string{})
	record := func(id string) {
		order.Store(append(order.Load().([]string), id))
	}

	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		record("first")
		w.WriteHeader(http.StatusOK)
	}))
	defer first.Close()
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		record("second")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer second.Close()
	third := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		record("third")
		w.WriteHeader(http.StatusOK)
	}))
	defer third.Close()

	cfg := fmt.Sprintf(`version: v1
defaults:
  timeout: 2s
  network: {allow_private: true, allow_http: true}
  retry: {max_attempts: 1, initial_interval: 1ms, max_interval: 1ms}
hooks:
  test.event:
    - {id: first, url: %s, body: 'function(ctx) {}'}
    - {id: second, url: %s, body: 'function(ctx) {}', can_interrupt: true}
    - {id: third, url: %s, body: 'function(ctx) {}'}
`, first.URL, second.URL, third.URL)

	engine := engineFor(t, cfg)
	results, err := engine.Dispatch(t.Context(), testEvent.Occurred(map[string]any{}))
	require.Error(t, err)

	assert.Equal(t, []string{"first", "second"}, order.Load().([]string),
		"hooks run in declaration order and stop once the operation is doomed")
	require.Len(t, results, 2)
	assert.Equal(t, "first", results[0].HookID)
	assert.Equal(t, "second", results[1].HookID)
}

func TestResponseParse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"decision":"allow","quota":5}`))
	}))
	defer server.Close()

	t.Run("captures the body", func(t *testing.T) {
		engine := engineFor(t, hookYAML(server.URL, "      can_interrupt: true\n      response: {parse: true}\n"))
		results, err := engine.Dispatch(t.Context(), testEvent.Occurred(map[string]any{}))
		require.NoError(t, err)

		parsed, ok := results.Parsed("main")
		require.True(t, ok)
		assert.JSONEq(t, `{"decision":"allow","quota":5}`, string(parsed))
	})

	t.Run("is not captured by default", func(t *testing.T) {
		engine := engineFor(t, hookYAML(server.URL, ""))
		results, err := engine.Dispatch(t.Context(), testEvent.Occurred(map[string]any{}))
		require.NoError(t, err)
		_, ok := results.Parsed("main")
		assert.False(t, ok)
	})
}

func TestResponseParseRejectsNonJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>not json</html>"))
	}))
	defer server.Close()

	engine := engineFor(t, hookYAML(server.URL, "      can_interrupt: true\n      response: {parse: true}\n"))
	_, err := engine.Dispatch(t.Context(), testEvent.Occurred(map[string]any{}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not valid json")
}

func TestResponseIgnore(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	engine := engineFor(t, hookYAML(server.URL, "      can_interrupt: true\n      response: {ignore: true}\n"))
	results, err := engine.Dispatch(t.Context(), testEvent.Occurred(map[string]any{}))
	require.NoError(t, err, "an ignored response cannot fail the operation, even at 500")
	require.Len(t, results, 1)
	assert.NoError(t, results[0].Err)
}

func TestForwardHeadersRequireBothConfigAndCallSite(t *testing.T) {
	t.Parallel()

	var seen string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	inbound := httptest.NewRequest(http.MethodPost, "/projects", nil)
	inbound.Header.Set("Authorization", "Bearer caller-token")
	inbound.Header.Set("Cookie", "session=abc")

	t.Run("neither forwards nothing", func(t *testing.T) {
		engine := engineFor(t, hookYAML(server.URL, ""))
		_, err := engine.Dispatch(t.Context(), testEvent.Occurred(map[string]any{}))
		require.NoError(t, err)
		assert.Empty(t, seen)
	})

	t.Run("config alone forwards nothing", func(t *testing.T) {
		seen = ""
		engine := engineFor(t, hookYAML(server.URL, "      forward_headers: [Authorization]\n"))
		_, err := engine.Dispatch(t.Context(), testEvent.Occurred(map[string]any{}))
		require.NoError(t, err)
		assert.Empty(t, seen, "without WithInboundRequest there is nothing to forward from")
	})

	t.Run("call site alone forwards nothing", func(t *testing.T) {
		seen = ""
		engine := engineFor(t, hookYAML(server.URL, ""))
		_, err := engine.Dispatch(t.Context(), testEvent.Occurred(map[string]any{}), WithInboundRequest(inbound))
		require.NoError(t, err)
		assert.Empty(t, seen, "without an allowlist no header is copied")
	})

	t.Run("both forward only the allowlisted header", func(t *testing.T) {
		seen = ""
		var cookie string
		allowlisted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			seen = r.Header.Get("Authorization")
			cookie = r.Header.Get("Cookie")
			w.WriteHeader(http.StatusOK)
		}))
		defer allowlisted.Close()

		engine := engineFor(t, hookYAML(allowlisted.URL, "      forward_headers: [authorization]\n"))
		_, err := engine.Dispatch(t.Context(), testEvent.Occurred(map[string]any{}), WithInboundRequest(inbound))
		require.NoError(t, err)
		assert.Equal(t, "Bearer caller-token", seen, "header names are matched case-insensitively")
		assert.Empty(t, cookie, "headers outside the allowlist are never copied")
	})
}

func TestDispatchIsBoundedByTheDispatchBudget(t *testing.T) {
	t.Parallel()

	// Every attempt outlives the per-attempt timeout, so the only thing that can
	// end the sequence is the dispatch budget.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(150 * time.Millisecond)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := fmt.Sprintf(`version: v1
defaults:
  timeout: 50ms
  max_dispatch_time: 200ms
  network: {allow_private: true, allow_http: true}
  retry: {max_attempts: 100, initial_interval: 1ms, max_interval: 1ms}
hooks:
  test.event:
    - {id: main, url: %s, body: 'function(ctx) {}', can_interrupt: true}
`, server.URL)

	engine := engineFor(t, cfg)

	start := time.Now()
	_, err := engine.Dispatch(t.Context(), testEvent.Occurred(map[string]any{}))
	require.Error(t, err)
	assert.Less(t, time.Since(start), time.Second,
		"attempts x timeout must not exceed the dispatch budget")
}

func TestDispatchOnNilAndUnboundEvents(t *testing.T) {
	t.Parallel()

	var nilEngine *Engine
	results, err := nilEngine.Dispatch(t.Context(), testEvent.Occurred(nil))
	require.NoError(t, err)
	assert.Nil(t, results)
	assert.False(t, nilEngine.Enabled("test.event"))

	engine := engineFor(t, "version: v1\n")
	results, err = engine.Dispatch(t.Context(), testEvent.Occurred(nil))
	require.NoError(t, err)
	assert.Nil(t, results)
	assert.False(t, engine.Enabled("test.event"))
}

func TestEngineRejectsUnsafeURL(t *testing.T) {
	t.Parallel()

	cfg, err := ParseConfig([]byte(`version: v1
hooks:
  test.event:
    - {id: main, url: "https://169.254.169.254/latest/meta-data", body: 'function(ctx) {}'}
`), "")
	require.NoError(t, err)

	_, err = New(cfg, zaptest.NewLogger(t))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not a public address")
}

func readAll(r *http.Request) ([]byte, error) {
	var sb strings.Builder
	buf := make([]byte, 512)
	for {
		n, err := r.Body.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			return []byte(sb.String()), nil
		}
	}
}
