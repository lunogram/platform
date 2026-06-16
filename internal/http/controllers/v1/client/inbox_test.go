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

// newValidatedRouter creates a chi router with only the OpenAPI spec validation
// middleware. Auth is skipped (NoopAuthenticationFunc) so we can test request
// body validation in isolation. A 200 OK passthrough handler is mounted at the
// given path+method so that any request that passes validation returns 200.
func newValidatedRouter(t *testing.T, method, path string) chi.Router {
	t.Helper()

	spec, err := oapi.Spec()
	require.NoError(t, err)

	router := chi.NewRouter()
	router.Use(oapi.Validator(spec, openapi3filter.Options{
		AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
	}))
	router.MethodFunc(method, path, func(w http.ResponseWriter, r *http.Request) {
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
			path: "/api/client/users/inbox",
			body: []map[string]any{
				withOverrides(validUserInboxMessage(), "target", []map[string]any{}),
			},
		},
		"user inbox events": {
			path: "/api/client/users/inbox/read",
			body: []map[string]any{
				withOverrides(validUserInboxEvent(), "target", []map[string]any{}),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			router := newValidatedRouter(t, http.MethodPost, tc.path)
			w := postJSON(t, router, tc.path, tc.body)
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
			path: "/api/client/users/inbox",
			body: []map[string]any{
				withOverrides(validUserInboxMessage(), "target", []map[string]any{{"external_id": ""}}),
			},
		},
		"user inbox events": {
			path: "/api/client/users/inbox/archived",
			body: []map[string]any{
				withOverrides(validUserInboxEvent(), "target", []map[string]any{{"external_id": ""}}),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			router := newValidatedRouter(t, http.MethodPost, tc.path)
			w := postJSON(t, router, tc.path, tc.body)
			assert.Equal(t, http.StatusBadRequest, w.Code, "empty external_id should be rejected")
		})
	}
}

func TestInboxSpecValidation_InvalidChannel(t *testing.T) {
	t.Parallel()

	router := newValidatedRouter(t, http.MethodPost, "/api/client/users/inbox")
	w := postJSON(t, router, "/api/client/users/inbox", []map[string]any{
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

			router := newValidatedRouter(t, http.MethodPost, "/api/client/users/inbox")
			w := postJSON(t, router, "/api/client/users/inbox", []map[string]any{msg})
			assert.Equal(t, http.StatusBadRequest, w.Code, "invalid priority should be rejected")
		})
	}
}

func TestInboxSpecValidation_EmptyArray(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		path string
	}{
		"user inbox messages": {path: "/api/client/users/inbox"},
		"user inbox read":     {path: "/api/client/users/inbox/read"},
		"user inbox archived": {path: "/api/client/users/inbox/archived"},
		"org inbox read":      {path: "/api/client/organizations/inbox/read"},
		"org inbox archived":  {path: "/api/client/organizations/inbox/archived"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			router := newValidatedRouter(t, http.MethodPost, tc.path)
			w := postJSON(t, router, tc.path, []map[string]any{})
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
			path: "/api/client/users/inbox",
			body: []map[string]any{validUserInboxMessage()},
		},
		"user inbox read": {
			path: "/api/client/users/inbox/read",
			body: []map[string]any{validUserInboxEvent()},
		},
		"user inbox archived": {
			path: "/api/client/users/inbox/archived",
			body: []map[string]any{validUserInboxEvent()},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			router := newValidatedRouter(t, http.MethodPost, tc.path)
			w := postJSON(t, router, tc.path, tc.body)
			assert.Equal(t, http.StatusOK, w.Code, "valid request should pass spec validation")
		})
	}
}

func TestInboxSpecValidation_ValidCreateWithSchedulingMetadata(t *testing.T) {
	t.Parallel()

	msg := validUserInboxMessage()
	msg["identifier"] = map[string]any{"external_id": "message_123"}
	msg["scheduled_at"] = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)

	router := newValidatedRouter(t, http.MethodPost, "/api/client/users/inbox")
	w := postJSON(t, router, "/api/client/users/inbox", []map[string]any{msg})
	assert.Equal(t, http.StatusOK, w.Code, "valid scheduling metadata should pass")
}

func TestInboxSpecValidation_ValidPriority(t *testing.T) {
	t.Parallel()

	for _, priority := range []int{1, 3, 5} {
		t.Run("priority_"+string(rune('0'+priority)), func(t *testing.T) {
			t.Parallel()
			msg := validUserInboxMessage()
			msg["priority"] = priority

			router := newValidatedRouter(t, http.MethodPost, "/api/client/users/inbox")
			w := postJSON(t, router, "/api/client/users/inbox", []map[string]any{msg})
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
