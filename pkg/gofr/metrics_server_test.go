package gofr

import (
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/datasource"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/testutil"
)

const metricsHealthPath = "/health"

var errKVStoreUnreachable = errors.New("dial tcp kv.internal:6379: connection refused")

// metricsMuxDo sends one request to the metrics mux under test and returns what came back, so no
// caller has a response body left to close.
func metricsMuxDo(t *testing.T, srv *httptest.Server, method, path string) (status int, header http.Header, body []byte) {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, srv.URL+path, http.NoBody)
	require.NoError(t, err)

	resp, err := srv.Client().Do(req)
	require.NoError(t, err)

	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, resp.Header, body
}

// TestMetricsMux_DetailedHealth covers GET /health on the metrics server: the detailed
// map counterpart to the redacted public /.well-known/health endpoint (see #3806).
func TestMetricsMux_DetailedHealth(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	srv := httptest.NewServer(newMetricsMux(app.container))
	defer srv.Close()

	t.Run("GET serves the full detail map", func(t *testing.T) {
		status, header, body := metricsMuxDo(t, srv, http.MethodGet, metricsHealthPath)

		assert.Equal(t, http.StatusOK, status)
		assert.Contains(t, header.Get("Content-Type"), "application/json")

		var got map[string]any
		require.NoError(t, json.Unmarshal(body, &got))

		// Pin the aggregate keys every consumer relies on; per-backend keys join them
		// whenever datasources are configured. No datasources are configured here, so
		// the aggregate is UP.
		assert.Equal(t, "UP", got["status"])
		assert.Contains(t, got, "name")
		assert.Contains(t, got, "version")
	})

	t.Run("non-GET is refused", func(t *testing.T) {
		status, header, _ := metricsMuxDo(t, srv, http.MethodPost, metricsHealthPath)

		assert.Equal(t, http.StatusMethodNotAllowed, status)
		assert.Equal(t, http.MethodGet, header.Get("Allow"))
	})

	t.Run("unknown paths still reach the underlying metrics router", func(t *testing.T) {
		// The "/" catch-all mounts metrics.GetHandler. An unknown path must answer
		// with that router's 404 rather than anything from the /health route — without
		// scraping /metrics itself, whose status depends on process-global Prometheus
		// registry state shared with every other test in this package.
		status, _, _ := metricsMuxDo(t, srv, http.MethodGet, "/no-such-route")

		assert.Equal(t, http.StatusNotFound, status)
	})
}

// TestMetricsMux_DetailedHealth_Datasource is the point of #3806: a configured datasource's
// details reach the metrics port and still do not reach the public endpoint.
func TestMetricsMux_DetailedHealth_Datasource(t *testing.T) {
	const kvHost = "kv.internal:6379"

	tests := []struct {
		desc       string
		health     datasource.Health
		err        error
		wantStatus string
	}{
		{
			desc:       "healthy datasource",
			health:     datasource.Health{Status: datasource.StatusUp, Details: map[string]any{"host": kvHost}},
			wantStatus: datasource.StatusUp,
		},
		{
			desc:       "datasource down still answers 200 with the detail",
			health:     datasource.Health{Status: datasource.StatusDown, Details: map[string]any{"host": kvHost}},
			err:        errKVStoreUnreachable,
			wantStatus: datasource.StatusDegraded,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			testutil.NewServerConfigs(t)

			app := New()

			kv := container.NewMockKVStore(gomock.NewController(t))
			kv.EXPECT().HealthCheck(gomock.Any()).Return(tc.health, tc.err).AnyTimes()
			app.AddKVStore(kv)

			srv := httptest.NewServer(newMetricsMux(app.container))
			defer srv.Close()

			status, _, body := metricsMuxDo(t, srv, http.MethodGet, metricsHealthPath)

			assert.Equal(t, http.StatusOK, status)

			var detailed struct {
				Status  string            `json:"status"`
				KVStore datasource.Health `json:"kv-store"`
			}

			require.NoError(t, json.Unmarshal(body, &detailed))

			assert.Equal(t, tc.wantStatus, detailed.Status)
			assert.Equal(t, tc.health.Status, detailed.KVStore.Status)
			assert.Equal(t, kvHost, detailed.KVStore.Details["host"])

			// The same container behind the public handler: aggregate only, host nowhere in it.
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "", http.NoBody)
			require.NoError(t, err)

			public, err := app.healthHandler(newContext(nil, gofrHTTP.NewRequest(req), app.container))
			require.NoError(t, err)

			publicBody, err := json.Marshal(public)
			require.NoError(t, err)

			var redacted map[string]any
			require.NoError(t, json.Unmarshal(publicBody, &redacted))

			assert.Equal(t, []string{"name", "status"}, slices.Sorted(maps.Keys(redacted)))
			assert.Equal(t, tc.wantStatus, redacted["status"])
			assert.NotContains(t, string(publicBody), kvHost)
		})
	}
}
