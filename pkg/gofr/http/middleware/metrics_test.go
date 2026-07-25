package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

// TestMetrics_MatchedRouteWithStaticExtension guards that a registered route
// whose URL ends in a static-looking extension keeps its route template as the
// path label (e.g. /openapi.json stays /openapi.json) rather than being
// reclassified by extension.
func TestMetrics_MatchedRouteWithStaticExtension(t *testing.T) {
	mockMetrics := &mockMetrics{}

	mockMetrics.On("RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/openapi.json", "method", "GET", "status", "200"}).Return(nil)

	router := mux.NewRouter()
	router.HandleFunc("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/openapi.json")

	router.Use(Metrics(mockMetrics))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/openapi.json", http.NoBody)
	router.ServeHTTP(httptest.NewRecorder(), req)

	mockMetrics.AssertCalled(t, "RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/openapi.json", "method", "GET", "status", "200"})
}

// TestMetrics_RootRouteLabel guards that the root route "/" keeps a "/" path
// label rather than being trimmed to an empty string.
func TestMetrics_RootRouteLabel(t *testing.T) {
	mockMetrics := &mockMetrics{}

	mockMetrics.On("RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/", "method", "GET", "status", "200"}).Return(nil)

	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/")

	router.Use(Metrics(mockMetrics))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	router.ServeHTTP(httptest.NewRecorder(), req)

	mockMetrics.AssertCalled(t, "RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/", "method", "GET", "status", "200"})
}

func TestMetrics_StaticFile(t *testing.T) {
	mockMetrics := &mockMetrics{}

	mockMetrics.On("RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/static", "method", "GET", "status", "200"}).Return(nil)

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
		[]string{"path", "/static", "method", "GET", "status", "200"})
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

// TestMetrics_NoTemplateFallsBackToRoot asserts the empty-template guard: when
// no route resolves (mux.CurrentRoute is nil) the path label is "/" rather than
// the raw URL. Raw URLs would make the label unbounded (one series per invented
// URL); "/" keeps it bounded. (In a full GoFr app the catch-all PathPrefix("/")
// already supplies "/" — see TestMetrics_GoFrRouterTopology — this guards the
// same bound for any router without that catch-all.)
func TestMetrics_NoTemplateFallsBackToRoot(t *testing.T) {
	mockMetrics := &mockMetrics{}

	mockMetrics.On("RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/", "method", "GET", "status", "404"}).Return(nil)

	notFound := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	handler := Metrics(mockMetrics)(notFound)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/no/such/route", http.NoBody)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	mockMetrics.AssertCalled(t, "RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/", "method", "GET", "status", "404"})
}

// TestMetrics_GoFrRouterTopology exercises the real GoFr router topology — a
// specific route, the static-files PathPrefix("/static/"), and the catch-all
// PathPrefix("/") that GoFr registers (gofr.go) — and asserts the path label is
// always a bounded route template, never the raw URL:
//   - specific route        -> its template (/customer/{id})
//   - static asset           -> "/static" (the static prefix template)
//   - static-looking 404     -> "/" (catch-all), NOT a per-file series
//   - arbitrary invented 404 -> "/" (catch-all)
func TestMetrics_GoFrRouterTopology(t *testing.T) {
	pathFor := func(t *testing.T, target string) string {
		t.Helper()

		var got string

		m := &mockMetrics{}
		m.On("RecordHistogram", mock.Anything, "app_http_response", mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				labels := args.Get(3).([]string)
				for i := 0; i+1 < len(labels); i += 2 {
					if labels[i] == "path" {
						got = labels[i+1]
					}
				}
			}).Return(nil)

		router := mux.NewRouter()
		router.HandleFunc("/customer/{id}", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodGet)
		router.PathPrefix("/static/").Handler(
			http.StripPrefix("/static/", http.FileServer(http.Dir(t.TempDir())))).Name("/static/")
		// GoFr's unconditional catch-all (gofr.go).
		router.PathPrefix("/").Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		router.Use(Metrics(m))

		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody))

		return got
	}

	tests := []struct {
		desc   string
		target string
		want   string
	}{
		{"specific route", "/customer/42", "/customer/{id}"},
		{"static asset", "/static/app.js", "/static"},
		{"static-looking 404", "/images/missing.png", "/"},
		{"invented 404", "/totally/unknown", "/"},
	}

	for i, tc := range tests {
		require.Equalf(t, tc.want, pathFor(t, tc.target), "TEST[%d] %s", i, tc.desc)
	}
}

func TestMetrics_StaticFileWithQueryParam(t *testing.T) {
	mockMetrics := &mockMetrics{}

	mockMetrics.On("RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/static", "method", "GET", "status", "200"}).Return(nil)

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
		[]string{"path", "/static", "method", "GET", "status", "200"})
}
