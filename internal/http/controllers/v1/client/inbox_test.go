package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testProjectID is the concrete project UUID inlined into client request paths.
// Every authenticated client route is mounted under /api/client/projects/{projectID}.
const testProjectID = "11111111-1111-1111-1111-111111111111"

// clientPath builds a concrete request path under the project prefix from a
// suffix such as "/users/inbox".
func clientPath(suffix string) string {
	return "/api/client/projects/" + testProjectID + suffix
}

// newValidatedRouter creates a chi router with only the OpenAPI spec validation
// middleware. Auth is skipped (NoopAuthenticationFunc) so we can test request
// body validation in isolation. A 200 OK passthrough handler is mounted at the
// chi route pattern for the given suffix (with a {projectID} segment) so that any
// request that passes validation returns 200.
func newValidatedRouter(t *testing.T, method, suffix string) chi.Router {
	t.Helper()

	spec, err := oapi.Spec()
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Use(oapi.Validator(spec, openapi3filter.Options{
		AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
	}))
	router.MethodFunc(method, "/api/client/projects/{projectID}"+suffix, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return router
}

func postJSON(t *testing.T, router chi.Router, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	data, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// validUserInboxMessage returns a minimal valid inbox message payload.
func validUserInboxMessage() map[string]any {
	return map[string]any{
		"target":     []map[string]any{{"external_id": "user_123"}},
		"identifier": map[string]any{"external_id": "message_123"},
		"channel":    "push",
		"content":    map[string]any{"title": "Hello"},
	}
}

// validUserInboxEvent returns a minimal valid inbox state-event payload (the
// shared shape accepted by both /inbox/read and /inbox/archived).
func validUserInboxEvent() map[string]any {
	return map[string]any{
		"target":     []map[string]any{{"external_id": "user_123"}},
		"message_id": "00000000-0000-0000-0000-000000000001",
	}
}

func TestInboxSpecValidation_EmptyTarget(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		path string
		body any
	}{
		"user inbox messages": {
			path: "/users/inbox",
			body: []map[string]any{
				withOverrides(validUserInboxMessage(), "target", []map[string]any{}),
			},
		},
		"user inbox events": {
			path: "/users/inbox/read",
			body: []map[string]any{
				withOverrides(validUserInboxEvent(), "target", []map[string]any{}),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			router := newValidatedRouter(t, http.MethodPost, tc.path)
			w := postJSON(t, router, clientPath(tc.path), tc.body)
			assert.Equal(t, http.StatusBadRequest, w.Code, "empty target should be rejected")
		})
	}
}

func TestInboxSpecValidation_EmptyExternalID(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		path string
		body any
	}{
		"user inbox messages": {
			path: "/users/inbox",
			body: []map[string]any{
				withOverrides(validUserInboxMessage(), "target", []map[string]any{{"external_id": ""}}),
			},
		},
		"user inbox events": {
			path: "/users/inbox/archived",
			body: []map[string]any{
				withOverrides(validUserInboxEvent(), "target", []map[string]any{{"external_id": ""}}),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			router := newValidatedRouter(t, http.MethodPost, tc.path)
			w := postJSON(t, router, clientPath(tc.path), tc.body)
			assert.Equal(t, http.StatusBadRequest, w.Code, "empty external_id should be rejected")
		})
	}
}

func TestInboxSpecValidation_InvalidChannel(t *testing.T) {
	t.Parallel()

	router := newValidatedRouter(t, http.MethodPost, "/users/inbox")
	w := postJSON(t, router, clientPath("/users/inbox"), []map[string]any{
		withOverrides(validUserInboxMessage(), "channel", "carrier_pigeon"),
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, "invalid channel should be rejected")
}

func TestInboxSpecValidation_InvalidPriority(t *testing.T) {
	t.Parallel()

	tests := map[string]int{
		"priority too low":  0,
		"priority too high": 6,
		"negative priority": -1,
	}

	for name, priority := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			msg := validUserInboxMessage()
			msg["priority"] = priority

			router := newValidatedRouter(t, http.MethodPost, "/users/inbox")
			w := postJSON(t, router, clientPath("/users/inbox"), []map[string]any{msg})
			assert.Equal(t, http.StatusBadRequest, w.Code, "invalid priority should be rejected")
		})
	}
}

func TestInboxSpecValidation_EmptyArray(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		path string
	}{
		"user inbox messages": {path: "/users/inbox"},
		"user inbox read":     {path: "/users/inbox/read"},
		"user inbox archived": {path: "/users/inbox/archived"},
		"org inbox read":      {path: "/organizations/inbox/read"},
		"org inbox archived":  {path: "/organizations/inbox/archived"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			router := newValidatedRouter(t, http.MethodPost, tc.path)
			w := postJSON(t, router, clientPath(tc.path), []map[string]any{})
			assert.Equal(t, http.StatusBadRequest, w.Code, "empty array should be rejected")
		})
	}
}

func TestInboxSpecValidation_ValidRequest(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		path string
		body any
	}{
		"user inbox messages": {
			path: "/users/inbox",
			body: []map[string]any{validUserInboxMessage()},
		},
		"user inbox read": {
			path: "/users/inbox/read",
			body: []map[string]any{validUserInboxEvent()},
		},
		"user inbox archived": {
			path: "/users/inbox/archived",
			body: []map[string]any{validUserInboxEvent()},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			router := newValidatedRouter(t, http.MethodPost, tc.path)
			w := postJSON(t, router, clientPath(tc.path), tc.body)
			assert.Equal(t, http.StatusOK, w.Code, "valid request should pass spec validation")
		})
	}
}

func TestInboxSpecValidation_ValidCreateWithSchedulingMetadata(t *testing.T) {
	t.Parallel()

	msg := validUserInboxMessage()
	msg["identifier"] = map[string]any{"external_id": "message_123"}
	msg["scheduled_at"] = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)

	router := newValidatedRouter(t, http.MethodPost, "/users/inbox")
	w := postJSON(t, router, clientPath("/users/inbox"), []map[string]any{msg})
	assert.Equal(t, http.StatusOK, w.Code, "valid scheduling metadata should pass")
}

func TestInboxSpecValidation_ValidPriority(t *testing.T) {
	t.Parallel()

	for _, priority := range []int{1, 3, 5} {
		t.Run("priority_"+string(rune('0'+priority)), func(t *testing.T) {
			t.Parallel()
			msg := validUserInboxMessage()
			msg["priority"] = priority

			router := newValidatedRouter(t, http.MethodPost, "/users/inbox")
			w := postJSON(t, router, clientPath("/users/inbox"), []map[string]any{msg})
			assert.Equal(t, http.StatusOK, w.Code, "valid priority should pass")
		})
	}
}

// withOverrides returns a shallow copy of m with the given key set to value.
func withOverrides(m map[string]any, key string, value any) map[string]any {
	clone := make(map[string]any, len(m))
	for k, v := range m {
		clone[k] = v
	}
	clone[key] = value
	return clone
}
