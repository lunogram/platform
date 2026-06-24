package oapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCORSWildcardWithoutCredentials verifies the client API CORS policy: a
// wildcard origin is reflected and credentials are never allowed. The client
// API is bearer-only (no cookies), so a wildcard is safe precisely because
// credentials are disabled.
func TestCORSWildcardWithoutCredentials(t *testing.T) {
	t.Parallel()

	var reached bool
	handler := CORS()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("simple request reflects wildcard, no credentials", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/client/v1/inbox", nil)
		req.Header.Set("Origin", "https://app.example.com")
		handler.ServeHTTP(rec, req)

		require.True(t, reached)
		require.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
		require.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"),
			"credentials must never be allowed on the wildcard client API")
	})

	t.Run("preflight allows the origin without credentials", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodOptions, "/api/client/v1/inbox", nil)
		req.Header.Set("Origin", "https://anywhere.example.org")
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		handler.ServeHTTP(rec, req)

		require.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
		require.Empty(t, rec.Header().Get("Access-Control-Allow-Credentials"))
	})
}
