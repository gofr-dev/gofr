package metrics

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/metrics/exporters"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_NewMetricsManagerSuccess(t *testing.T) {
	metrics := NewMetricsManager(exporters.Prometheus("testing-app", "v1.0.0"),
		logging.NewMockLogger(logging.INFO))

	metrics.NewGauge("gauge-test", "this is metric to test gauge")
	metrics.NewCounter("counter-test", "this is metric to test counter")
	metrics.NewUpDownCounter("up-down-counter", "this is metric to test up-down-counter")
	metrics.NewHistogram("histogram-test", "this is metric to test histogram")

	metrics.SetGauge("gauge-test", 50)
	metrics.IncrementCounter(t.Context(), "counter-test")
	metrics.DeltaUpDownCounter(t.Context(), "up-down-counter", 10)
	metrics.RecordHistogram(t.Context(), "histogram-test", 1)

	server := httptest.NewServer(GetHandler(metrics))

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/metrics", http.NoBody)
	resp, _ := server.Client().Do(req)
	body, _ := io.ReadAll(resp.Body)

	defer resp.Body.Close()

	stringBody := string(body)

	assert.Contains(t, stringBody, `otel_scope_name="testing-app",otel_scope_schema_url="",otel_scope_version="v1.0.0"`,
		"TEST Failed. service name and version not coming in metrics")

	assert.Contains(t, stringBody, `counter_test this is metric to test counter`,
		"TEST Failed. counter-test metrics registration failed")

	assert.Contains(t, stringBody, `counter_test{otel_scope_name="testing-app",otel_scope_schema_url="",otel_scope_version="v1.0.0"} 1`,
		"TEST Failed. counter-test metrics registration failed")

	assert.Contains(t, stringBody, `gauge_test this is metric to test gauge`,
		"TEST Failed. gauge-test metrics registration failed")

	assert.Contains(t, stringBody, `gauge_test{otel_scope_name="testing-app",otel_scope_schema_url="",otel_scope_version="v1.0.0"} 50`,
		"TEST Failed. gauge_test metrics value not set")

	assert.Contains(t, stringBody, `up_down_counter{otel_scope_name="testing-app",otel_scope_schema_url="",otel_scope_version="v1.0.0"} 10`,
		"TEST Failed. up-down-counter metrics value did not reflect")

	assert.Contains(t, stringBody, `up_down_counter this is metric to test up-down-counter`,
		"TEST Failed. up-down-counter metrics registration failed")

	assert.Contains(t, stringBody, `histogram_test this is metric to test histogram`,
		"TEST Failed. histogram metrics registration failed")

	assert.Contains(t, stringBody,
		`histogram_test_bucket{otel_scope_name="testing-app",otel_scope_schema_url="",otel_scope_version="v1.0.0",le="0"} 0`,
		"TEST Failed. histogram metrics value did not reflect")
}

func Test_NewMetricsManagerMetricsNotRegistered(t *testing.T) {
	logs := func() {
		metrics := NewMetricsManager(exporters.Prometheus("testing-app", "v1.0.0"),
			logging.NewMockLogger(logging.INFO))

		metrics.SetGauge("gauge-test", 50)
		metrics.IncrementCounter(t.Context(), "counter-test")
		metrics.DeltaUpDownCounter(t.Context(), "up-down-counter", 10)
		metrics.RecordHistogram(t.Context(), "histogram-test", 1)
	}

	log := testutil.StderrOutputForFunc(logs)

	assert.Contains(t, log, `Metrics gauge-test is not registered`, "TEST Failed. gauge-test metrics registered")
	assert.Contains(t, log, `Metrics counter-test is not registered`, "TEST Failed. counter-test metrics registered")
	assert.Contains(t, log, `Metrics up-down-counter is not registered`, "TEST Failed. up-down-counter metrics registered")
	assert.Contains(t, log, `Metrics histogram-test is not registered`, "TEST Failed. histogram-test metrics registered")
}

func Test_NewMetricsManagerInvalidMetricsName(t *testing.T) {
	logs := func() {
		metrics := NewMetricsManager(exporters.Prometheus("testing-app", "v1.0.0"),
			logging.NewMockLogger(logging.INFO))

		metrics.NewCounter("", "counter metric with empty name")
		metrics.NewUpDownCounter("", "up-down-counter metric with empty name")
		metrics.NewHistogram("", "histogram metric with empty name")
		metrics.NewGauge("", "gauge metric with empty name")
	}

	log := testutil.StderrOutputForFunc(logs)

	assert.Contains(t, log, `invalid instrument name`, "TEST Failed. counter metric with empty name")
	assert.Contains(t, log, `invalid instrument name`, "TEST Failed. up-down-counter metric with empty name")
	assert.Contains(t, log, `invalid instrument name`, "TEST Failed. histogram metric with empty name")
	assert.Contains(t, log, `invalid instrument name`, "TEST Failed. gauge metric with empty name")
}

func Test_NewMetricsManagerDuplicateMetricsRegistration(t *testing.T) {
	logs := func() {
		metrics := NewMetricsManager(exporters.Prometheus("testing-app", "v1.0.0"),
			logging.NewMockLogger(logging.INFO))

		metrics.NewGauge("gauge-test", "this is metric to test gauge")
		metrics.NewCounter("counter-test", "this is metric to test counter")
		metrics.NewUpDownCounter("up-down-counter", "this is metric to test up-down-counter")
		metrics.NewHistogram("histogram-test", "this is metric to test histogram")

		metrics.NewGauge("gauge-test", "this is metric to test gauge")
		metrics.NewCounter("counter-test", "this is metric to test counter")
		metrics.NewUpDownCounter("up-down-counter", "this is metric to test up-down-counter")
		metrics.NewHistogram("histogram-test", "this is metric to test histogram")
	}

	log := testutil.StderrOutputForFunc(logs)

	assert.Contains(t, log, `Metrics gauge-test already registered`, "TEST Failed. gauge-test metrics not registered")
	assert.Contains(t, log, `Metrics counter-test already registered`, "TEST Failed. counter-test metrics not registered")
	assert.Contains(t, log, `Metrics up-down-counter already registered`, "TEST Failed. up-down-counter metrics not registered")
	assert.Contains(t, log, `Metrics up-down-counter already registered`, "TEST Failed. histogram-test metrics not registered")
}

func Test_NewMetricsManagerInvalidLabelPairErrors(t *testing.T) {
	logs := func() {
		metrics := NewMetricsManager(exporters.Prometheus("testing-app", "v1.0.0"),
			logging.NewMockLogger(logging.INFO))

		metrics.NewCounter("counter-test", "this is metric to test counter")

		metrics.IncrementCounter(t.Context(), "counter-test",
			"label1", "value1", "label2", "value2", "label3")
	}

	log := testutil.StdoutOutputForFunc(logs)

	assert.Contains(t, log, `metrics counter-test label has invalid key-value pairs`, "TEST Failed. Invalid key-value pair for labels")
}

func Test_NewMetricsManagerLabelHighCardinality(t *testing.T) {
	logs := func() {
		metrics := NewMetricsManager(exporters.Prometheus("testing-app", "v1.0.0"),
			logging.NewMockLogger(logging.INFO))

		metrics.NewCounter("counter-test", "this is metric to test counter")

		metrics.IncrementCounter(t.Context(), "counter-test",
			"label1", "value1", "label2", "value2", "label3", "value3", "label4", "value4", "label5", "value5", "label6", "value6",
			"label7", "value7", "label8", "value8", "label9", "value9", "label10", "value10", "label11", "value11", "label12", "value12")
	}

	log := testutil.StdoutOutputForFunc(logs)

	assert.Contains(t, log, `metrics counter-test has high cardinality: 24`, "TEST Failed. high cardinality of metrics")
}

// BenchmarkAttrBuild_HTTP measures the per-request cost of building the
// 3-label attribute slice for an HTTP metric: {path, method, status}.
// Every request hits this function via the HTTP metrics middleware.
//
// PR-9 target: precompute the route-static portion ({path, method}) at
// route registration time and append only `status` per request. Should
// drop B/op and allocs/op substantially after that PR.
func BenchmarkAttrBuild_HTTP(b *testing.B) {
	mgr := NewMetricsManager(
		exporters.Prometheus("bench", "v0.0.0"),
		logging.NewMockLogger(logging.ERROR),
	).(*metricsManager)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = mgr.getAttributes(
			"app_http_response",
			"path", "/users/{id}",
			"method", "GET",
			"status", "200",
		)
	}
}

// benchHistogramAttrs are the labels a request metric carries: for a given route, method and status
// they are byte-identical on every single observation.
func benchHistogramAttrs() []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("path", "/users/{id}"),
		attribute.String("method", http.MethodGet),
		attribute.String("status", "200"),
	}
}

// benchManager returns the concrete manager, because the option-based recorder is a capability of
// the implementation rather than part of the Manager interface -- callers reach it by assertion.
func benchManager(b *testing.B) *metricsManager {
	b.Helper()

	cfg := exporters.Config{AppName: "bench-app", AppVersion: "v1.0.0"}
	shutdown, meter := exporters.Build(b.Context(), &cfg, logging.NewMockLogger(logging.ERROR))

	b.Cleanup(func() {
		if shutdown != nil {
			_ = shutdown(context.Background())
		}
	})

	m, ok := NewMetricsManager(meter, logging.NewMockLogger(logging.ERROR)).(*metricsManager)
	if !ok {
		b.Fatal("NewMetricsManager did not return *metricsManager")
	}

	m.NewHistogram("bench-histogram", "histogram used by the benchmarks")

	return m
}

// BenchmarkRecordHistogramAttrs is the cost of an observation when the measurement option is rebuilt
// each time: metric.WithAttributes sorts and deduplicates the attributes into a new attribute.Set
// and wraps it, on every single request.
func BenchmarkRecordHistogramAttrs(b *testing.B) {
	m := benchManager(b)
	attrs := benchHistogramAttrs()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		m.RecordHistogramAttrs(b.Context(), "bench-histogram", 1, attrs...)
	}
}

// BenchmarkRecordHistogramOpt is the same observation with the option built once by the caller and
// reused, which is what a request metric can do because its label combinations come from a small
// fixed set.
func BenchmarkRecordHistogramOpt(b *testing.B) {
	m := benchManager(b)

	cached := []metric.RecordOption{metric.WithAttributes(benchHistogramAttrs()...)}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		m.RecordHistogramOpt(b.Context(), "bench-histogram", 1, cached...)
	}
}
