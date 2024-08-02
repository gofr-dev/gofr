package metrics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/metrics/exporters"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_MetricsGetHandler_MetricsNotRegistered(t *testing.T) {
	var server *httptest.Server

	logs := func() {
		manager := NewMetricsManager(exporters.Prometheus("test-app", "v1.0.0"),
			logging.NewMockLogger(logging.INFO))

		handler := GetHandler(manager)

		server = httptest.NewServer(handler)

		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/metrics", http.NoBody)

		resp, _ := server.Client().Do(req)
		if resp != nil {
			defer resp.Body.Close()
		}
	}

	assert.Contains(t, testutil.StderrOutputForFunc(logs), "Metrics app_go_routines is not registered\n"+
		"Metrics app_sys_memory_alloc is not registered\n"+"Metrics app_sys_total_alloc is not registered\n"+
		"Metrics app_go_numGC is not registered\n"+"Metrics app_go_sys is not registered\n")
}

func Test_MetricsGetHandler_SystemMetricsRegistered(t *testing.T) {
	manager := NewMetricsManager(exporters.Prometheus("test-app", "v1.0.0"),
		logging.NewMockLogger(logging.INFO))

	// Registering the metrics because the values are being set in the GetHandler function.
	manager.NewGauge("app_go_routines", "Number of Go routines running.")
	manager.NewGauge("app_sys_memory_alloc", "Number of bytes allocated for heap objects.")
	manager.NewGauge("app_sys_total_alloc", "Number of cumulative bytes allocated for heap objects.")
	manager.NewGauge("app_go_numGC", "Number of completed Garbage Collector cycles.")
	manager.NewGauge("app_go_sys", "Number of total bytes of memory.")

	handler := GetHandler(manager)

	server := httptest.NewServer(handler)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server.URL+"/metrics", http.NoBody)

	resp, err := server.Client().Do(req)

	require.NoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	bodyString := string(body)

	assert.Contains(t, bodyString, `app_go_sys{otel_scope_name="test-app",otel_scope_version="v1.0.0"}`)
	assert.Contains(t, bodyString, `app_sys_memory_alloc{otel_scope_name="test-app",otel_scope_version="v1.0.0"}`)
	assert.Contains(t, bodyString, `app_sys_total_alloc{otel_scope_name="test-app",otel_scope_version="v1.0.0"}`)
	assert.Contains(t, bodyString, `app_sys_total_alloc{otel_scope_name="test-app",otel_scope_version="v1.0.0"}`)
	assert.Contains(t, bodyString, `app_go_numGC{otel_scope_name="test-app",otel_scope_version="v1.0.0"}`)
}
