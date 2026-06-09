package middleware

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
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

// graphqlPath is the canonical GraphQL endpoint that the Metrics
// middleware skips — GraphQL has its own dedicated app_graphql_*
// metrics, so recording app_http_response for it would double-count.
const graphqlPath = "/graphql"

// staticAssetPath and unmatchedPath are bounded sentinel values for the
// app_http_response "path" label. Requests that don't resolve to a registered
// route template would otherwise carry their raw URL as the label — unbounded
// for static files (one series per filename) and for unmatched 404s (one per
// URL a client invents) — inflating cardinality on this single instrument and
// the routeAttrs cache. Collapsing them to fixed labels keeps app_http_response
// bounded regardless of traffic.
const (
	staticAssetPath = "/static/*"
	unmatchedPath   = "/*"
)

// isStaticAsset reports whether urlPath is a static-asset request whose raw
// path should be collapsed to staticAssetPath.
func isStaticAsset(urlPath string) bool {
	if strings.HasPrefix(urlPath, "/static") {
		return true
	}

	switch strings.ToLower(filepath.Ext(urlPath)) {
	case ".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg",
		".txt", ".html", ".json", ".woff", ".woff2", ".ttf", ".eot", ".pdf":
		return true
	default:
		return false
	}
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

	// Cache the status attribute with a string value (not Int) so the
	// fast path matches the slow path's varargs label type.
	// metricsManager.getAttributes emits status as a string for the slow
	// path, and OTLP exporters distinguish KeyValue types — a mismatch
	// would break user queries expecting the label to be a string across
	// both code paths.
	statusAttr := func(code int) attribute.KeyValue {
		if v, ok := statusAttrs.Load(code); ok {
			return v.(attribute.KeyValue)
		}

		kv := attribute.String("status", strconv.Itoa(code))
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

			// Resolve the "path" label. Prefer the registered route template
			// (bounded by the number of routes). mux.CurrentRoute is nil for
			// unmatched routes (404), and GetPathTemplate returns "" for routes
			// without an explicit Path() (e.g. PathPrefix-only handlers). Rather
			// than fall back to the raw URL — which is unbounded and explodes
			// cardinality — collapse static assets and unmatched requests to
			// bounded sentinels.
			var path string
			if cr := mux.CurrentRoute(r); cr != nil {
				path, _ = cr.GetPathTemplate()
			}

			switch {
			case isStaticAsset(r.URL.Path):
				path = staticAssetPath
			case path == "":
				path = unmatchedPath
			default:
				path = strings.TrimSuffix(path, "/")
			}

			// Skip recording for /graphql — it has its own dedicated metrics
			// (app_graphql_*). time.Now() (vDSO call) is deferred past this
			// branch so /graphql does not pay for a timestamp we throw away.
			if path == graphqlPath {
				inner.ServeHTTP(srw, r)
				return
			}

			start := time.Now()

			// this has to be called in the end so that status code is populated.
			// res.Status() normalizes a zero internal status (handler wrote
			// nothing, net/http implicit-200) to http.StatusOK so neither the
			// histogram nor the statusAttrs cache gets poisoned with status=0.
			defer func(res *StatusResponseWriter, req *http.Request) {
				duration := time.Since(start)
				status := res.Status()

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
					attrs := [3]attribute.KeyValue{b[0], b[1], statusAttr(status)}
					attrer.RecordHistogramAttrs(context.Background(), "app_http_response", duration.Seconds(), attrs[:]...)

					return
				}

				// Slow path: external metrics implementation, no fast method.
				metrics.RecordHistogram(context.Background(), "app_http_response", duration.Seconds(),
					"path", path, "method", req.Method, "status", fmt.Sprintf("%d", status))
			}(srw, r)

			inner.ServeHTTP(srw, r)
		})
	}
}
