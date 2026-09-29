package gcp

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	metricSdk "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
)

// metricSpec describes one metric of a fixture: its scope, its aggregation kind
// and how many data points it carries.
type metricSpec struct {
	scope string
	kind  string
	n     int
}

func idSet(i int) attribute.Set { return attribute.NewSet(attribute.Int("i", i)) }

// idPoints builds n data points of type P, each carrying the next sequential "i"
// attribute. Every metricdata point type has an Attributes field.
func idPoints[P any](next *int, n int) []P {
	pts := make([]P, n)
	for k := range pts {
		reflect.ValueOf(&pts[k]).Elem().FieldByName("Attributes").Set(reflect.ValueOf(idSet(*next)))
		*next++
	}

	return pts
}

// buildRM builds a ResourceMetrics whose data points carry a sequential "i"
// attribute, so a test can check that splitting kept every point, in order.
// Consecutive specs with the same scope share one ScopeMetrics.
func buildRM(t *testing.T, specs ...metricSpec) *metricdata.ResourceMetrics {
	t.Helper()

	rm := &metricdata.ResourceMetrics{Resource: resource.NewSchemaless(attribute.String("service.name", "svc"))}
	next := 0

	for k, s := range specs {
		if len(rm.ScopeMetrics) == 0 || rm.ScopeMetrics[len(rm.ScopeMetrics)-1].Scope.Name != s.scope {
			rm.ScopeMetrics = append(rm.ScopeMetrics, metricdata.ScopeMetrics{Scope: instrumentation.Scope{Name: s.scope}})
		}

		build, ok := aggregations(&next, s.n)[s.kind]
		if !ok {
			t.Fatalf("unknown kind %q", s.kind)
		}

		m := metricdata.Metrics{
			Name: fmt.Sprintf("%s/%s-%d", s.scope, s.kind, k), Description: s.kind, Unit: "1", Data: build(),
		}

		sm := &rm.ScopeMetrics[len(rm.ScopeMetrics)-1]
		sm.Metrics = append(sm.Metrics, m)
	}

	return rm
}

// aggregations builds n data points for each aggregation kind the SDK exports,
// numbered from *next.
func aggregations(next *int, n int) map[string]func() metricdata.Aggregation {
	cum := metricdata.CumulativeTemporality

	return map[string]func() metricdata.Aggregation{
		"gauge-i": func() metricdata.Aggregation {
			return metricdata.Gauge[int64]{DataPoints: idPoints[metricdata.DataPoint[int64]](next, n)}
		},
		"gauge-f": func() metricdata.Aggregation {
			return metricdata.Gauge[float64]{DataPoints: idPoints[metricdata.DataPoint[float64]](next, n)}
		},
		"sum-i": func() metricdata.Aggregation {
			return metricdata.Sum[int64]{
				DataPoints: idPoints[metricdata.DataPoint[int64]](next, n), Temporality: cum, IsMonotonic: true,
			}
		},
		"sum-f": func() metricdata.Aggregation {
			return metricdata.Sum[float64]{DataPoints: idPoints[metricdata.DataPoint[float64]](next, n), Temporality: cum}
		},
		"hist-i": func() metricdata.Aggregation {
			return metricdata.Histogram[int64]{
				DataPoints: idPoints[metricdata.HistogramDataPoint[int64]](next, n), Temporality: cum,
			}
		},
		"hist-f": func() metricdata.Aggregation {
			return metricdata.Histogram[float64]{
				DataPoints: idPoints[metricdata.HistogramDataPoint[float64]](next, n), Temporality: cum,
			}
		},
		"exphist-i": func() metricdata.Aggregation {
			return metricdata.ExponentialHistogram[int64]{
				DataPoints: idPoints[metricdata.ExponentialHistogramDataPoint[int64]](next, n), Temporality: cum,
			}
		},
		"exphist-f": func() metricdata.Aggregation {
			return metricdata.ExponentialHistogram[float64]{
				DataPoints: idPoints[metricdata.ExponentialHistogramDataPoint[float64]](next, n), Temporality: cum,
			}
		},
		"summary": func() metricdata.Aggregation {
			return metricdata.Summary{DataPoints: idPoints[metricdata.SummaryDataPoint](next, n)}
		},
	}
}

// pointIDs lists the "i" attribute of every data point in rm, in order (0 when a
// point has none, as SDK-produced data does, so its length is still the count).
// It reads the points by reflection, independently of the production counting,
// so the tests do not derive their expectation from the code under test.
func pointIDs(t *testing.T, rm *metricdata.ResourceMetrics) []int {
	t.Helper()

	var ids []int

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			dps := reflect.ValueOf(m.Data).FieldByName("DataPoints")
			if !dps.IsValid() {
				t.Fatalf("pointIDs: %T has no DataPoints", m.Data)
			}

			for k := range dps.Len() {
				set, _ := dps.Index(k).FieldByName("Attributes").Interface().(attribute.Set)
				v, _ := set.Value("i")
				ids = append(ids, int(v.AsInt64()))
			}
		}
	}

	return ids
}

// shape describes a metric without its data points, so a split metric can be
// compared with the one it came from.
func shape(m metricdata.Metrics) string {
	data := reflect.New(reflect.TypeOf(m.Data)).Elem()
	data.Set(reflect.ValueOf(m.Data))
	data.FieldByName("DataPoints").SetZero()

	return fmt.Sprintf("%s|%s|%s|%T|%+v", m.Name, m.Description, m.Unit, m.Data, data.Interface())
}

// shapes maps every metric in rm, by name, to its shape.
func shapes(rm *metricdata.ResourceMetrics) map[string]string {
	out := map[string]string{}

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			out[m.Name] = shape(m)
		}
	}

	return out
}

// checkChunk asserts that a chunk kept the source's resource, lists each scope
// once, files every metric under the scope it came from and kept its metadata.
func checkChunk(t *testing.T, c *metricdata.ResourceMetrics, res *resource.Resource, want map[string]string) {
	t.Helper()

	if c.Resource != res {
		t.Errorf("chunk lost the resource")
	}

	seen := map[string]bool{}

	for _, sm := range c.ScopeMetrics {
		if seen[sm.Scope.Name] {
			t.Errorf("scope %q appears twice in one chunk", sm.Scope.Name)
		}

		seen[sm.Scope.Name] = true

		for _, m := range sm.Metrics {
			if !strings.HasPrefix(m.Name, sm.Scope.Name+"/") {
				t.Errorf("metric %q placed under scope %q", m.Name, sm.Scope.Name)
			}

			if shape(m) != want[m.Name] {
				t.Errorf("metric metadata changed: got %s, want %s", shape(m), want[m.Name])
			}
		}
	}
}

func Test_splitResourceMetrics(t *testing.T) {
	// Every aggregation kind, over three scopes, with chunk boundaries falling
	// inside a metric (gauge-f) and on a scope boundary.
	mixed := []metricSpec{
		{"a", "gauge-i", 150}, {"a", "gauge-f", 100},
		{"b", "sum-i", 20}, {"b", "sum-f", 30}, {"b", "hist-i", 40},
		{"b", "hist-f", 10}, {"b", "exphist-i", 20}, {"b", "exphist-f", 30},
		{"c", "summary", 50},
	}

	tests := []struct {
		name      string
		specs     []metricSpec
		limit     int
		wantSizes []int
	}{
		{"no scopes", nil, 200, []int{0}},
		{"under the cap", []metricSpec{{"a", "gauge-i", 199}}, 200, []int{199}},
		{"exactly the cap", []metricSpec{{"a", "gauge-i", 200}}, 200, []int{200}},
		{"one over the cap", []metricSpec{{"a", "gauge-i", 201}}, 200, []int{200, 1}},
		{"one metric larger than two requests", []metricSpec{{"a", "hist-f", 450}}, 200, []int{200, 200, 50}},
		{"spans metrics, scopes and every aggregation", mixed, 200, []int{200, 200, 50}},
		{"limit of zero disables splitting", mixed, 0, []int{450}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := buildRM(t, tc.specs...)
			srcIDs := pointIDs(t, src)
			srcShapes := shapes(src)

			var gotSizes, gotIDs []int

			for _, c := range splitResourceMetrics(src, tc.limit) {
				ids := pointIDs(t, c)
				gotSizes = append(gotSizes, len(ids))
				gotIDs = append(gotIDs, ids...)

				checkChunk(t, c, src.Resource, srcShapes)
			}

			if !slices.Equal(gotSizes, tc.wantSizes) {
				t.Errorf("chunk sizes = %v, want %v", gotSizes, tc.wantSizes)
			}

			if !slices.Equal(gotIDs, srcIDs) {
				t.Errorf("points were lost, duplicated or reordered: got %d, want %d", len(gotIDs), len(srcIDs))
			}

			if after := pointIDs(t, src); !slices.Equal(after, srcIDs) {
				t.Errorf("splitting mutated the source")
			}
		})
	}
}

var errUpload = errors.New("rpc error: code = InvalidArgument")

// recordingExporter records how many points each Export call carried and can
// fail chosen calls.
type recordingExporter struct {
	t      *testing.T
	mu     sync.Mutex
	calls  []int
	failOn map[int]error
}

func (e *recordingExporter) Export(_ context.Context, rm *metricdata.ResourceMetrics) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	i := len(e.calls)
	e.calls = append(e.calls, len(pointIDs(e.t, rm)))

	return e.failOn[i]
}

func (e *recordingExporter) snapshot() []int {
	e.mu.Lock()
	defer e.mu.Unlock()

	return slices.Clone(e.calls)
}

func (*recordingExporter) Temporality(k metricSdk.InstrumentKind) metricdata.Temporality {
	return metricSdk.DefaultTemporalitySelector(k)
}

func (*recordingExporter) Aggregation(k metricSdk.InstrumentKind) metricSdk.Aggregation {
	return metricSdk.DefaultAggregationSelector(k)
}

func (*recordingExporter) ForceFlush(context.Context) error { return nil }
func (*recordingExporter) Shutdown(context.Context) error   { return nil }

func Test_chunkingExporter_Export(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := []struct {
		name      string
		ctx       context.Context //nolint:containedctx // table input.
		points    int
		failOn    map[int]error
		wantCalls []int
		wantErr   error
	}{
		{"under the cap is one request", context.Background(), 10, nil, []int{10}, nil},
		{"delivers every chunk", context.Background(), 450, nil, []int{200, 200, 50}, nil},
		{
			"a rejected chunk does not drop the rest", context.Background(), 450,
			map[int]error{1: errUpload}, []int{200, 200, 50}, errUpload,
		},
		{"a canceled context sends nothing", canceled, 450, nil, nil, context.Canceled},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingExporter{t: t, failOn: tc.failOn}
			exp := &chunkingExporter{Exporter: rec, limit: maxPointsPerRequest}

			err := exp.Export(tc.ctx, buildRM(t, metricSpec{"a", "gauge-i", tc.points}))

			if got := rec.snapshot(); !slices.Equal(got, tc.wantCalls) {
				t.Errorf("requests = %v, want %v", got, tc.wantCalls)
			}

			if !errors.Is(err, tc.wantErr) {
				t.Errorf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

// Test_newReader drives the real SDK pipeline, so it proves the reader the
// exporter hands out actually splits what a MeterProvider collects.
func Test_newReader(t *testing.T) {
	tests := []struct {
		name      string
		sdkBatch  string
		wantCalls []int
	}{
		{"splits at the Telemetry API cap", "", []int{200, 200, 50}},
		// The SDK's experimental batcher may already have split the collection;
		// the exporter must then pass the smaller batches through untouched.
		{"composes with the SDK's own batching", "150", []int{150, 150, 150}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OTEL_GO_X_METRIC_EXPORT_BATCH_SIZE", tc.sdkBatch)

			rec := &recordingExporter{t: t}
			reader := newReader(rec, time.Hour)
			provider := metricSdk.NewMeterProvider(metricSdk.WithReader(reader))

			counter, err := provider.Meter("test").Int64Counter("requests")
			if err != nil {
				t.Fatal(err)
			}

			for i := range 450 {
				counter.Add(context.Background(), 1, metric.WithAttributes(attribute.Int("path", i)))
			}

			if err := provider.ForceFlush(context.Background()); err != nil {
				t.Fatalf("ForceFlush: %v", err)
			}

			got := rec.snapshot()

			_ = provider.Shutdown(context.Background())

			if !slices.Equal(got, tc.wantCalls) {
				t.Errorf("requests = %v, want %v", got, tc.wantCalls)
			}
		})
	}
}
