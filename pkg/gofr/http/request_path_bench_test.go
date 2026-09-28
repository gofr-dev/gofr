package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

// BenchmarkRequestPath measures a complete request through the router and a
// realistic middleware chain, rather than the matcher alone.
//
// The matcher-only benchmark next door answers "how fast is route lookup"; this
// answers "what does a request cost", which is the number a service actually
// experiences and the one an optimization has to move. Every middleware here is
// a stand-in of the same SHAPE as the real chain -- one that wraps the writer,
// one that adds a header, one that reads the route template from the context --
// so the per-request work being measured is the framework's plumbing rather than
// any particular middleware's body.
//
// It is deliberately in package http and free of the container, so it can be run
// on its own and gives a stable, fast signal: go test -bench RequestPath -benchmem.
func BenchmarkRequestPath(b *testing.B) {
	for _, matcher := range []string{MatcherMux, MatcherTrie} {
		for _, routes := range []int{10, 100, 1000} {
			for _, shape := range []struct {
				name    string
				pattern string
				request string
			}{
				{"static", "/bench/target", "/bench/target"},
				{"param", "/bench/target/{id}", "/bench/target/42"},
			} {
				name := fmt.Sprintf("%s/%s/routes=%d", matcher, shape.name, routes)

				b.Run(name, func(b *testing.B) {
					b.Setenv(RouterEnvVar, matcher)

					r := buildBenchRouter(routes, shape.pattern)
					req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, shape.request, http.NoBody)
					w := &nopWriter{header: make(http.Header, 4)}

					// One request outside the timer so the lazy index build and any
					// first-use caches are warm; a benchmark that measured them would
					// be measuring startup.
					r.ServeHTTP(w, req)

					b.ReportAllocs()
					b.ResetTimer()

					for range b.N {
						r.ServeHTTP(w, req)
					}
				})
			}
		}
	}
}

// buildBenchRouter registers total routes with the benchmarked one registered
// LAST -- mux's worst case, and the position that makes route-table size visible.
func buildBenchRouter(total int, pattern string) *Router {
	r := NewRouter()

	r.Use(writerWrappingMW, headerSettingMW, routeReadingMW)

	// Registered through Add, which is what a GoFr application uses -- app.GET and
	// friends reach the router this way. Routes created straight on the embedded
	// mux.Router are deliberately excluded from the middleware-chain cache, since
	// their handler is not necessarily the route's own, so registering them here
	// would benchmark a path no real app takes.
	for i := range total - 1 {
		r.Add(http.MethodGet, fmt.Sprintf("/api/v1/resource%d/{id}/action", i), benchHandler())
	}

	r.Add(http.MethodGet, pattern, benchHandler())

	return r
}

func benchHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Touch the vars the way a real handler does, so a change in how they are
		// carried is visible here.
		_ = mux.Vars(r)["id"]

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":"ok"}`))
	})
}

// writerWrappingMW stands in for the logging middleware: it wraps the writer so
// the status can be read afterwards.
func writerWrappingMW(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := &benchStatusWriter{ResponseWriter: w}
		inner.ServeHTTP(sw, r)

		_ = sw.status
	})
}

// headerSettingMW stands in for the correlation-ID and CORS middleware.
func headerSettingMW(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header()["X-Bench-Id"] = benchHeaderValue
		inner.ServeHTTP(w, r)
	})
}

// routeReadingMW stands in for the tracer and metrics middleware, both of which
// read the matched route template out of the request on every request.
func routeReadingMW(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = RouteTemplate(r)
		inner.ServeHTTP(w, r)
	})
}

//nolint:gochecknoglobals // immutable benchmark fixture.
var benchHeaderValue = []string{"bench"}

type benchStatusWriter struct {
	http.ResponseWriter

	status int
}

func (w *benchStatusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// nopWriter is a ResponseWriter that costs nothing, so the benchmark measures
// the framework rather than httptest.ResponseRecorder's buffer growth.
type nopWriter struct {
	header http.Header
}

func (w *nopWriter) Header() http.Header       { return w.header }
func (*nopWriter) Write(b []byte) (int, error) { return len(b), nil }
func (*nopWriter) WriteHeader(int)             {}
