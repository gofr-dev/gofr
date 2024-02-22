package metrics

import (
	"context"
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/metrics/exporters"
	"gofr.dev/pkg/gofr/testutil"
	"io"
	"net/http/httptest"
	"testing"
)

func Test_NewMetricsManagerSuccess(t *testing.T) {
	metrics := NewMetricsManager(exporters.Prometheus("testing-app", "v1.0.0"),
		testutil.NewMockLogger(testutil.INFOLOG))

	metrics.NewGauge("gauge-test", "this is metric to test gauge")
	metrics.NewCounter("counter-test", "this is metric to test counter")
	metrics.NewUpDownCounter("up-down-counter", "this is metric to test up-down-counter")
	metrics.NewHistogram("histogram-test", "this is metric to test histogram")

	metrics.SetGauge("gauge-test", 50)
	metrics.IncrementCounter(context.Background(), "counter-test")
	metrics.DeltaUpDownCounter(context.Background(), "up-down-counter", 10)
	metrics.RecordHistogram(context.Background(), "histogram-test", 1)

	server := httptest.NewServer(GetHandler(metrics))

	resp, _ := server.Client().Get(server.URL + "/metrics")

	body, _ := io.ReadAll(resp.Body)

	stringBody := string(body)

	assert.Contains(t, stringBody, `otel_scope_name="testing-app",otel_scope_version="v1.0.0"`,
		"TEST Failed. service name and version not coming in metrics")

	assert.Contains(t, stringBody, `counter_test_total this is metric to test counter`,
		"TEST Failed. counter-test metrics registration failed")

	assert.Contains(t, stringBody, `counter_test_total{otel_scope_name="testing-app",otel_scope_version="v1.0.0"} 1`,
		"TEST Failed. gauge-test metrics registration failed")

	assert.Contains(t, stringBody, `gauge_test this is metric to test gauge`, "TEST Failed. gauge-test metrics registration failed")

	assert.Contains(t, stringBody, `gauge_test{otel_scope_name="testing-app",otel_scope_version="v1.0.0"} 50`,
		"TEST Failed. gauge_test metrics value not set")

	assert.Contains(t, stringBody, `p_down_counter{otel_scope_name="testing-app",otel_scope_version="v1.0.0"} 10`,
		"TEST Failed. up-down-counter metrics value did not reflect")

	assert.Contains(t, stringBody, `up_down_counter this is metric to test up-down-counter`,
		"TEST Failed. up-down-counter metrics registration failed")

	assert.Contains(t, stringBody, `histogram_test this is metric to test histogram`,
		"TEST Failed. histogram metrics registration failed")

	assert.Contains(t, stringBody, `histogram_test_bucket{otel_scope_name="testing-app",otel_scope_version="v1.0.0",le="0"} 0`,
		"TEST Failed. histogram metrics value did not reflect")
}

func Test_NewMetricsManagerMetricsNotRegistered(t *testing.T) {
	getLogs := func() {
		metrics := NewMetricsManager(exporters.Prometheus("testing-app", "v1.0.0"),
			testutil.NewMockLogger(testutil.INFOLOG))

		metrics.SetGauge("gauge-test", 50)
		metrics.IncrementCounter(context.Background(), "counter-test")
		metrics.DeltaUpDownCounter(context.Background(), "up-down-counter", 10)
		metrics.RecordHistogram(context.Background(), "histogram-test", 1)
	}

	log := testutil.StderrOutputForFunc(getLogs)

	assert.Contains(t, log, `Metrics gauge-test is not registered`, "TEST Failed. gauge-test metrics registered")
	assert.Contains(t, log, `Metrics counter-test is not registered`, "TEST Failed. counter-test metrics registered")
	assert.Contains(t, log, `Metrics up-down-counter is not registered`, "TEST Failed. up-down-counter metrics registered")
	assert.Contains(t, log, `Metrics histogram-test is not registered`, "TEST Failed. histogram-test metrics registered")
}

func Test_NewMetricsManagerDuplicateMetricsRegistration(t *testing.T) {
	getLogs := func() {
		metrics := NewMetricsManager(exporters.Prometheus("testing-app", "v1.0.0"),
			testutil.NewMockLogger(testutil.INFOLOG))

		metrics.NewGauge("gauge-test", "this is metric to test gauge")
		metrics.NewCounter("counter-test", "this is metric to test counter")
		metrics.NewUpDownCounter("up-down-counter", "this is metric to test up-down-counter")
		metrics.NewHistogram("histogram-test", "this is metric to test histogram")

		metrics.NewGauge("gauge-test", "this is metric to test gauge")
		metrics.NewCounter("counter-test", "this is metric to test counter")
		metrics.NewUpDownCounter("up-down-counter", "this is metric to test up-down-counter")
		metrics.NewHistogram("histogram-test", "this is metric to test histogram")
	}

	log := testutil.StderrOutputForFunc(getLogs)

	assert.Contains(t, log, `Metrics gauge-test already registered`, "TEST Failed. gauge-test metrics not registered")
	assert.Contains(t, log, `Metrics counter-test already registered`, "TEST Failed. counter-test metrics not registered")
	assert.Contains(t, log, `Metrics up-down-counter already registered`, "TEST Failed. up-down-counter metrics not registered")
	assert.Contains(t, log, `Metrics up-down-counter already registered`, "TEST Failed. histogram-test metrics not registered")
}

func Test_NewMetricsManagerInvalidLabelPairErrors(t *testing.T) {
	getLogs := func() {
		metrics := NewMetricsManager(exporters.Prometheus("testing-app", "v1.0.0"),
			testutil.NewMockLogger(testutil.INFOLOG))

		metrics.NewCounter("counter-test", "this is metric to test counter")

		metrics.IncrementCounter(context.Background(), "counter-test",
			"label1", "value1", "label2", "value2", "label3")
	}

	log := testutil.StdoutOutputForFunc(getLogs)

	assert.Contains(t, log, `Metrics counter-test label has invalid key-value pairs`, "TEST Failed. Invalid key-value pair for labels")
}

func Test_NewMetricsManagerLabelHighCardianality(t *testing.T) {
	getLogs := func() {
		metrics := NewMetricsManager(exporters.Prometheus("testing-app", "v1.0.0"),
			testutil.NewMockLogger(testutil.INFOLOG))

		metrics.NewCounter("counter-test", "this is metric to test counter")

		metrics.IncrementCounter(context.Background(), "counter-test",
			"label1", "value1", "label2", "value2", "label3", "value3", "label4", "value4", "label5", "value5", "label6", "value6",
			"label7", "value7", "label8", "value8", "label9", "value9", "label10", "value10", "label11", "valu11", "label12", "value12")
	}

	log := testutil.StdoutOutputForFunc(getLogs)

	assert.Contains(t, log, `Metrics counter-test has high cardinality: 24`, "TEST Failed. high cardinality of metrics")
}
