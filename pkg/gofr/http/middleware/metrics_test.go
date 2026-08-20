package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"sync"
	"testing"

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
