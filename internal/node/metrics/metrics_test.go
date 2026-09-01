package metrics

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func scrape(t *testing.T) string {
	t.Helper()

	server := httptest.NewServer(NewHandler())
	defer server.Close()

	resp, err := http.Get(server.URL) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(body)
}

func TestObserveDatasetQueryRecordsRowsAndDuration(t *testing.T) {
	project := uuid.New()

	ObserveDatasetQuery(QueryListRecompute, project, 2*time.Second, 1500, nil)

	body := scrape(t)
	// The exposition format sorts labels alphabetically, so project_id leads.
	labels := `project_id="` + project.String() + `",query="` + QueryListRecompute + `"`

	assert.Contains(t, body, `lunogram_dataset_queries_total{`+labels+`} 1`)
	assert.Contains(t, body, `lunogram_dataset_query_rows_total{`+labels+`} 1500`)
	assert.Contains(t, body, `lunogram_dataset_query_duration_seconds_count{`+labels+`} 1`)

	// 1500 rows lands in the 10000 bucket but not the 1000 one, and the two
	// second query lands above the 1s boundary.
	assert.Contains(t, body, `lunogram_dataset_query_rows_bucket{`+labels+`,le="1000"} 0`)
	assert.Contains(t, body, `lunogram_dataset_query_rows_bucket{`+labels+`,le="10000"} 1`)
	assert.Contains(t, body, `lunogram_dataset_query_duration_seconds_bucket{`+labels+`,le="1"} 0`)
	assert.Contains(t, body, `lunogram_dataset_query_duration_seconds_bucket{`+labels+`,le="2.5"} 1`)
}

func TestObserveDatasetQueryFailureRecordsNoRows(t *testing.T) {
	project := uuid.New()

	ObserveDatasetQuery(QueryListPreview, project, time.Millisecond, 0, assert.AnError)

	body := scrape(t)
	labels := `project_id="` + project.String() + `",query="` + QueryListPreview + `"`

	assert.Contains(t, body, `lunogram_dataset_query_errors_total{`+labels+`} 1`)
	assert.Contains(t, body, `lunogram_dataset_queries_total{`+labels+`} 1`)

	// A failed query has no meaningful row count, so it must not be recorded
	// as having returned zero rows -- that would drag the distribution down
	// and make an outage look like a quiet period.
	assert.NotContains(t, body, `lunogram_dataset_query_rows_count{`+labels+`}`)
}

// serveOnEphemeralPort starts the real server on a port the kernel picks and
// returns its base URL. The listener is open before serve runs, so a request
// made straight after this returns is accepted rather than refused.
func serveOnEphemeralPort(t *testing.T) (string, graceful.Context, <-chan struct{}) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	ctx := graceful.NewContext(context.Background())
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)
		serve(ctx, zap.NewNop(), listener)
	}()

	return "http://" + listener.Addr().String(), ctx, stopped
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()

	resp, err := http.Get(url) //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, string(body)
}

func TestServeExposesMetricsEndpoint(t *testing.T) {
	url, ctx, stopped := serveOnEphemeralPort(t)
	defer ctx.Shutdown()

	status, body := get(t, url+"/metrics")
	assert.Equal(t, http.StatusOK, status)

	// The registry has to actually carry Lunogram's series, not just the Go
	// runtime collectors promauto registers by default.
	assert.Contains(t, body, "lunogram_", "expected lunogram metrics in the scrape output")

	status, _ = get(t, url+"/health")
	assert.Equal(t, http.StatusOK, status)

	// Nothing else is mounted: the registry is the whole reason this listener
	// exists, and a catch-all here is how it would stop being.
	status, _ = get(t, url+"/")
	assert.Equal(t, http.StatusNotFound, status)

	select {
	case <-stopped:
		t.Fatal("server stopped while it should still be serving")
	default:
	}
}

func TestServeShutsDownWithTheContext(t *testing.T) {
	url, ctx, stopped := serveOnEphemeralPort(t)

	status, _ := get(t, url+"/metrics")
	require.Equal(t, http.StatusOK, status)

	ctx.Shutdown()

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("metrics server did not shut down with its context")
	}

	// The port is genuinely released rather than left held by a listener whose
	// server has gone away.
	_, err := http.Get(url + "/metrics") //nolint:noctx
	assert.Error(t, err)
}
