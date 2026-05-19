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

// methodKV constructs the http.request.method attribute (OTel HTTP
// semconv ≥ v1.21 stable). Previously this used the v1.4 "http.method"
// attribute key — that key is now deprecated in upstream semconv and
// downstream dashboards built against current semconv versions miss the
// attribute if we keep emitting the old key.
//
// Allocation note: the returned KeyValue + the underlying string are
// hoisted into the variadic ...attribute.KeyValue slice passed to
// trace.WithAttributes below, which the compiler escape-analyzes onto
// the heap (verified via go build -gcflags='-m=2'). Factoring the
// constructor into its own function does not avoid the alloc — it is
// still useful as a single source of truth for the attribute key.
func methodKV(method string) attribute.KeyValue {
	return attribute.String("http.request.method", method)
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
// request and records http.request.method, http.route, and
// http.response.status_code attributes on it, following the current OTel
// HTTP semantic conventions (≥ v1.21).
//
// Behavioral change vs prior versions: GoFr used to wrap routes in
// otelhttp.NewHandler("gofr-router") which produced spans with the static
// name "gofr-router". Spans are now named "METHOD /route-template" (e.g.
// "GET /users/{id}") per the OTel HTTP semconv span-name guidance. Users
// with dashboards or alerts filtering on span.name == "gofr-router" must
// update their filters.
//
// The tracer is resolved once at chain-build time (after App.New has installed
// the real provider via initTracer; see factory.go) and captured in the
// per-request closure — otel.GetTracerProvider().Tracer(name) is a mutex-
// guarded map lookup under the SDK provider, so resolving once saves that
// lookup on every request.
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

		// http.response.status_code is set after the handler returns via
		// the StatusResponseWriter wrap shared with Logging.

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
				s.SetAttributes(attribute.Int("http.response.status_code", rw.status))
			}
		}(span, srw)

		inner.ServeHTTP(w, r.WithContext(ctxOut))
	})
}
