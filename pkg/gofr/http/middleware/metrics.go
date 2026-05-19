package middleware

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel/attribute"
)

type metrics interface {
	IncrementCounter(ctx context.Context, name string, labels ...string)
	DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	SetGauge(name string, value float64, labels ...string)
}

// metricsAttrer is the optional fast-path interface — when the concrete
// metrics implementation provides RecordHistogramAttrs, the middleware
// uses pre-built attribute slices instead of the string varargs path,
// avoiding the per-request attribute conversion in
// metricsManager.getAttributes. Not part of the public metrics.Manager
// interface, so external implementers are unaffected — they fall back to
// RecordHistogram.
type metricsAttrer interface {
	RecordHistogramAttrs(ctx context.Context, name string, value float64, attrs ...attribute.KeyValue)
}

// routeMethodKey identifies a (path-template, method) pair for caching
// the precomputed attribute slice for app_http_response.
type routeMethodKey struct {
	path, method string
}

// Metrics is a middleware that records request response time metrics using the provided metrics interface.
func Metrics(metrics metrics) func(inner http.Handler) http.Handler {
	// Per-middleware-instance caches (closure-owned, not package globals):
	//   * routeAttrs maps (path, method) → [path-kv, method-kv]. The slice is
	//     built once per unique route-method combination and reused for every
	//     subsequent request. Bounded by routes × methods (typically <100
	//     entries even for large APIs).
	//   * statusAttrs maps int → attribute.KeyValue for the http status code.
	//     Bounded by ~20 distinct status codes seen in practice.
	//   * attrer is the type-asserted fast-path receiver, captured once if
	//     available.
	var (
		routeAttrs  sync.Map // map[routeMethodKey][]attribute.KeyValue
		statusAttrs sync.Map // map[int]attribute.KeyValue
	)

	statusAttr := func(code int) attribute.KeyValue {
		if v, ok := statusAttrs.Load(code); ok {
			return v.(attribute.KeyValue)
		}

		kv := attribute.Int("status", code)
		statusAttrs.Store(code, kv)

		return kv
	}

	attrer, hasAttrer := metrics.(metricsAttrer)

	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If an outer middleware (Logging) has already wrapped w in a
			// StatusResponseWriter, reuse it instead of double-wrapping. Both
			// middlewares only read status — a single wrapper captures it for
			// both, saving one allocation per request.
			srw, ok := w.(*StatusResponseWriter)
			if !ok {
				srw = &StatusResponseWriter{ResponseWriter: w}
			}

			// Guard against nil — mux.CurrentRoute is nil for unmatched routes
			// (404). Calling .GetPathTemplate() on nil panics, so fall back to
			// URL.Path for those cases.
			var path string
			if cr := mux.CurrentRoute(r); cr != nil {
				path, _ = cr.GetPathTemplate()
			}

			ext := strings.ToLower(filepath.Ext(r.URL.Path))
			switch ext {
			case ".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".txt", ".html", ".json", ".woff", ".woff2", ".ttf", ".eot", ".pdf":
				path = r.URL.Path
			}

			if path == "/" || strings.HasPrefix(path, "/static") {
				path = r.URL.Path
			}

			path = strings.TrimSuffix(path, "/")

			// Skip recording for /graphql — it has its own dedicated metrics
			// (app_graphql_*). time.Now() (vDSO call) is deferred past this
			// branch so /graphql does not pay for a timestamp we throw away.
			if path == "/graphql" {
				inner.ServeHTTP(srw, r)
				return
			}

			start := time.Now()

			// this has to be called in the end so that status code is populated
			defer func(res *StatusResponseWriter, req *http.Request) {
				duration := time.Since(start)

				if hasAttrer {
					// Fast path: copy the cached (path, method) attribute pair
					// into a fixed 3-element local array and add the
					// per-request status KV in slot 2. Avoids the per-request
					// append-and-grow that occurs when growing a cap=2 slice
					// to length 3.
					key := routeMethodKey{path: path, method: req.Method}

					base, ok := routeAttrs.Load(key)
					if !ok {
						b := []attribute.KeyValue{
							attribute.String("path", path),
							attribute.String("method", req.Method),
						}
						base, _ = routeAttrs.LoadOrStore(key, b)
					}

					b := base.([]attribute.KeyValue)
					attrs := [3]attribute.KeyValue{b[0], b[1], statusAttr(res.status)}
					attrer.RecordHistogramAttrs(context.Background(), "app_http_response", duration.Seconds(), attrs[:]...)

					return
				}

				// Slow path: external metrics implementation, no fast method.
				metrics.RecordHistogram(context.Background(), "app_http_response", duration.Seconds(),
					"path", path, "method", req.Method, "status", fmt.Sprintf("%d", res.status))
			}(srw, r)

			inner.ServeHTTP(srw, r)
		})
	}
}
