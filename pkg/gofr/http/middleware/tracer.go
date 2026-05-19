package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/version"
)

// methodAttr is a small static lookup for the http.method attribute key.
// Building attribute.String("http.method", "GET") per request allocates a
// KeyValue with a copy of the string header — pre-allocating the common
// methods saves that on every request.
//
//nolint:gochecknoglobals // immutable lookup table for hot-path attribute reuse
var methodAttr = map[string]attribute.KeyValue{
	http.MethodGet:     attribute.String("http.method", http.MethodGet),
	http.MethodPost:    attribute.String("http.method", http.MethodPost),
	http.MethodPut:     attribute.String("http.method", http.MethodPut),
	http.MethodDelete:  attribute.String("http.method", http.MethodDelete),
	http.MethodPatch:   attribute.String("http.method", http.MethodPatch),
	http.MethodHead:    attribute.String("http.method", http.MethodHead),
	http.MethodOptions: attribute.String("http.method", http.MethodOptions),
}

// methodKV returns the precomputed KeyValue for known HTTP methods, falling
// back to allocation for non-standard ones (rare; RFC 7231 lists only the
// standard set).
func methodKV(method string) attribute.KeyValue {
	if kv, ok := methodAttr[method]; ok {
		return kv
	}

	return attribute.String("http.method", method)
}

// routeTemplate returns the matched route template (e.g. "/users/{id}") for
// the request when gorilla/mux has resolved one, otherwise the raw URL path.
// Used for the span name and http.route attribute so tracing cardinality
// stays bounded by route count, not request count.
func routeTemplate(r *http.Request) string {
	if route := mux.CurrentRoute(r); route != nil {
		if t, err := route.GetPathTemplate(); err == nil && t != "" {
			return t
		}
	}

	return r.URL.Path
}

// Tracer is a middleware that starts a new OpenTelemetry trace span for each
// request and records the http.method, http.route and http.status_code
// attributes on it.
//
// The tracer is resolved once at chain-build time (after App.New has installed
// the real provider via initTracer) and captured in the per-request closure —
// otel.GetTracerProvider().Tracer(name) is a mutex-guarded map lookup under
// the SDK provider, so resolving once saves that lookup on every request.
//
// HTTP semconv attributes are passed via trace.WithAttributes at span Start
// so the SDK can size its internal attribute slice exactly once instead of
// growing it. http.status_code is set after the handler returns via the
// StatusResponseWriter wrap shared with the Logging middleware.
func Tracer(inner http.Handler) http.Handler {
	tr := otel.Tracer("gofr-" + version.Framework)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// extract the traceID and spanID from the headers and create a new context for the same
		// this context will make a new span using the traceID and link the incoming SpanID as
		// its parentID, thus connecting two spans
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))

		method := strings.ToUpper(r.Method)
		// Prefer the gorilla/mux route template (e.g. "/users/{id}") so the
		// span name and http.route attribute do not explode to one unique
		// value per concrete path (e.g. "/users/42"). Fall back to URL.Path
		// when no route matched (404 / unknown route).
		route := routeTemplate(r)
		spanName := fmt.Sprintf("%s %s", method, route)

		ctxOut, span := tr.Start(ctx, spanName, trace.WithAttributes(
			methodKV(method),
			attribute.String("http.route", route),
		))
		defer span.End()

		// Use the StatusResponseWriter wrap (provided by the Logging
		// middleware) to capture the response status; type assert on the
		// way out. If we are not after Logging in the chain — uncommon —
		// fall back to wrapping locally.
		srw, ok := w.(*StatusResponseWriter)
		if !ok {
			srw = &StatusResponseWriter{ResponseWriter: w}
			w = srw
		}

		defer func(s trace.Span, rw *StatusResponseWriter) {
			if rw.status != 0 {
				s.SetAttributes(attribute.Int("http.status_code", rw.status))
			}
		}(span, srw)

		inner.ServeHTTP(w, r.WithContext(ctxOut))
	})
}
