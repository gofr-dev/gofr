package gofr

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

// TestMetricsMux_DetailedHealth covers GET /health on the metrics server: the detailed
// map counterpart to the redacted public /.well-known/health endpoint (see #3806).
func TestMetricsMux_DetailedHealth(t *testing.T) {
	testutil.NewServerConfigs(t)

	app := New()

	srv := httptest.NewServer(newMetricsMux(app.container))
	defer srv.Close()

	const healthPath = "/health"

	t.Run("GET serves the full detail map", func(t *testing.T) {
		resp, err := srv.Client().Get(srv.URL + healthPath)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

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
		resp, err := srv.Client().Post(srv.URL+healthPath, "application/json", http.NoBody)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
	})

	t.Run("unknown paths still reach the underlying metrics router", func(t *testing.T) {
		// The "/" catch-all mounts metrics.GetHandler. An unknown path must answer
		// with that router's 404 rather than anything from the /health route — without
		// scraping /metrics itself, whose status depends on process-global Prometheus
		// registry state shared with every other test in this package.
		resp, err := srv.Client().Get(srv.URL + "/no-such-route")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
