package exporters

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"gofr.dev/pkg/gofr/logging"
)

func TestNewPushGateway(t *testing.T) {
	l := logging.NewMockLogger(logging.DEBUG)
	pg := NewPushGateway("http://localhost:9091", "test-job", prometheus.DefaultGatherer, l)

	assert.NotNil(t, pg)
	assert.Equal(t, "http://localhost:9091/metrics/job/test-job", pg.pushURL)
	assert.Equal(t, "http://localhost:9091/metrics", pg.metricsURL)
	assert.Equal(t, l, pg.logger)
}

func TestNewPushGateway_TrailingSlash(t *testing.T) {
	l := logging.NewMockLogger(logging.DEBUG)
	pg := NewPushGateway("http://localhost:9091/", "my-job", prometheus.DefaultGatherer, l)

	assert.Equal(t, "http://localhost:9091/metrics/job/my-job", pg.pushURL)
	assert.Equal(t, "http://localhost:9091/metrics", pg.metricsURL)
}

func TestPush_FirstRun(t *testing.T) {
	var putBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			putBody = string(body)

			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	registry := prometheus.NewRegistry()
	counter := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_counter", Help: "test"})
	registry.MustRegister(counter)
	counter.Inc()

	l := logging.NewMockLogger(logging.DEBUG)
	pg := NewPushGateway(server.URL, "test-job", registry, l)

	err := pg.Push(context.Background())

	require.NoError(t, err)
	assert.Contains(t, putBody, "test_counter")
	assert.Contains(t, putBody, "1")
}

func TestPush_CounterAccumulation(t *testing.T) {
	existingMetrics := `# HELP test_counter test
# TYPE test_counter counter
test_counter 5
`

	var putBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/plain")

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(existingMetrics))
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			putBody = string(body)

			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	registry := prometheus.NewRegistry()
	counter := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_counter", Help: "test"})
	registry.MustRegister(counter)
	counter.Inc()

	l := logging.NewMockLogger(logging.DEBUG)
	pg := NewPushGateway(server.URL, "test-job", registry, l)

	err := pg.Push(context.Background())

	require.NoError(t, err)

	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(putBody))
	require.NoError(t, err)

	mf, ok := families["test_counter"]
	require.True(t, ok)
	assert.InDelta(t, 6.0, mf.GetMetric()[0].GetCounter().GetValue(), 0.01)
}

func TestPush_CounterAccumulationWithLabels(t *testing.T) {
	existingMetrics := `# HELP cmd_success test
# TYPE cmd_success counter
cmd_success{command="hello"} 3
cmd_success{command="batch"} 2
`

	var putBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/plain")

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(existingMetrics))
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			putBody = string(body)

			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	registry := prometheus.NewRegistry()
	counter := prometheus.NewCounterVec(prometheus.CounterOpts{Name: "cmd_success", Help: "test"}, []string{"command"})
	registry.MustRegister(counter)
	counter.WithLabelValues("hello").Inc()

	l := logging.NewMockLogger(logging.DEBUG)
	pg := NewPushGateway(server.URL, "test-job", registry, l)

	err := pg.Push(context.Background())

	require.NoError(t, err)

	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(putBody))
	require.NoError(t, err)

	mf := families["cmd_success"]
	require.NotNil(t, mf)

	for _, m := range mf.GetMetric() {
		for _, lp := range m.GetLabel() {
			if lp.GetName() == "command" && lp.GetValue() == "hello" {
				assert.InDelta(t, 4.0, m.GetCounter().GetValue(), 0.01)
			}
		}
	}
}

func TestPush_GaugeReplacement(t *testing.T) {
	existingMetrics := `# HELP last_ts test
# TYPE last_ts gauge
last_ts 1000
`

	var putBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/plain")

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(existingMetrics))
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			putBody = string(body)

			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	registry := prometheus.NewRegistry()
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "last_ts", Help: "test"})
	registry.MustRegister(gauge)
	gauge.Set(2000)

	l := logging.NewMockLogger(logging.DEBUG)
	pg := NewPushGateway(server.URL, "test-job", registry, l)

	err := pg.Push(context.Background())

	require.NoError(t, err)

	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(putBody))
	require.NoError(t, err)

	mf := families["last_ts"]
	require.NotNil(t, mf)

	assert.InDelta(t, 2000.0, mf.GetMetric()[0].GetGauge().GetValue(), 0.01)
}

func TestPush_HistogramMerge(t *testing.T) {
	existingMetrics := `# HELP duration test
# TYPE duration histogram
duration_bucket{le="0.5"} 2
duration_bucket{le="1"} 3
duration_bucket{le="+Inf"} 4
duration_sum 2.5
duration_count 4
`

	var putBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/plain")

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(existingMetrics))
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			putBody = string(body)

			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	registry := prometheus.NewRegistry()
	hist := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "duration",
		Help:    "test",
		Buckets: []float64{0.5, 1},
	})
	registry.MustRegister(hist)
	hist.Observe(0.3)

	l := logging.NewMockLogger(logging.DEBUG)
	pg := NewPushGateway(server.URL, "test-job", registry, l)

	err := pg.Push(context.Background())

	require.NoError(t, err)

	parser := expfmt.NewTextParser(model.LegacyValidation)
	families, err := parser.TextToMetricFamilies(strings.NewReader(putBody))
	require.NoError(t, err)

	mf := families["duration"]
	require.NotNil(t, mf)

	h := mf.GetMetric()[0].GetHistogram()

	assert.Equal(t, uint64(5), h.GetSampleCount())
	assert.InDelta(t, 2.8, h.GetSampleSum(), 0.01)

	for _, b := range h.GetBucket() {
		if b.GetUpperBound() == 0.5 {
			assert.Equal(t, uint64(3), b.GetCumulativeCount())
		}
	}
}

func TestPush_FetchFailure_PushesLocalOnly(t *testing.T) {
	var putReceived bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusInternalServerError)
		case http.MethodPut:
			putReceived = true

			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	registry := prometheus.NewRegistry()
	counter := prometheus.NewCounter(prometheus.CounterOpts{Name: "test_counter", Help: "test"})
	registry.MustRegister(counter)
	counter.Inc()

	l := logging.NewMockLogger(logging.DEBUG)
	pg := NewPushGateway(server.URL, "test-job", registry, l)

	err := pg.Push(context.Background())

	require.NoError(t, err)
	assert.True(t, putReceived)
}

func TestPush_PutFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
		case http.MethodPut:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	l := logging.NewMockLogger(logging.DEBUG)
	pg := NewPushGateway(server.URL, "test-job", prometheus.NewRegistry(), l)

	err := pg.Push(context.Background())

	require.Error(t, err)
}

func TestPush_ContextTimeout(t *testing.T) {
	l := logging.NewMockLogger(logging.DEBUG)
	pg := NewPushGateway("http://192.0.2.1:1", "test-job", prometheus.NewRegistry(), l)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := pg.Push(ctx)

	require.Error(t, err)
}

func TestPush_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	l := logging.NewMockLogger(logging.DEBUG)
	pg := NewPushGateway(server.URL, "test-job", prometheus.NewRegistry(), l)

	err := pg.Push(context.Background())

	require.Error(t, err)
}

func TestPush_CustomRegistry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	registry := prometheus.NewRegistry()
	l := logging.NewMockLogger(logging.DEBUG)
	pg := NewPushGateway(server.URL, "test-job", registry, l)

	err := pg.Push(context.Background())

	require.NoError(t, err)
}

func TestMergeMetrics_NoExisting(t *testing.T) {
	counter := dto.MetricType_COUNTER
	name := "test_counter"
	val := 1.0

	local := []*dto.MetricFamily{
		{
			Name: &name,
			Type: &counter,
			Metric: []*dto.Metric{
				{Counter: &dto.Counter{Value: &val}},
			},
		},
	}

	result := mergeMetrics(nil, local)

	require.Len(t, result, 1)
	assert.InDelta(t, 1.0, result[0].GetMetric()[0].GetCounter().GetValue(), 0.01)
}

func TestMergeMetrics_CounterMerge(t *testing.T) {
	counter := dto.MetricType_COUNTER
	name := "test_counter"
	existingVal := 5.0
	localVal := 1.0

	existing := map[string]*dto.MetricFamily{
		"test_counter": {
			Name: &name,
			Type: &counter,
			Metric: []*dto.Metric{
				{Counter: &dto.Counter{Value: &existingVal}},
			},
		},
	}

	local := []*dto.MetricFamily{
		{
			Name: &name,
			Type: &counter,
			Metric: []*dto.Metric{
				{Counter: &dto.Counter{Value: &localVal}},
			},
		},
	}

	result := mergeMetrics(existing, local)

	require.Len(t, result, 1)
	assert.InDelta(t, 6.0, result[0].GetMetric()[0].GetCounter().GetValue(), 0.01)
}

func TestMergeMetrics_GaugeReplace(t *testing.T) {
	gauge := dto.MetricType_GAUGE
	name := "test_gauge"
	existingVal := 1000.0
	localVal := 2000.0

	existing := map[string]*dto.MetricFamily{
		"test_gauge": {
			Name: &name,
			Type: &gauge,
			Metric: []*dto.Metric{
				{Gauge: &dto.Gauge{Value: &existingVal}},
			},
		},
	}

	local := []*dto.MetricFamily{
		{
			Name: &name,
			Type: &gauge,
			Metric: []*dto.Metric{
				{Gauge: &dto.Gauge{Value: &localVal}},
			},
		},
	}

	result := mergeMetrics(existing, local)

	require.Len(t, result, 1)
	assert.InDelta(t, 2000.0, result[0].GetMetric()[0].GetGauge().GetValue(), 0.01)
}

func TestLabelKey(t *testing.T) {
	labels := []*dto.LabelPair{
		{Name: proto.String("command"), Value: proto.String("hello")},
		{Name: proto.String("region"), Value: proto.String("us-east")},
	}

	key := labelKey(labels)
	assert.Equal(t, "command=hello,region=us-east", key)
}

func TestLabelKey_FiltersExternalLabels(t *testing.T) {
	labels := []*dto.LabelPair{
		{Name: proto.String("command"), Value: proto.String("hello")},
		{Name: proto.String("instance"), Value: proto.String("")},
		{Name: proto.String("job"), Value: proto.String("cli-hello")},
		{Name: proto.String("otel_scope_name"), Value: proto.String("cli-hello")},
		{Name: proto.String("otel_scope_schema_url"), Value: proto.String("")},
		{Name: proto.String("otel_scope_version"), Value: proto.String("dev")},
	}

	key := labelKey(labels)
	assert.Equal(t, "command=hello", key)
}

func TestLabelKey_Empty(t *testing.T) {
	key := labelKey(nil)
	assert.Empty(t, key)
}
