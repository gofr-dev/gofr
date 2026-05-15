package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"gofr.dev/pkg/gofr/version"
)

// Tracer is a middleware that  starts a new OpenTelemetry trace span for each request.
//
// The tracer is resolved once at chain-build time (after App.New has installed
// the real provider via initTracer) and captured in the per-request closure —
// otel.GetTracerProvider().Tracer(name) is a mutex-guarded map lookup under
// the SDK provider, so resolving once saves that lookup on every request.
func Tracer(inner http.Handler) http.Handler {
	tr := otel.Tracer("gofr-" + version.Framework)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start context and Tracing
		ctx := r.Context()

		// extract the traceID and spanID from the headers and create a new context for the same
		// this context will make a new span using the traceID and link the incoming SpanID as
		// its parentID, thus connecting two spans
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))

		ctx, span := tr.Start(ctx, fmt.Sprintf("%s %s", strings.ToUpper(r.Method), r.URL.Path))

		defer span.End()

		inner.ServeHTTP(w, r.WithContext(ctx))
	})
}
