package outbound

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient(t *testing.T, opts Options) *Client {
	t.Helper()
	if opts.Timeout == 0 {
		opts.Timeout = 2 * time.Second
	}
	opts.Network = Network{AllowPrivate: true, AllowHTTP: true}
	if opts.Retry.MaxAttempts == 0 {
		opts.Retry = Retry{MaxAttempts: 3, InitialInterval: time.Millisecond, MaxInterval: 5 * time.Millisecond}
	}
	client, err := NewClient(opts)
	require.NoError(t, err)
	return client
}

func TestRetryable(t *testing.T) {
	t.Parallel()

	for _, status := range []int{500, 502, 503, 504, http.StatusRequestTimeout, http.StatusTooManyRequests} {
		assert.True(t, Retryable(status), "status %d should be retryable", status)
	}
	for _, status := range []int{400, 401, 403, 404, 409, 422} {
		assert.False(t, Retryable(status), "status %d should not be retryable", status)
	}
}

func TestDoRetriesServerErrors(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if attempts.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := testClient(t, Options{})
	resp, err := client.Do(t.Context(), Request{Method: http.MethodPost, URL: server.URL})
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.EqualValues(t, 3, attempts.Load())
}

func TestDoDoesNotRetryClientErrors(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusUnprocessableEntity} {
		var attempts atomic.Int64
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts.Add(1)
			w.WriteHeader(status)
		}))

		client := testClient(t, Options{})
		_, err := client.Do(t.Context(), Request{Method: http.MethodPost, URL: server.URL})
		require.Error(t, err)

		statusErr, ok := AsStatusError(err)
		require.True(t, ok)
		assert.Equal(t, status, statusErr.StatusCode)
		assert.EqualValues(t, 1, attempts.Load(), "status %d must not be retried", status)

		server.Close()
	}
}

func TestDoRetriesTimeoutAndTooManyRequests(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests} {
		var attempts atomic.Int64
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			attempts.Add(1)
			w.WriteHeader(status)
		}))

		client := testClient(t, Options{})
		_, err := client.Do(t.Context(), Request{Method: http.MethodPost, URL: server.URL})
		require.Error(t, err)
		assert.EqualValues(t, 3, attempts.Load(), "status %d should exhaust the attempt budget", status)

		server.Close()
	}
}

func TestDoStopsAtMaxAttempts(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	client := testClient(t, Options{Retry: Retry{MaxAttempts: 2, InitialInterval: time.Millisecond, MaxInterval: time.Millisecond}})
	_, err := client.Do(t.Context(), Request{Method: http.MethodPost, URL: server.URL})
	require.Error(t, err)
	assert.EqualValues(t, 2, attempts.Load())
}

func TestDoHonoursContextCancellation(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := testClient(t, Options{Retry: Retry{MaxAttempts: 100, InitialInterval: 20 * time.Millisecond, MaxInterval: 20 * time.Millisecond}})

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.Do(ctx, Request{Method: http.MethodPost, URL: server.URL})
	require.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second, "retries must stop when the context expires")
	assert.Less(t, attempts.Load(), int64(100))
}

func TestDoRespectsMaxElapsedTime(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := testClient(t, Options{Retry: Retry{
		MaxAttempts:     1000,
		InitialInterval: 10 * time.Millisecond,
		MaxInterval:     10 * time.Millisecond,
		MaxElapsedTime:  80 * time.Millisecond,
	}})

	start := time.Now()
	_, err := client.Do(t.Context(), Request{Method: http.MethodPost, URL: server.URL})
	require.Error(t, err)
	assert.Less(t, time.Since(start), time.Second)
}

func TestDoBoundsResponseBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", 4096)))
	}))
	defer server.Close()

	client := testClient(t, Options{MaxResponseBytes: 128})
	resp, err := client.Do(t.Context(), Request{Method: http.MethodGet, URL: server.URL})
	require.NoError(t, err)
	assert.Len(t, resp.Body, 128)
}

func TestDoSendsBodyQueryAndHeaders(t *testing.T) {
	t.Parallel()

	var body, query, header string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(raw)
		body = string(raw)
		query = r.URL.RawQuery
		header = r.Header.Get("X-Webhook-Event")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := testClient(t, Options{})
	_, err := client.Do(t.Context(), Request{
		Method: http.MethodPost,
		URL:    server.URL,
		Body:   []byte(`{"a":1}`),
		Query:  map[string][]string{"limit": {"10"}},
		Header: http.Header{"X-Webhook-Event": []string{"project.created"}},
	})
	require.NoError(t, err)
	assert.Equal(t, `{"a":1}`, body)
	assert.Equal(t, "limit=10", query)
	assert.Equal(t, "project.created", header)
}

func TestStatusErrorDoesNotInterpolateBody(t *testing.T) {
	t.Parallel()

	err := &StatusError{StatusCode: 500, Body: []byte("internal detail: db password=hunter2")}
	assert.NotContains(t, err.Error(), "hunter2")
	assert.Contains(t, err.Error(), "500")
}

func TestNewClientRejectsBadRetryConfig(t *testing.T) {
	t.Parallel()

	_, err := NewClient(Options{Retry: Retry{MaxAttempts: -1}})
	require.Error(t, err)

	_, err = NewClient(Options{Retry: Retry{MaxAttempts: 2, InitialInterval: time.Minute, MaxInterval: time.Second}})
	require.Error(t, err)
}

func TestIgnoreStatusSkipsStatusRetries(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := testClient(t, Options{Retry: Retry{MaxAttempts: 5, InitialInterval: time.Millisecond, MaxInterval: time.Millisecond}})
	resp, err := client.Do(t.Context(), Request{Method: http.MethodPost, URL: server.URL, IgnoreStatus: true})
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	assert.EqualValues(t, 1, attempts.Load(), "a status nobody reads must not be retried")
}

func TestIgnoreStatusStillRetriesTransportFailures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	url := server.URL
	server.Close() // nothing is listening, so every attempt fails to connect

	client := testClient(t, Options{Retry: Retry{MaxAttempts: 3, InitialInterval: time.Millisecond, MaxInterval: time.Millisecond}})
	_, err := client.Do(t.Context(), Request{Method: http.MethodPost, URL: url, IgnoreStatus: true})
	require.Error(t, err)
}
