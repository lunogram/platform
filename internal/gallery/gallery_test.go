package gallery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lunogram/platform/internal/outbound"
	"github.com/lunogram/platform/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func testGallery(t *testing.T, url string, mutate func(*webhook.GalleryConfig)) *Client {
	t.Helper()
	cfg := &webhook.GalleryConfig{
		URL:     url,
		Timeout: 2 * time.Second,
		Network: outbound.Network{AllowPrivate: true, AllowHTTP: true},
		Retry:   &outbound.Retry{MaxAttempts: 2, InitialInterval: time.Millisecond, MaxInterval: time.Millisecond},
	}
	if mutate != nil {
		mutate(cfg)
	}
	client, err := New(zaptest.NewLogger(t), cfg)
	require.NoError(t, err)
	return client
}

func TestListDecodesAndForwardsOnlyKnownParams(t *testing.T) {
	t.Parallel()

	var query string
	var authorization, cookie string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		authorization = r.Header.Get("Authorization")
		cookie = r.Header.Get("Cookie")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total":1,"limit":10,"offset":0,"results":[{"id":"welcome","label":"Welcome","extra":"ignored"}]}`))
	}))
	defer server.Close()

	limit, offset := 10, 0
	search := "wel"

	out, err := testGallery(t, server.URL, nil).List(t.Context(), Query{Limit: &limit, Offset: &offset, Search: &search})
	require.NoError(t, err)

	assert.Equal(t, 1, out.Total)
	require.Len(t, out.Results, 1)
	assert.Equal(t, "welcome", out.Results[0].ID)
	assert.Equal(t, "Welcome", out.Results[0].Label)

	assert.Contains(t, query, "limit=10")
	assert.Contains(t, query, "search=wel")
	assert.Empty(t, authorization, "the caller's credentials are never forwarded to the gallery")
	assert.Empty(t, cookie)
}

func TestListRejectsMalformedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>this is not the gallery</html>`))
	}))
	defer server.Close()

	_, err := testGallery(t, server.URL, nil).List(t.Context(), Query{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid gallery response")
}

func TestListBoundsTheResponseBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"results":[` + strings.Repeat(`{"id":"a","label":"b"},`, 10000) + `{"id":"z","label":"z"}]}`))
	}))
	defer server.Close()

	_, err := testGallery(t, server.URL, func(cfg *webhook.GalleryConfig) { cfg.MaxResponseBytes = 512 }).
		List(t.Context(), Query{})
	require.Error(t, err, "a truncated body must fail rather than be partially trusted")
}

func TestListAppliesAuth(t *testing.T) {
	t.Parallel()

	var seen string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("X-Gallery-Key")
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer server.Close()

	client := testGallery(t, server.URL, func(cfg *webhook.GalleryConfig) {
		parsed, err := webhook.ParseConfig([]byte(`version: v1
email_templates:
  url: `+server.URL+`
  auth: {type: api_key, config: {name: X-Gallery-Key, value: gallery-secret}}
`), "")
		require.NoError(t, err)
		cfg.Auth = parsed.EmailTemplates.Auth
	})

	_, err := client.List(t.Context(), Query{})
	require.NoError(t, err)
	assert.Equal(t, "gallery-secret", seen)
}

func TestNilAndUnconfiguredClient(t *testing.T) {
	t.Parallel()

	client, err := New(zaptest.NewLogger(t), nil)
	require.NoError(t, err)
	assert.False(t, client.Enabled())
	_, err = client.List(t.Context(), Query{})
	require.Error(t, err)

	client, err = New(zaptest.NewLogger(t), &webhook.GalleryConfig{})
	require.NoError(t, err)
	assert.False(t, client.Enabled())
}

func TestNewRejectsUnsafeURL(t *testing.T) {
	t.Parallel()

	_, err := New(zaptest.NewLogger(t), &webhook.GalleryConfig{URL: "http://169.254.169.254/latest/meta-data"})
	require.Error(t, err)
}
