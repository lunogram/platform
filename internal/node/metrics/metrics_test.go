package metrics

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	ObserveDatasetQuery(QueryListRecompute, project, time.Now().Add(-2*time.Second), 1500, nil)

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

	ObserveDatasetQuery(QueryListPreview, project, time.Now(), 0, assert.AnError)

	body := scrape(t)
	labels := `project_id="` + project.String() + `",query="` + QueryListPreview + `"`

	assert.Contains(t, body, `lunogram_dataset_query_errors_total{`+labels+`} 1`)
	assert.Contains(t, body, `lunogram_dataset_queries_total{`+labels+`} 1`)

	// A failed query has no meaningful row count, so it must not be recorded
	// as having returned zero rows -- that would drag the distribution down
	// and make an outage look like a quiet period.
	assert.NotContains(t, body, `lunogram_dataset_query_rows_count{`+labels+`}`)
}

func TestServeExposesMetricsEndpoint(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", NewHandler())

	server := httptest.NewServer(mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/metrics") //nolint:noctx
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	// The registry has to actually carry Lunogram's series, not just the Go
	// runtime collectors promauto registers by default.
	assert.True(t, strings.Contains(string(body), "lunogram_"),
		"expected lunogram metrics in the scrape output")
}
