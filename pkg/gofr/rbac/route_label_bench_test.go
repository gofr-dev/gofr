package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/internal/routemark"
)

// BenchmarkMiddleware_Unmatched404 measures a request that only GoFr's catch-all answers, with RBAC
// on, under both routers. The catch-all check runs for every such request, so its cost shows here.
func BenchmarkMiddleware_Unmatched404(b *testing.B) {
	for _, router := range []string{gofrHTTP.MatcherMux, gofrHTTP.MatcherTrie} {
		b.Run(router, func(b *testing.B) {
			b.Setenv(gofrHTTP.RouterEnvVar, router)

			config := newTestConfig(b, []EndpointMapping{
				{Path: "/api/{path:.*}", Methods: []string{http.MethodGet}, Public: true},
			}, nil)
			config.Tracer = noop.NewTracerProvider().Tracer("bench")

			notFound := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })

			r := gofrHTTP.NewRouter()
			r.Use(Middleware(config))
			r.Add(http.MethodGet, "/api/users/{id}", notFound)
			routemark.MarkCatchAll(r.PathPrefix("/")).Handler(notFound)

			req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/nowhere", http.NoBody)
			rec := httptest.NewRecorder()

			b.ReportAllocs()
			b.ResetTimer()

			for b.Loop() {
				r.ServeHTTP(rec, req)
			}
		})
	}
}

// BenchmarkCatchAllAnswered measures the lookup the span label does on every request.
func BenchmarkCatchAllAnswered(b *testing.B) {
	r := gofrHTTP.NewRouter()
	ok := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

	var seen *http.Request

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			seen = req
			next.ServeHTTP(w, req)
		})
	})
	routemark.MarkCatchAll(r.PathPrefix("/")).Handler(ok)
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/x", http.NoBody))

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = routemark.CatchAllAnswered(seen)
	}
}
