package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/lunogram/platform/internal/ratelimit"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// newRedisLimiter builds a Limiter backed by a real Redis container with a
// unique prefix so the budget is isolated per test.
func newRedisLimiter(t *testing.T) *ratelimit.Limiter {
	t.Helper()

	connstr := container.RunRedis(t)
	opts, err := redis.ParseURL(connstr)
	require.NoError(t, err)

	client := redis.NewClient(opts)
	t.Cleanup(func() { client.Close() })

	prefix := fmt.Sprintf("mwtest:%s:", uuid.New())
	return ratelimit.New(client, prefix, zaptest.NewLogger(t))
}

// newBrokenLimiter builds a Limiter whose Redis backend is unreachable, so
// every Allow call hits a transport error. The middleware must fail open.
func newBrokenLimiter(t *testing.T) *ratelimit.Limiter {
	t.Helper()

	// Port 1 on the loopback refuses connections; the short dial timeout keeps
	// the test fast.
	client := redis.NewClient(&redis.Options{
		Addr:        "127.0.0.1:1",
		DialTimeout: 200 * time.Millisecond,
		MaxRetries:  -1,
	})
	t.Cleanup(func() { client.Close() })

	return ratelimit.New(client, "broken:", zaptest.NewLogger(t))
}

// okHandler records whether the wrapped handler was reached and returns 200.
func okHandler(reached *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestRateLimitDeniesOverLimit(t *testing.T) {
	t.Parallel()

	limiter := newRedisLimiter(t)

	limit := 2
	window := time.Minute
	mw := RateLimit(limiter, limit, window, 0, oapi.WriteProblem)

	var reached int
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached++
		w.WriteHeader(http.StatusOK)
	}))

	newReq := func() *http.Request {
		// A stable RemoteAddr keys all requests to one ip: bucket.
		r := httptest.NewRequest(http.MethodGet, "/api/client/v1/inbox", nil)
		r.RemoteAddr = "198.51.100.10:4444"
		return r
	}

	// The first `limit` requests pass through to the handler.
	for i := range limit {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, newReq())
		require.Equal(t, http.StatusOK, rec.Code, "request %d should be allowed", i+1)
	}
	require.Equal(t, limit, reached)

	// The next request is over budget: 429 with a Retry-After header, handler
	// never reached.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, newReq())

	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Equal(t, limit, reached, "handler must not run on a denied request")

	retryAfter := rec.Header().Get("Retry-After")
	require.NotEmpty(t, retryAfter, "Retry-After header must be set on 429")
	secs, err := strconv.Atoi(retryAfter)
	require.NoError(t, err, "Retry-After must be an integer number of seconds")
	require.Greater(t, secs, 0)
	require.LessOrEqual(t, secs, int(window.Seconds()))
}

func TestRateLimitFailsOpenOnBackendError(t *testing.T) {
	t.Parallel()

	limiter := newBrokenLimiter(t)

	// A limit of zero would deny everything if the backend were consulted; the
	// fail-open path must still let the request through when Redis errors.
	mw := RateLimit(limiter, 0, time.Minute, 0, oapi.WriteProblem)

	var reached bool
	handler := mw(okHandler(&reached))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/client/v1/inbox", nil)
	req.RemoteAddr = "198.51.100.20:5555"
	handler.ServeHTTP(rec, req)

	require.True(t, reached, "request must pass through when the limiter backend errors")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get("Retry-After"))
}

func TestRateLimitKey(t *testing.T) {
	t.Parallel()

	t.Run("authenticated actor keys on key:<id> regardless of surface", func(t *testing.T) {
		t.Parallel()

		actor := rbac.NewActor(rbac.ActorAPIKey, "ak_123")

		client := httptest.NewRequest(http.MethodGet, "/api/client/v1/inbox", nil)
		client.RemoteAddr = "203.0.113.1:1111"
		client = client.WithContext(rbac.WithActor(client.Context(), actor))

		mgmt := httptest.NewRequest(http.MethodGet, "/api/management/v1/users", nil)
		mgmt.RemoteAddr = "203.0.113.2:2222"
		mgmt = mgmt.WithContext(rbac.WithActor(mgmt.Context(), actor))

		require.Equal(t, "key:ak_123", rateLimitKey(client, 0))
		// Same key across a different surface and different IP: one shared budget.
		require.Equal(t, rateLimitKey(client, 0), rateLimitKey(mgmt, 0))
	})

	t.Run("unauthenticated falls back to ip:", func(t *testing.T) {
		t.Parallel()

		r := httptest.NewRequest(http.MethodGet, "/api/client/v1/inbox", nil)
		r.RemoteAddr = "203.0.113.7:9999"

		require.Equal(t, "ip:203.0.113.7", rateLimitKey(r, 0))
	})

	t.Run("empty actor id falls back to ip:", func(t *testing.T) {
		t.Parallel()

		actor := &rbac.Actor{Type: rbac.ActorAPIKey, ID: ""}
		r := httptest.NewRequest(http.MethodGet, "/api/client/v1/inbox", nil)
		r.RemoteAddr = "203.0.113.8:8888"
		r = r.WithContext(rbac.WithActor(r.Context(), actor))

		require.Equal(t, "ip:203.0.113.8", rateLimitKey(r, 0))
	})

	t.Run("ip key honours trusted proxy hops", func(t *testing.T) {
		t.Parallel()

		r := httptest.NewRequest(http.MethodGet, "/api/client/v1/inbox", nil)
		r.RemoteAddr = "10.0.0.1:7777"
		r.Header.Set("X-Forwarded-For", "9.9.9.9, 1.2.3.4")

		require.Equal(t, "ip:1.2.3.4", rateLimitKey(r, 1))
	})
}

func TestClientIP(t *testing.T) {
	t.Parallel()

	req := func(xff string) *http.Request {
		r := &http.Request{RemoteAddr: "203.0.113.7:5555", Header: http.Header{}}
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}

	tests := []struct {
		name string
		xff  string
		hops int
		want string
	}{
		{"no trusted proxy ignores spoofed XFF", "1.2.3.4", 0, "203.0.113.7"},
		{"single trusted proxy takes client hop", "9.9.9.9, 1.2.3.4", 1, "1.2.3.4"},
		{"one proxy, single XFF entry is the client", "1.2.3.4", 1, "1.2.3.4"},
		{"two trusted proxies", "5.5.5.5, 9.9.9.9, 1.2.3.4", 2, "9.9.9.9"},
		{"more hops than chain clamps to left-most", "1.2.3.4", 5, "1.2.3.4"},
		{"no XFF falls back to remote", "", 1, "203.0.113.7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clientIP(req(tt.xff), tt.hops); got != tt.want {
				t.Errorf("clientIP(hops=%d, xff=%q) = %q, want %q", tt.hops, tt.xff, got, tt.want)
			}
		})
	}
}
