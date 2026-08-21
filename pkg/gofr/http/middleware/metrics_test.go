package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type mockMetrics struct {
	mock.Mock
}

func (m *mockMetrics) IncrementCounter(ctx context.Context, name string, labels ...string) {
	m.Called(ctx, name, labels)
}

func (m *mockMetrics) DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string) {
	m.Called(ctx, name, value, labels)
}

func (m *mockMetrics) RecordHistogram(ctx context.Context, name string, value float64, labels ...string) {
	m.Called(ctx, name, value, labels)
}

func (m *mockMetrics) SetGauge(name string, value float64, _ ...string) {
	m.Called(name, value)
}

func TestMetrics(t *testing.T) {
	mockMetrics := &mockMetrics{}

	mockMetrics.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	router := mux.NewRouter()
	router.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/test")

	route := router.NewRoute()
	route.Path("/test").Name("/test")

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rr := httptest.NewRecorder()

	router.Use(Metrics(mockMetrics))

	router.ServeHTTP(rr, req)

	mockMetrics.AssertCalled(t, "RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/test", "method", "GET", "status", "200"})
}

func TestMetrics_StaticFile(t *testing.T) {
	mockMetrics := &mockMetrics{}

	mockMetrics.On("RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/static/example.js", "method", "GET", "status", "200"}).Return(nil)

	// Create a temporary static file for the test
	tempDir := t.TempDir()
	staticFilePath := tempDir + "/example.js"

	err := os.WriteFile(staticFilePath, []byte("console.log('test');"), 0600)
	if err != nil {
		t.Errorf("failed to create temporary static file: %v", err)
	}

	router := mux.NewRouter()
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(tempDir)))).Name("/static/")

	router.Use(Metrics(mockMetrics))

	req := httptest.NewRequest(http.MethodGet, "/static/example.js", http.NoBody)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	mockMetrics.AssertCalled(t, "RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/static/example.js", "method", "GET", "status", "200"})
}

// TestMetrics_GraphQLSkipsRootOnly asserts that the Metrics middleware
// skips recording app_http_response for the canonical /graphql endpoint
// (which emits its own app_graphql_* metrics) but DOES record for
// sub-paths like /graphql/playground. A future change that broadens the
// skip to a prefix match would silently drop sub-path metrics and must
// fail this test.
func TestMetrics_GraphQLSkipsRootOnly(t *testing.T) {
	mockMetrics := &mockMetrics{}

	// Allow any RecordHistogram call so we can later assert which paths
	// were recorded (vs absent) explicitly.
	mockMetrics.On("RecordHistogram",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil)

	router := mux.NewRouter()
	router.HandleFunc("/graphql", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/graphql")
	router.HandleFunc("/graphql/playground", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/graphql/playground")

	router.Use(Metrics(mockMetrics))

	rootReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/graphql", http.NoBody)
	router.ServeHTTP(httptest.NewRecorder(), rootReq)

	playgroundReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/graphql/playground", http.NoBody)
	router.ServeHTTP(httptest.NewRecorder(), playgroundReq)

	// /graphql is skipped — no RecordHistogram call for it.
	mockMetrics.AssertNotCalled(t, "RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/graphql", "method", "GET", "status", "200"})

	// /graphql/playground is recorded — sub-paths still get metrics.
	mockMetrics.AssertCalled(t, "RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/graphql/playground", "method", "GET", "status", "200"})
}

// TestMetrics_UnmatchedRouteDoesNotPanic asserts that the Metrics
// middleware survives a request that hits no route (404 path), where
// mux.CurrentRoute returns nil. Before this stack added the nil-guard,
// .GetPathTemplate() on a nil route panicked and crashed the server on
// any 404 — Copilot flagged this in review.
//
// We only assert the no-panic contract here; the (empty) path label
// emitted in that case is current behavior and out of scope for this
// regression test.
func TestMetrics_UnmatchedRouteDoesNotPanic(t *testing.T) {
	mockMetrics := &mockMetrics{}

	mockMetrics.On("RecordHistogram",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil)

	// Construct the handler chain WITHOUT a router so mux.CurrentRoute(r)
	// returns nil for every request. Wrap a 404 handler with Metrics
	// directly, exercising the nil-guard at metrics.go:84.
	notFound := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := Metrics(mockMetrics)(notFound)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/no/such/route", http.NoBody)
	rr := httptest.NewRecorder()

	require.NotPanics(t, func() { handler.ServeHTTP(rr, req) },
		"Metrics middleware must not panic when mux.CurrentRoute is nil")

	require.Equal(t, http.StatusNotFound, rr.Code, "404 handler still ran end-to-end")
}

func TestMetrics_StaticFileWithQueryParam(t *testing.T) {
	mockMetrics := &mockMetrics{}

	mockMetrics.On("RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/static/example.js", "method", "GET", "status", "200"}).Return(nil)

	// Create a temporary static file for the test
	tempDir := t.TempDir()
	staticFilePath := tempDir + "/example.js"

	err := os.WriteFile(staticFilePath, []byte("console.log('test');"), 0600)
	if err != nil {
		t.Errorf("failed to create temporary static file: %v", err)
	}

	router := mux.NewRouter()
	router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(tempDir)))).Name("/static/")

	router.Use(Metrics(mockMetrics))

	req := httptest.NewRequest(http.MethodGet, "/static/example.js?v=42", http.NoBody)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	mockMetrics.AssertCalled(t, "RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/static/example.js", "method", "GET", "status", "200"})
}

// mockOptRecorder implements both optional fast-path interfaces so the
// measurement-option path is exercised, and records what it was handed.
type mockOptRecorder struct {
	noopMetrics

	mu      sync.Mutex
	optHits int
	values  []float64
	// attrs holds the attribute set of every recorded observation, so a test
	// can assert WHAT was labeled and not merely that something was.
	attrs []attribute.Set
}

// noopMetrics satisfies the metrics interface without assertions, so the option
// path can be exercised without a mock expectation for every call.
type noopMetrics struct{}

func (noopMetrics) IncrementCounter(context.Context, string, ...string)            {}
func (noopMetrics) DeltaUpDownCounter(context.Context, string, float64, ...string) {}
func (noopMetrics) RecordHistogram(context.Context, string, float64, ...string)    {}
func (noopMetrics) SetGauge(string, float64, ...string)                            {}

func (m *mockOptRecorder) RecordHistogramOpt(_ context.Context, _ string, v float64, opts ...metric.RecordOption) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.optHits++
	m.values = append(m.values, v)
	m.attrs = append(m.attrs, metric.NewRecordConfig(opts).Attributes())
}

// attrMap flattens the i-th recorded attribute set into a map of key to the
// value AND its type, so a String->Int regression is visible.
func (m *mockOptRecorder) attrMap(i int) map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()

	out := map[string]string{}
	for _, kv := range m.attrs[i].ToSlice() {
		out[string(kv.Key)] = kv.Value.Type().String() + ":" + kv.Value.String()
	}

	return out
}

// TestMetricsOptRecorderIsUsedAndStillRecords pins that the faster path is taken
// and that every request is still observed exactly once.
func TestMetricsOptRecorderIsUsedAndStillRecords(t *testing.T) {
	rec := &mockOptRecorder{}

	router := mux.NewRouter()
	router.Use(Metrics(rec))
	router.NewRoute().Methods(http.MethodGet).Path("/users/{id}").
		HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	const n = 25
	for i := range n {
		router.ServeHTTP(httptest.NewRecorder(),
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users/"+strconv.Itoa(i), http.NoBody))
	}

	require.Equal(t, n, rec.optHits, "every request must still be recorded")

	for _, v := range rec.values {
		// Not Positive: a handler this trivial can complete inside the clock's
		// granularity, making a zero-second duration legitimate rather than a
		// defect. The upper bound is the assertion that earns its place — it
		// pins the unit, so passing nanoseconds here instead of seconds fails.
		require.GreaterOrEqual(t, v, 0.0, "duration must be a non-negative number of seconds")
		require.Less(t, v, 1.0, "a trivial handler records well under a second; larger means wrong units")
	}
}

// TestMetricsOptRecorderLabels pins WHAT the fast path emits. metricsManager
// implements RecordHistogramOpt, so production flows through optionRecorder --
// yet nothing asserted its labels, which meant changing status from
// attribute.String to attribute.Int, or emitting the concrete path instead of
// the template, kept every test green while dashboards broke and metric
// cardinality exploded.
func TestMetricsOptRecorderLabels(t *testing.T) {
	rec := &mockOptRecorder{}

	router := mux.NewRouter()
	router.Use(Metrics(rec))
	router.NewRoute().Methods(http.MethodGet).Path("/users/{id}").
		HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) })

	router.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users/7", http.NoBody))

	require.Equal(t, 1, rec.optHits)

	assert.Equal(t, map[string]string{
		// The ROUTE TEMPLATE, never the concrete path: "/users/7" here would put
		// one time series per user id into the backend.
		"path":   "STRING:/users/{id}",
		"method": "STRING:GET",
		// STRING, not INT64: metricsManager's varargs path emits status as a
		// string and OTLP distinguishes KeyValue types, so the two recorders
		// must agree or a query breaks depending on which one was selected.
		"status": "STRING:201",
	}, rec.attrMap(0))
}

// TestMetricsOptCacheIsBounded drives the guard rather than asserting that a
// compile-time constant is positive. Deleting the ceiling used to leave this
// test green.
func TestMetricsOptCacheIsBounded(t *testing.T) {
	require.Positive(t, routeCacheLimit, "the option cache must carry a ceiling")

	o := &optionRecorder{rec: &mockOptRecorder{}, statusAttr: newStatusAttrCache()}

	// Well past the ceiling, all templated and all with a cacheable method, so
	// nothing but the ceiling itself can stop the growth.
	for i := range routeCacheLimit + 1000 {
		o.record("/r/"+strconv.Itoa(i), http.MethodGet, http.StatusOK, 0.01, true)
	}

	assert.Equal(t, int64(routeCacheLimit), o.cache.len(),
		"the option cache must stop at its ceiling")
}

// TestMetricsCachesRejectUntemplatedPaths is the regression test for real routes
// being starved out of the cache by catch-all traffic.
//
// Every unmatched request resolves through GoFr's PathPrefix("/") catch-all and
// falls back to the raw URL path, which is caller-controlled. Those used to be
// cached, so a stream of unique unmatched paths filled the cache to its ceiling;
// after that every first-seen legitimate route could never be stored and rebuilt
// its measurement option per request forever. Memory stayed bounded and the
// optimization silently reverted to baseline for production traffic.
func TestMetricsCachesRejectUntemplatedPaths(t *testing.T) {
	t.Run("optionRecorder", func(t *testing.T) {
		o := &optionRecorder{rec: &mockOptRecorder{}, statusAttr: newStatusAttrCache()}

		for i := range 2000 {
			o.record("/attacker/"+strconv.Itoa(i), http.MethodGet, http.StatusOK, 0.01, false)
		}

		require.Zero(t, o.cache.len(), "a caller-controlled path must never be cached")

		// A real route is still cached, and is not competing with the noise.
		o.record("/users/{id}", http.MethodGet, http.StatusOK, 0.01, true)
		assert.Equal(t, int64(1), o.cache.len())
	})

	t.Run("attrsRecorder", func(t *testing.T) {
		a := &attrsRecorder{rec: &mockAttrsRecorder{}, statusAttr: newStatusAttrCache()}

		for i := range 2000 {
			a.record("/attacker/"+strconv.Itoa(i), http.MethodGet, http.StatusOK, 0.01, false)
		}

		require.Zero(t, a.cache.len(), "a caller-controlled path must never be cached")

		a.record("/users/{id}", http.MethodGet, http.StatusOK, 0.01, true)
		assert.Equal(t, int64(1), a.cache.len())
	})
}

// TestMetricsCachesRejectUndefinedMethods pins the other half of the key. The
// path is bounded by the route table, but net/http accepts any RFC 7230 token as
// a method, so an unbounded method half reopens the same growth on a bounded
// route.
func TestMetricsCachesRejectUndefinedMethods(t *testing.T) {
	o := &optionRecorder{rec: &mockOptRecorder{}, statusAttr: newStatusAttrCache()}
	a := &attrsRecorder{rec: &mockAttrsRecorder{}, statusAttr: newStatusAttrCache()}

	for i := range 2000 {
		o.record("/users/{id}", "M"+strconv.Itoa(i), http.StatusOK, 0.01, true)
		a.record("/users/{id}", "M"+strconv.Itoa(i), http.StatusOK, 0.01, true)
	}

	assert.Zero(t, o.cache.len(), "an undefined method must never mint an option-cache entry")
	assert.Zero(t, a.cache.len(), "an undefined method must never mint an attrs-cache entry")
}

// TestAttrsRecorderIsCorrectAndBounded covers attrsRecorder, which no in-tree
// type selects (*metricsManager implements both optional interfaces, so
// optionRecorder always wins) and which therefore ran at zero coverage while its
// cache had no ceiling at all.
func TestAttrsRecorderIsCorrectAndBounded(t *testing.T) {
	rec := &mockAttrsRecorder{}
	a := &attrsRecorder{rec: rec, statusAttr: newStatusAttrCache()}

	a.record("/users/{id}", http.MethodGet, http.StatusCreated, 0.02, true)

	require.Len(t, rec.attrs, 1)
	assert.Equal(t, map[string]string{
		"path":   "STRING:/users/{id}",
		"method": "STRING:GET",
		"status": "STRING:201",
	}, kvSlice(rec.attrs[0]))

	// Repeated calls reuse the cached base pair rather than growing.
	for range 100 {
		a.record("/users/{id}", http.MethodGet, http.StatusOK, 0.02, true)
	}

	assert.Equal(t, int64(1), a.cache.len())

	for i := range routeCacheLimit + 500 {
		a.record("/r/"+strconv.Itoa(i), http.MethodGet, http.StatusOK, 0.01, true)
	}

	assert.Equal(t, int64(routeCacheLimit), a.cache.len(),
		"the attrs cache must carry the same ceiling as the option cache")
}

// mockAttrsRecorder captures what the attribute fast path emits.
type mockAttrsRecorder struct {
	noopMetrics

	mu    sync.Mutex
	attrs [][]attribute.KeyValue
}

func (m *mockAttrsRecorder) RecordHistogramAttrs(_ context.Context, _ string, _ float64,
	attrs ...attribute.KeyValue,
) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.attrs = append(m.attrs, append([]attribute.KeyValue(nil), attrs...))
}

func kvSlice(kvs []attribute.KeyValue) map[string]string {
	out := map[string]string{}
	for _, kv := range kvs {
		out[string(kv.Key)] = kv.Value.Type().String() + ":" + kv.Value.String()
	}

	return out
}

// Characterization suite for the Metrics HTTP middleware.
//
// Everything below pins CURRENT behavior of pkg/gofr/http/middleware/metrics.go
// exactly as it is today. Where the behavior looks like a latent bug the test
// still asserts the observed value and says so in a comment — these tests are
// a tripwire, not a specification of what SHOULD happen.
//
// All identifiers introduced here are prefixed with metChar to stay collision
// free with the other _test.go files in this package.
// ---------------------------------------------------------------------------

// Recorded call kinds captured by metCharRecorder.
const (
	metCharKindHistogram = "RecordHistogram"
	metCharKindAttrs     = "RecordHistogramAttrs"
	metCharKindCounter   = "IncrementCounter"
	metCharKindUpDown    = "DeltaUpDownCounter"
	metCharKindGauge     = "SetGauge"
)

// metCharMetricName is the only metric the middleware is allowed to emit.
const metCharMetricName = "app_http_response"

// metCharCall is a single, fully captured metrics call.
type metCharCall struct {
	kind   string
	name   string
	value  float64
	labels []string
	attrs  []attribute.KeyValue
}

// metCharRecorder implements the unexported `metrics` interface ONLY, so the
// middleware takes the slow (string varargs) path. It is mutex protected
// because the concurrency test drives it from many goroutines under -race.
type metCharRecorder struct {
	mu    sync.Mutex
	calls []metCharCall
}

func (f *metCharRecorder) record(c *metCharCall) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls = append(f.calls, *c)
}

// all returns a copy of every call recorded so far.
func (f *metCharRecorder) all() []metCharCall {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]metCharCall, len(f.calls))
	copy(out, f.calls)

	return out
}

// one asserts that exactly one metrics call was recorded and returns it.
func (f *metCharRecorder) one(t *testing.T) *metCharCall {
	t.Helper()

	calls := f.all()
	require.Len(t, calls, 1, "expected exactly one recorded metrics call")

	return &calls[0]
}

func (f *metCharRecorder) IncrementCounter(_ context.Context, name string, labels ...string) {
	f.record(&metCharCall{kind: metCharKindCounter, name: name, labels: labels})
}

func (f *metCharRecorder) DeltaUpDownCounter(_ context.Context, name string, value float64, labels ...string) {
	f.record(&metCharCall{kind: metCharKindUpDown, name: name, value: value, labels: labels})
}

func (f *metCharRecorder) RecordHistogram(_ context.Context, name string, value float64, labels ...string) {
	f.record(&metCharCall{kind: metCharKindHistogram, name: name, value: value, labels: append([]string(nil), labels...)})
}

func (f *metCharRecorder) SetGauge(name string, value float64, labels ...string) {
	f.record(&metCharCall{kind: metCharKindGauge, name: name, value: value, labels: labels})
}

// metCharAttrRecorder additionally implements the unexported metricsAttrer
// optional interface, which switches the middleware onto the fast path.
type metCharAttrRecorder struct {
	*metCharRecorder
}

func (f *metCharAttrRecorder) RecordHistogramAttrs(_ context.Context, name string,
	value float64, attrs ...attribute.KeyValue) {
	f.record(&metCharCall{
		kind:  metCharKindAttrs,
		name:  name,
		value: value,
		attrs: append([]attribute.KeyValue(nil), attrs...),
	})
}

func metCharNewAttrRecorder() *metCharAttrRecorder {
	return &metCharAttrRecorder{metCharRecorder: &metCharRecorder{}}
}

// metCharLabelSet normalizes a recorded call to a flat key/value string slice
// so the fast and slow paths can be compared directly. For the fast path it
// also pins that EVERY attribute value is of type attribute.STRING.
func metCharLabelSet(t *testing.T, c *metCharCall) []string {
	t.Helper()

	if c.kind == metCharKindHistogram {
		return c.labels
	}

	labels := make([]string, 0, len(c.attrs)*2)

	for _, kv := range c.attrs {
		require.Equal(t, attribute.STRING, kv.Value.Type(),
			"attribute %q must carry a STRING value, got %v", kv.Key, kv.Value.Type())

		labels = append(labels, string(kv.Key), kv.Value.AsString())
	}

	return labels
}

// metCharOKHandler writes an explicit 200.
func metCharOKHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// metCharRegister builds a plain mux Path() registration for tpl.
func metCharRegister(tpl string) func(r *mux.Router, h http.Handler) {
	return func(r *mux.Router, h http.Handler) {
		r.Handle(tpl, h)
	}
}

// metCharChain builds the handler under test. When register is nil the Metrics
// middleware wraps h directly, so mux.CurrentRoute(r) is nil (the 404 shape).
// The returned handler owns exactly ONE Metrics instance, so repeated requests
// through it exercise the routeAttrs/statusAttrs caches.
func metCharChain(m metrics, register func(r *mux.Router, h http.Handler), h http.Handler) http.Handler {
	if register == nil {
		return Metrics(m)(h)
	}

	router := mux.NewRouter()
	register(router, h)
	router.Use(Metrics(m))

	return router
}

// metCharServe drives one request through handler.
func metCharServe(t *testing.T, handler http.Handler, method, target string) {
	t.Helper()

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), method, target, http.NoBody))
}

// metCharWant builds the canonical expected label slice in the exact order the
// middleware emits it.
func metCharWant(path, method, status string) []string {
	return []string{"path", path, "method", method, "status", status}
}

// ---------------------------------------------------------------------------
// 1. Metric identity, value unit, and exact slow-path label varargs.
// ---------------------------------------------------------------------------

// Test_MetricsContractSlowPathExactCall pins that an implementation satisfying
// only the `metrics` interface receives exactly one RecordHistogram call, with
// the exact metric name, a duration expressed in SECONDS as a float64, and the
// exact ordered varargs slice ["path", p, "method", m, "status", "<code>"]
// where status is a STRING produced by fmt.Sprintf("%d").
func Test_MetricsContractSlowPathExactCall(t *testing.T) {
	rec := &metCharRecorder{}

	slow := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	handler := metCharChain(rec, metCharRegister("/users/{id}"), slow)
	metCharServe(t, handler, http.MethodGet, "/users/42")

	call := rec.one(t)

	require.Equal(t, metCharKindHistogram, call.kind, "slow path must use RecordHistogram")
	require.Equal(t, "app_http_response", call.name, "metric name is fixed")
	require.Equal(t, metCharMetricName, call.name)

	// Value is duration.Seconds(): a float64 in seconds, so a 5ms handler must
	// land above 0.004 and comfortably below one second. This pins the UNIT —
	// a switch to milliseconds/nanoseconds would break it.
	require.Positive(t, call.value, "duration must be strictly positive")
	require.Greater(t, call.value, 0.004, "5ms handler must record >= ~0.005 seconds")
	require.Less(t, call.value, 1.0, "value must be seconds, not milliseconds/nanos")

	require.Equal(t, metCharWant("/users/{id}", http.MethodGet, "200"), call.labels,
		"exact ordered varargs label slice")

	// status is a string, never an int.
	require.Equal(t, "200", call.labels[5])
	require.IsType(t, "", call.labels[5])
}

// Test_MetricsContractNoOtherMetricsEmitted pins that the middleware emits the
// response histogram and nothing else — no counters, no gauges.
func Test_MetricsContractNoOtherMetricsEmitted(t *testing.T) {
	rec := &metCharRecorder{}
	handler := metCharChain(rec, metCharRegister("/ping"), metCharOKHandler())

	metCharServe(t, handler, http.MethodGet, "/ping")

	calls := rec.all()
	// Guard the loop: with no recorded calls the assertions below would not run
	// at all and the test would pass even if the middleware emitted nothing.
	require.Len(t, calls, 1, "exactly one metric is emitted per request")

	for _, c := range calls {
		require.Equal(t, metCharKindHistogram, c.kind, "only RecordHistogram may be called")
		require.Equal(t, metCharMetricName, c.name)
	}
}

// ---------------------------------------------------------------------------
// 2. Fast path (metricsAttrer) — exact attribute.KeyValue triple.
// ---------------------------------------------------------------------------

// Test_MetricsContractFastPathExactAttrs pins that when the metrics
// implementation also provides RecordHistogramAttrs, the middleware calls it
// (and never RecordHistogram) with exactly three attributes, in order:
// path(string), method(string), status(STRING — not Int).
func Test_MetricsContractFastPathExactAttrs(t *testing.T) {
	rec := metCharNewAttrRecorder()
	handler := metCharChain(rec, metCharRegister("/users/{id}"), metCharOKHandler())

	metCharServe(t, handler, http.MethodGet, "/users/42")

	call := rec.one(t)

	require.Equal(t, metCharKindAttrs, call.kind, "fast path must use RecordHistogramAttrs")
	require.Equal(t, metCharMetricName, call.name)
	require.GreaterOrEqual(t, call.value, 0.0)
	require.Less(t, call.value, 1.0, "value is seconds")

	require.Len(t, call.attrs, 3, "exactly three attributes")

	require.Equal(t, attribute.Key("path"), call.attrs[0].Key)
	require.Equal(t, attribute.STRING, call.attrs[0].Value.Type())
	require.Equal(t, "/users/{id}", call.attrs[0].Value.AsString())

	require.Equal(t, attribute.Key("method"), call.attrs[1].Key)
	require.Equal(t, attribute.STRING, call.attrs[1].Value.Type())
	require.Equal(t, http.MethodGet, call.attrs[1].Value.AsString())

	// The status attribute is deliberately a STRING, matching the slow path's
	// fmt.Sprintf("%d"). attribute.Int would change the OTLP wire type.
	require.Equal(t, attribute.Key("status"), call.attrs[2].Key)
	require.Equal(t, attribute.STRING, call.attrs[2].Value.Type(),
		"status must be attribute.String, NOT attribute.Int")
	require.Equal(t, "200", call.attrs[2].Value.AsString())

	// RecordHistogram must not be called at all on the fast path.
	for _, c := range rec.all() {
		require.NotEqual(t, metCharKindHistogram, c.kind,
			"RecordHistogram must not be used when RecordHistogramAttrs exists")
	}
}

// Test_MetricsContractFastAndSlowPathsAgree pins that both code paths produce
// semantically identical label sets for the same traffic.
func Test_MetricsContractFastAndSlowPathsAgree(t *testing.T) {
	type req struct {
		route  string
		method string
		target string
	}

	reqs := []req{
		{"/users/{id}", http.MethodGet, "/users/42"},
		{"/users/{id}", http.MethodPost, "/users/7"},
		{"/assets/{name}", http.MethodGet, "/assets/logo.png"},
		{"/", http.MethodGet, "/"},
		{"/static/{f}", http.MethodGet, "/static/app"},
	}

	for _, r := range reqs {
		t.Run(r.method+" "+r.target, func(t *testing.T) {
			slowRec := &metCharRecorder{}
			fastRec := metCharNewAttrRecorder()

			metCharServe(t, metCharChain(slowRec, metCharRegister(r.route), metCharOKHandler()), r.method, r.target)
			metCharServe(t, metCharChain(fastRec, metCharRegister(r.route), metCharOKHandler()), r.method, r.target)

			slow := slowRec.one(t)
			fast := fastRec.one(t)

			require.Equal(t, slow.name, fast.name)
			require.Equal(t, metCharLabelSet(t, slow), metCharLabelSet(t, fast),
				"fast and slow paths must produce identical labels")
		})
	}
}

// ---------------------------------------------------------------------------
// 3. Path label resolution.
// ---------------------------------------------------------------------------

// metCharPathCase is one path-label characterization case.
type metCharPathCase struct {
	name     string
	route    string // mux Path() template; empty means "no router at all".
	register func(r *mux.Router, h http.Handler)
	method   string
	target   string
	wantPath string
}

func metCharRunPathCases(t *testing.T, cases []metCharPathCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			register := tc.register
			if register == nil && tc.route != "" {
				register = metCharRegister(tc.route)
			}

			rec := &metCharRecorder{}
			handler := metCharChain(rec, register, metCharOKHandler())

			metCharServe(t, handler, tc.method, tc.target)

			call := rec.one(t)
			require.Equal(t, metCharMetricName, call.name)
			require.Equal(t, metCharWant(tc.wantPath, tc.method, "200"), call.labels)
		})
	}
}

// Test_MetricsContractPathLabelResolution pins how the path label is derived
// from the mux route template versus the raw r.URL.Path.
func Test_MetricsContractPathLabelResolution(t *testing.T) {
	metCharRunPathCases(t, []metCharPathCase{
		{
			name:     "route template wins over raw path",
			route:    "/users/{id}",
			method:   http.MethodGet,
			target:   "/users/42",
			wantPath: "/users/{id}",
		},
		{
			name:     "multi segment template",
			route:    "/orgs/{org}/repos/{repo}",
			method:   http.MethodGet,
			target:   "/orgs/gofr/repos/gofr",
			wantPath: "/orgs/{org}/repos/{repo}",
		},
		{
			name:     "no mux route at all falls back to raw path",
			method:   http.MethodGet,
			target:   "/no/such/route",
			wantPath: "/no/such/route",
		},
		{
			name:     "route without a Path matcher falls back to raw path",
			register: func(r *mux.Router, h http.Handler) { r.NewRoute().Methods(http.MethodGet).Handler(h) },
			method:   http.MethodGet,
			target:   "/anything/at/all",
			wantPath: "/anything/at/all",
		},
		{
			name:     "PathPrefix route collapses sub paths onto the prefix template",
			register: func(r *mux.Router, h http.Handler) { r.PathPrefix("/api/").Handler(h) },
			method:   http.MethodGet,
			target:   "/api/deeply/nested/thing",
			wantPath: "/api",
		},
	})
}

// Test_MetricsContractTrailingSlashTrimming pins strings.TrimSuffix(path, "/").
//
// LATENT BUG pinned here: a request to exactly "/" is first forced onto the
// raw path ("/") and then trimmed, producing an EMPTY STRING as the path
// label. That is almost certainly unintended (an empty `path` dimension in
// Prometheus/OTLP), but it is current behavior and is characterized, not
// fixed.
func Test_MetricsContractTrailingSlashTrimming(t *testing.T) {
	metCharRunPathCases(t, []metCharPathCase{
		{
			name:     "trailing slash trimmed from template",
			route:    "/users/",
			method:   http.MethodGet,
			target:   "/users/",
			wantPath: "/users",
		},
		{
			name:     "trailing slash trimmed from raw path when unrouted",
			method:   http.MethodGet,
			target:   "/raw/path/",
			wantPath: "/raw/path",
		},
		{
			name:     "root route yields an EMPTY path label (latent bug)",
			route:    "/",
			method:   http.MethodGet,
			target:   "/",
			wantPath: "",
		},
		{
			name:     "root request without a router also yields an EMPTY path label",
			method:   http.MethodGet,
			target:   "/",
			wantPath: "",
		},
	})
}

// Test_MetricsContractStaticPrefixForcesRawPath pins the `path == "/" ||
// strings.HasPrefix(path, "/static")` branch.
//
// Note the check is applied to the RESOLVED path (usually the route TEMPLATE),
// not to r.URL.Path — and "/staticfiles" also satisfies HasPrefix("/static"),
// which is a very likely unintended prefix match.
func Test_MetricsContractStaticPrefixForcesRawPath(t *testing.T) {
	metCharRunPathCases(t, []metCharPathCase{
		{
			name:     "template under /static forces the raw url path",
			route:    "/static/{file}",
			method:   http.MethodGet,
			target:   "/static/bundle",
			wantPath: "/static/bundle",
		},
		{
			name:     "template /staticfiles also matches HasPrefix /static (unintended)",
			route:    "/staticfiles/{id}",
			method:   http.MethodGet,
			target:   "/staticfiles/42",
			wantPath: "/staticfiles/42",
		},
		{
			name:     "template /statically also matches HasPrefix /static (unintended)",
			route:    "/statically/{id}",
			method:   http.MethodGet,
			target:   "/statically/9",
			wantPath: "/statically/9",
		},
		{
			name:     "url under /static but template elsewhere keeps the template",
			route:    "/{a}/{b}",
			method:   http.MethodGet,
			target:   "/static/thing",
			wantPath: "/{a}/{b}",
		},
		{
			name:     "template /staticky-ish sibling that does not match prefix keeps template",
			route:    "/stati/{id}",
			method:   http.MethodGet,
			target:   "/stati/1",
			wantPath: "/stati/{id}",
		},
	})
}

// ---------------------------------------------------------------------------
// 4. Static-file extension special case.
// ---------------------------------------------------------------------------

// metCharStaticExts is the exact extension allow-list in metrics.go.
func metCharStaticExts() []string {
	return []string{
		".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".ico",
		".svg", ".txt", ".html", ".json", ".woff", ".woff2", ".ttf", ".eot", ".pdf",
	}
}

// Test_MetricsContractStaticExtensionForcesRawPath pins that for every
// extension in the allow-list the RAW url path replaces the matched route
// template.
func Test_MetricsContractStaticExtensionForcesRawPath(t *testing.T) {
	for _, ext := range metCharStaticExts() {
		t.Run(ext, func(t *testing.T) {
			target := "/assets/logo" + ext
			rec := &metCharRecorder{}
			handler := metCharChain(rec, metCharRegister("/assets/{name}"), metCharOKHandler())

			metCharServe(t, handler, http.MethodGet, target)

			require.Equal(t, metCharWant(target, http.MethodGet, "200"), rec.one(t).labels)
		})
	}
}

// Test_MetricsContractStaticExtensionIsCaseInsensitive pins that the extension
// is lower-cased before the switch, so upper and mixed case also force the raw
// path.
func Test_MetricsContractStaticExtensionIsCaseInsensitive(t *testing.T) {
	targets := []string{
		"/assets/LOGO.PNG",
		"/assets/style.CSS",
		"/assets/data.JsOn",
		"/assets/font.WOFF2",
		"/assets/page.HtMl",
	}

	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			rec := &metCharRecorder{}
			handler := metCharChain(rec, metCharRegister("/assets/{name}"), metCharOKHandler())

			metCharServe(t, handler, http.MethodGet, target)

			require.Equal(t, metCharWant(target, http.MethodGet, "200"), rec.one(t).labels)
		})
	}
}

// Test_MetricsContractNonStaticExtensionKeepsTemplate pins the negative side of
// the allow-list: extensions that are NOT listed leave the route template in
// place, so cardinality stays bounded.
func Test_MetricsContractNonStaticExtensionKeepsTemplate(t *testing.T) {
	metCharRunPathCases(t, []metCharPathCase{
		{
			name:     "webp is not in the allow-list",
			route:    "/assets/{name}",
			method:   http.MethodGet,
			target:   "/assets/pic.webp",
			wantPath: "/assets/{name}",
		},
		{
			name:     "source map keeps template even though the name contains .js",
			route:    "/assets/{name}",
			method:   http.MethodGet,
			target:   "/assets/app.js.map",
			wantPath: "/assets/{name}",
		},
		{
			name:     "no extension at all keeps template",
			route:    "/assets/{name}",
			method:   http.MethodGet,
			target:   "/assets/logo",
			wantPath: "/assets/{name}",
		},
		{
			name:     "dot only in a directory segment is not an extension",
			route:    "/v1.0/{id}",
			method:   http.MethodGet,
			target:   "/v1.0/42",
			wantPath: "/v1.0/{id}",
		},
		{
			name:     "dotted directory plus static file still forces raw path",
			route:    "/v1.0/{name}",
			method:   http.MethodGet,
			target:   "/v1.0/logo.png",
			wantPath: "/v1.0/logo.png",
		},
	})
}

// Test_MetricsContractExtensionIgnoresQueryString pins that filepath.Ext is
// taken from r.URL.Path only — the query string neither contributes an
// extension nor leaks into the label.
func Test_MetricsContractExtensionIgnoresQueryString(t *testing.T) {
	metCharRunPathCases(t, []metCharPathCase{
		{
			name:     "static file with query string records the bare path",
			route:    "/assets/{name}",
			method:   http.MethodGet,
			target:   "/assets/logo.png?v=42",
			wantPath: "/assets/logo.png",
		},
		{
			name:     "css only in the query string does not force the raw path",
			route:    "/assets/{name}",
			method:   http.MethodGet,
			target:   "/assets/logo?fallback=a.css",
			wantPath: "/assets/{name}",
		},
	})
}

// ---------------------------------------------------------------------------
// 5. /graphql skip.
// ---------------------------------------------------------------------------

// Test_MetricsContractGraphQLRecordsNothing pins that when the RESOLVED path
// is exactly "/graphql" the middleware records nothing at all, while the inner
// handler still runs.
func Test_MetricsContractGraphQLRecordsNothing(t *testing.T) {
	cases := []struct {
		name   string
		route  string
		target string
	}{
		{"routed /graphql", "/graphql", "/graphql"},
		{"unrouted /graphql", "", "/graphql"},
		{"unrouted /graphql/ trailing slash is trimmed then skipped", "", "/graphql/"},
		{"route template /graphql/ is trimmed then skipped", "/graphql/", "/graphql/"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var register func(r *mux.Router, h http.Handler)
			if tc.route != "" {
				register = metCharRegister(tc.route)
			}

			var served bool

			inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				served = true

				w.WriteHeader(http.StatusOK)
			})

			rec := &metCharRecorder{}
			metCharServe(t, metCharChain(rec, register, inner), http.MethodGet, tc.target)

			require.True(t, served, "inner handler must still run for /graphql")
			require.Empty(t, rec.all(), "no metric may be recorded for /graphql")
		})
	}
}

// Test_MetricsContractGraphQLSkipIsPathBasedNotURLBased pins that the skip
// keys off the RESOLVED path label, not r.URL.Path: a wildcard route serving
// the /graphql url still records (with the template as the label), and a
// /graphql sub-path is not skipped.
func Test_MetricsContractGraphQLSkipIsPathBasedNotURLBased(t *testing.T) {
	metCharRunPathCases(t, []metCharPathCase{
		{
			name:     "wildcard route serving /graphql is still recorded",
			route:    "/{resource}",
			method:   http.MethodPost,
			target:   "/graphql",
			wantPath: "/{resource}",
		},
		{
			name:     "graphql sub path is not skipped",
			route:    "/graphql/playground",
			method:   http.MethodGet,
			target:   "/graphql/playground",
			wantPath: "/graphql/playground",
		},
		{
			name:     "graphqlx is not skipped",
			method:   http.MethodGet,
			target:   "/graphqlx",
			wantPath: "/graphqlx",
		},
	})
}

// ---------------------------------------------------------------------------
// 6. Status label.
// ---------------------------------------------------------------------------

// Test_MetricsContractStatusLabel pins the status label across explicit
// WriteHeader, implicit 200 via Write, and a handler that does nothing at all
// (StatusResponseWriter.Status normalizes 0 to 200 — the label is "200", never
// "0").
func Test_MetricsContractStatusLabel(t *testing.T) {
	cases := []struct {
		name       string
		handler    http.Handler
		wantStatus string
	}{
		{
			name: "explicit 200",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
			wantStatus: "200",
		},
		{
			name: "explicit 404",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}),
			wantStatus: "404",
		},
		{
			name: "explicit 500",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}),
			wantStatus: "500",
		},
		{
			name: "body only write implies 200",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte("hello"))
			}),
			wantStatus: "200",
		},
		{
			name:       "handler writes nothing at all normalizes 0 to 200",
			handler:    http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {}),
			wantStatus: "200",
		},
		{
			name: "first WriteHeader wins over a later one",
			handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTeapot)
				w.WriteHeader(http.StatusInternalServerError)
			}),
			wantStatus: "418",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			slowRec := &metCharRecorder{}
			fastRec := metCharNewAttrRecorder()

			metCharServe(t, metCharChain(slowRec, metCharRegister("/s"), tc.handler), http.MethodGet, "/s")
			metCharServe(t, metCharChain(fastRec, metCharRegister("/s"), tc.handler), http.MethodGet, "/s")

			want := metCharWant("/s", http.MethodGet, tc.wantStatus)

			require.Equal(t, want, slowRec.one(t).labels)
			require.Equal(t, want, metCharLabelSet(t, fastRec.one(t)))
		})
	}
}

// ---------------------------------------------------------------------------
// 7. Method label.
// ---------------------------------------------------------------------------

// Test_MetricsContractMethodLabelIsRawRequestMethod pins that the method label
// is r.Method verbatim — it is NOT upper-cased (unlike the tracer middleware),
// so a lowercase "get" is recorded as "get".
func Test_MetricsContractMethodLabelIsRawRequestMethod(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodPost, http.MethodDelete, "get", "PaTcH", "CUSTOMVERB"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			rec := &metCharRecorder{}
			// No router: mux would reject the odd verbs before the middleware runs.
			handler := metCharChain(rec, nil, metCharOKHandler())

			metCharServe(t, handler, method, "/verb")

			call := rec.one(t)
			require.Equal(t, metCharWant("/verb", method, "200"), call.labels)
			require.Equal(t, method, call.labels[3], "method label is verbatim r.Method")
		})
	}
}

// ---------------------------------------------------------------------------
// 8. StatusResponseWriter wrapping / reuse.
// ---------------------------------------------------------------------------

// Test_MetricsContractResponseWriterReuse pins the wrapping contract.
//
//   - When the incoming ResponseWriter is ALREADY a *StatusResponseWriter
//     (Logging ran first), Metrics reuses it — no double wrapping, and the
//     inner handler sees the very same pointer.
//   - Otherwise Metrics allocates one. Note that the code does NOT reassign
//     the local `w` — but it passes `srw` to inner.ServeHTTP, so the inner
//     handler DOES receive the wrapper (and Unwrap() returns the original
//     writer). This test pins that, since the non-reassignment reads like a
//     bug at a glance.
func Test_MetricsContractResponseWriterReuse(t *testing.T) {
	t.Run("already wrapped is reused not double wrapped", func(t *testing.T) {
		rec := &metCharRecorder{}
		rr := httptest.NewRecorder()
		outer := &StatusResponseWriter{ResponseWriter: rr}

		var seen http.ResponseWriter

		inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			seen = w

			w.WriteHeader(http.StatusAccepted)
		})

		handler := metCharChain(rec, nil, inner)
		handler.ServeHTTP(outer, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/w", http.NoBody))

		require.Same(t, outer, seen, "existing *StatusResponseWriter must be reused")
		require.Equal(t, http.StatusAccepted, outer.Status())
		require.Equal(t, metCharWant("/w", http.MethodGet, "202"), rec.one(t).labels)
	})

	t.Run("plain writer is wrapped and the wrapper reaches the inner handler", func(t *testing.T) {
		rec := &metCharRecorder{}
		rr := httptest.NewRecorder()

		var seen http.ResponseWriter

		inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			seen = w

			w.WriteHeader(http.StatusCreated)
		})

		handler := metCharChain(rec, nil, inner)
		handler.ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/w", http.NoBody))

		srw, ok := seen.(*StatusResponseWriter)
		require.True(t, ok, "inner handler receives the newly created *StatusResponseWriter")
		require.NotSame(t, http.ResponseWriter(rr), seen)
		require.Same(t, rr, srw.Unwrap(), "wrapper unwraps to the original writer")
		require.Equal(t, http.StatusCreated, rr.Code, "status still reaches the real writer")
		require.Equal(t, metCharWant("/w", http.MethodGet, "201"), rec.one(t).labels)
	})

	t.Run("graphql skip path also passes the wrapper through", func(t *testing.T) {
		rec := &metCharRecorder{}
		rr := httptest.NewRecorder()

		var seen http.ResponseWriter

		inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			seen = w

			w.WriteHeader(http.StatusOK)
		})

		handler := metCharChain(rec, nil, inner)
		handler.ServeHTTP(rr, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql", http.NoBody))

		require.IsType(t, &StatusResponseWriter{}, seen)
		require.Empty(t, rec.all())
	})
}

// ---------------------------------------------------------------------------
// 9. Caching of route/status attributes.
// ---------------------------------------------------------------------------

// Test_MetricsContractAttributeCacheKeying pins that the per-instance
// routeAttrs/statusAttrs caches are keyed correctly: repeated requests produce
// identical labels, and different methods on the SAME path produce different,
// correct labels (a path-only cache key would leak the first method).
func Test_MetricsContractAttributeCacheKeying(t *testing.T) {
	rec := metCharNewAttrRecorder()

	handler := metCharChain(rec, func(r *mux.Router, h http.Handler) {
		r.Handle("/users/{id}", h)
		r.Handle("/orders/{id}", h)
	}, metCharOKHandler())

	metCharServe(t, handler, http.MethodGet, "/users/1")
	metCharServe(t, handler, http.MethodGet, "/users/2")
	metCharServe(t, handler, http.MethodPost, "/users/3")
	metCharServe(t, handler, http.MethodGet, "/orders/9")
	metCharServe(t, handler, http.MethodPost, "/users/4")

	calls := rec.all()
	require.Len(t, calls, 5)

	want := [][]string{
		metCharWant("/users/{id}", http.MethodGet, "200"),
		metCharWant("/users/{id}", http.MethodGet, "200"),
		metCharWant("/users/{id}", http.MethodPost, "200"),
		metCharWant("/orders/{id}", http.MethodGet, "200"),
		metCharWant("/users/{id}", http.MethodPost, "200"),
	}

	for i := range calls {
		require.Equal(t, want[i], metCharLabelSet(t, &calls[i]), "call %d", i)
	}
}

// Test_MetricsContractStatusCacheKeying pins that the status attribute cache
// returns the right value per status code, including after a repeat.
func Test_MetricsContractStatusCacheKeying(t *testing.T) {
	rec := metCharNewAttrRecorder()

	codes := []int{http.StatusOK, http.StatusNotFound, http.StatusOK, http.StatusInternalServerError, http.StatusNotFound}

	handler := metCharChain(rec, metCharRegister("/c/{code}"),
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			code, _ := strconv.Atoi(mux.Vars(r)["code"])

			w.WriteHeader(code)
		}))

	for _, code := range codes {
		metCharServe(t, handler, http.MethodGet, "/c/"+strconv.Itoa(code))
	}

	calls := rec.all()
	require.Len(t, calls, len(codes))

	for i := range calls {
		require.Equal(t, metCharWant("/c/{code}", http.MethodGet, strconv.Itoa(codes[i])),
			metCharLabelSet(t, &calls[i]))
	}
}

// ---------------------------------------------------------------------------
// 10. Concurrency.
// ---------------------------------------------------------------------------

// Test_MetricsContractConcurrentRequests fires many concurrent requests through
// a SINGLE Metrics instance across several routes, methods and statuses and
// asserts the recorded multiset of label sets is exactly what is expected. Run
// under -race this also covers the sync.Map caches.
func Test_MetricsContractConcurrentRequests(t *testing.T) {
	const iterations = 40

	rec := metCharNewAttrRecorder()

	handler := metCharChain(rec, func(r *mux.Router, h http.Handler) {
		r.Handle("/users/{id}", h)
		r.Handle("/orders/{id}", h)
		r.Handle("/assets/{name}", h)
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("fail") == "1" {
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))

	type reqSpec struct {
		method string
		target string
		want   []string
	}

	specs := []reqSpec{
		{http.MethodGet, "/users/1", metCharWant("/users/{id}", http.MethodGet, "200")},
		{http.MethodPost, "/users/2", metCharWant("/users/{id}", http.MethodPost, "200")},
		{http.MethodGet, "/orders/3?fail=1", metCharWant("/orders/{id}", http.MethodGet, "500")},
		{http.MethodGet, "/assets/logo.png", metCharWant("/assets/logo.png", http.MethodGet, "200")},
	}

	var wg sync.WaitGroup

	for range iterations {
		for _, spec := range specs {
			wg.Add(1)

			go func(spec reqSpec) {
				defer wg.Done()

				rr := httptest.NewRecorder()
				req := httptest.NewRequestWithContext(t.Context(), spec.method, spec.target, http.NoBody)

				handler.ServeHTTP(rr, req)
			}(spec)
		}
	}

	wg.Wait()

	got := make(map[string]int, len(specs))

	concurrentCalls := rec.all()

	for i := range concurrentCalls {
		require.Equal(t, metCharMetricName, concurrentCalls[i].name)
		require.Equal(t, metCharKindAttrs, concurrentCalls[i].kind)

		got[strings.Join(metCharLabelSet(t, &concurrentCalls[i]), "|")]++
	}

	want := make(map[string]int, len(specs))
	for _, spec := range specs {
		want[strings.Join(spec.want, "|")] = iterations
	}

	require.Equal(t, want, got, "exact multiset of recorded label sets")
}

// Test_MetricsContractConcurrentSlowPath repeats the concurrency check for an
// implementation that only satisfies `metrics`, so the slow varargs path is
// exercised under -race as well.
func Test_MetricsContractConcurrentSlowPath(t *testing.T) {
	const iterations = 30

	rec := &metCharRecorder{}
	handler := metCharChain(rec, metCharRegister("/users/{id}"), metCharOKHandler())

	var wg sync.WaitGroup

	for range iterations {
		wg.Add(1)

		go func() {
			defer wg.Done()

			metCharServe(t, handler, http.MethodGet, "/users/1")
		}()
	}

	wg.Wait()

	calls := rec.all()
	require.Len(t, calls, iterations)

	for _, c := range calls {
		require.Equal(t, metCharWant("/users/{id}", http.MethodGet, "200"), c.labels)
	}
}
