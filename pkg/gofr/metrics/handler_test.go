package metrics

import (
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/metrics/exporters"
	"gofr.dev/pkg/gofr/testutil"
	"io"
	"net/http/httptest"
	"testing"
)

func Test_MetricsGetHandler_MetricsNotRegistered(t *testing.T) {
	var server *httptest.Server

	getLogs := func() {
		manager := NewMetricsManager(exporters.Prometheus("test-app", "v1.0.0"),
			testutil.NewMockLogger(testutil.INFOLOG))

		handler := GetHandler(manager)

		server = httptest.NewServer(handler)

		server.Client().Get(server.URL + "/metrics")
	}

	assert.Contains(t, "Metrics app_go_routines is not registered\nMetrics app_sys_memory_alloc is not registered\nMetrics app_sys_total_alloc is not registered\nMetrics app_go_numGC is not registered\nMetrics app_go_sys is not registered\n",
		testutil.StderrOutputForFunc(getLogs))
}

func Test_MetricsGetHandler_SystemMetricsRegistered(t *testing.T) {
	manager := NewMetricsManager(exporters.Prometheus("test-app", "v1.0.0"),
		testutil.NewMockLogger(testutil.INFOLOG))

	manager.NewGauge("app_go_routines", "Number of Go routines running.")
	manager.NewGauge("app_sys_memory_alloc", "Number of bytes allocated for heap objects.")
	manager.NewGauge("app_sys_total_alloc", "Number of cumulative bytes allocated for heap objects.")
	manager.NewGauge("app_go_numGC", "Number of completed Garbage Collector cycles.")
	manager.NewGauge("app_go_sys", "Number of total bytes of memory.")

	handler := GetHandler(manager)

	server := httptest.NewServer(handler)

	resp, err := server.Client().Get(server.URL + "/metrics")

	assert.Nil(t, err)

	body, err := io.ReadAll(resp.Body)

	bodyString := string(body)

	assert.Contains(t, bodyString, `app_go_sys{otel_scope_name="test-app",otel_scope_version="v1.0.0"}`)
	assert.Contains(t, bodyString, `app_sys_memory_alloc{otel_scope_name="test-app",otel_scope_version="v1.0.0"}`)
	assert.Contains(t, bodyString, `app_sys_total_alloc{otel_scope_name="test-app",otel_scope_version="v1.0.0"}`)
	assert.Contains(t, bodyString, `app_sys_total_alloc{otel_scope_name="test-app",otel_scope_version="v1.0.0"}`)
	assert.Contains(t, bodyString, `app_go_numGC{otel_scope_name="test-app",otel_scope_version="v1.0.0"}`)
}
