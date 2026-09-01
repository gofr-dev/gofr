package middleware

import (
	"context"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	gofrhttp "gofr.dev/pkg/gofr/http"
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

// Metrics is a middleware that records request response time metrics using the provided metrics interface.
func Metrics(metrics metrics) func(inner http.Handler) http.Handler {
	// The recording strategy is selected once, here, from what the metrics
	// backend supports. The per-request path then makes a single call and
	// carries none of that branching itself.
	recorder := newHistogramRecorder(metrics)

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

			path, templated := metricsPath(r)

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
			// histogram nor any status cache is poisoned with status=0.
			defer func(res *StatusResponseWriter, req *http.Request) {
				recorder.record(path, req.Method, res.Status(), time.Since(start).Seconds(), templated)
			}(srw, r)

			inner.ServeHTTP(srw, r)
		})
	}
}

// metricsPath resolves the path label and reports whether it is a bounded route
// template.
//
// The label itself is unchanged. What is new is the second return value, which
// decides whether the result may be used as a cache key. Every branch that falls
// back to r.URL.Path yields a caller-controlled string: an unmatched route, a
// static-asset extension, /static, and - crucially - the "/" template that
// GoFr's PathPrefix("/") catch-all produces for every unmatched request. Without
// this distinction a stream of unique unmatched paths filled the shared cache to
// its ceiling, after which every first-seen LEGITIMATE route could never be
// stored and rebuilt its measurement option per request forever. Memory stayed
// bounded; the optimization silently reverted to baseline for real traffic.
func metricsPath(r *http.Request) (path string, templated bool) {
	path = gofrhttp.RouteTemplate(r)
	templated = path != ""

	// It is "" for unmatched routes and for routes built without an explicit
	// Path() (e.g. PathPrefix-only handlers), so fall back to r.URL.Path there
	// to keep a usable path label rather than an empty key.
	if !templated {
		path = r.URL.Path
	}

	ext := strings.ToLower(filepath.Ext(r.URL.Path))
	switch ext {
	case ".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg", ".txt", ".html", ".json", ".woff", ".woff2", ".ttf", ".eot", ".pdf":
		path, templated = r.URL.Path, false
	}

	if path == "/" || strings.HasPrefix(path, "/static") {
		path, templated = r.URL.Path, false
	}

	return strings.TrimSuffix(path, "/"), templated
}

// histogramName is the request-duration histogram this middleware records.
const histogramName = "app_http_response"

// metricsOptRecorder is the fastest optional interface: an implementation that
// accepts an already-built measurement option lets this middleware build each
// (path, method, status) option once instead of on every request.
type metricsOptRecorder interface {
	RecordHistogramOpt(ctx context.Context, name string, value float64, opts ...metric.RecordOption)
}

// statusRouteKey identifies a (path, method, status) triple.
type statusRouteKey struct {
	path, method string
	status       int
}

// histogramRecorder records one app_http_response observation.
//
// Three implementations exist because a metrics backend may support three
// levels of pre-building. Choosing between them once, at construction, keeps
// that decision out of the request path and keeps each strategy independently
// readable and testable.
type histogramRecorder interface {
	// templated reports whether path is a bounded route template. Only a
	// templated path may be used as a cache key -- see metricsPath.
	record(path, method string, status int, seconds float64, templated bool)
}

// newHistogramRecorder selects the most capable strategy the backend supports.
func newHistogramRecorder(m metrics) histogramRecorder {
	statusAttr := newStatusAttrCache()

	if r, ok := m.(metricsOptRecorder); ok {
		return &optionRecorder{rec: r, statusAttr: statusAttr}
	}

	if a, ok := m.(metricsAttrer); ok {
		return &attrsRecorder{rec: a, statusAttr: statusAttr}
	}

	return &labelsRecorder{rec: m}
}

// newStatusAttrCache returns a memoized status-attribute builder.
//
// The value is a string, not an Int: metricsManager's varargs path emits status
// as a string and OTLP exporters distinguish KeyValue types, so a mismatch would
// break user queries expecting a string across the two code paths.
func newStatusAttrCache() func(int) attribute.KeyValue {
	var cache sync.Map // map[int]attribute.KeyValue

	return func(code int) attribute.KeyValue {
		if v, ok := cache.Load(code); ok {
			kv, _ := v.(attribute.KeyValue)

			return kv
		}

		kv := attribute.String("status", strconv.Itoa(code))
		cache.Store(code, kv)

		return kv
	}
}

// optionRecorder caches one fully built measurement option per
// (path, method, status), so metric.WithAttributes -- which builds and sorts an
// attribute.Set -- runs once per combination rather than once per request.
type optionRecorder struct {
	rec        metricsOptRecorder
	statusAttr func(int) attribute.KeyValue
	cache      routeCache // statusRouteKey -> []metric.RecordOption
}

func (o *optionRecorder) record(path, method string, status int, seconds float64, templated bool) {
	cacheable := templated && cacheableMethod(method)
	key := statusRouteKey{path: path, method: method, status: status}

	if cacheable {
		// The assertion's ok is honored rather than discarded: a failed one would
		// leave opts nil and record the measurement with NO attributes, which is a
		// silently unlabeled time series rather than a visible failure. Falling
		// through rebuilds instead.
		if v, ok := o.cache.load(key); ok {
			if opts, ok := v.([]metric.RecordOption); ok {
				o.rec.RecordHistogramOpt(context.Background(), histogramName, seconds, opts...)

				return
			}
		}
	}

	// Cache the option slice, not the option: passing a lone option into the
	// variadic would allocate a one-element slice on every request.
	//
	// The status attribute is a String, not an Int: metricsManager's varargs
	// path emits status as a string and OTLP exporters distinguish KeyValue
	// types, so the two paths must agree or a dashboard breaks depending on
	// which recorder the backend selected.
	opts := []metric.RecordOption{metric.WithAttributes(
		attribute.String("path", path),
		attribute.String("method", method),
		o.statusAttr(status),
	)}

	if cacheable {
		o.cache.store(key, opts)
	}

	o.rec.RecordHistogramOpt(context.Background(), histogramName, seconds, opts...)
}

// attrsRecorder caches the (path, method) attribute pair and copies it into a
// fixed three-element array, avoiding the append-and-grow that appending status
// to a cap=2 slice would cost.
type attrsRecorder struct {
	rec        metricsAttrer
	statusAttr func(int) attribute.KeyValue
	cache      routeCache // routeMethodKey -> []attribute.KeyValue
}

func (a *attrsRecorder) record(path, method string, status int, seconds float64, templated bool) {
	key := routeMethodKey{path: path, method: method}

	var b []attribute.KeyValue

	// This cache used to have no ceiling at all while its sibling above had one,
	// which is exactly the inconsistency that makes a shared key space dangerous:
	// one malicious request stream inflates whichever cache is unbounded. Both
	// now go through routeCache under the same rules.
	cacheable := templated && cacheableMethod(method)
	if cacheable {
		if v, ok := a.cache.load(key); ok {
			b, _ = v.([]attribute.KeyValue)
		}
	}

	if b == nil {
		b = []attribute.KeyValue{
			attribute.String("path", path),
			attribute.String("method", method),
		}

		if cacheable {
			a.cache.store(key, b)
		}
	}

	attrs := [3]attribute.KeyValue{b[0], b[1], a.statusAttr(status)}

	a.rec.RecordHistogramAttrs(context.Background(), histogramName, seconds, attrs[:]...)
}

// labelsRecorder is the fallback for an external metrics implementation that
// offers neither fast-path interface.
type labelsRecorder struct{ rec metrics }

func (l *labelsRecorder) record(path, method string, status int, seconds float64, _ bool) {
	l.rec.RecordHistogram(context.Background(), histogramName, seconds,
		"path", path, "method", method, "status", strconv.Itoa(status))
}
