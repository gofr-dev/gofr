package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"gofr.dev/pkg/gofr/version"
)

func Tracer(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start context and Tracing
		ctx := r.Context()

		// extract the traceID and spanID from the headers and create a new context for the same
		// this context will make a new span using the traceID and link the incoming SpanID as
		// its parentID, thus connecting two spans
		ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))

		tr := otel.GetTracerProvider().Tracer("gofr-" + version.Framework)
		ctx, span := tr.Start(ctx, fmt.Sprintf("%s %s", strings.ToUpper(r.Method), r.URL.Path))
		defer span.End()

		inner.ServeHTTP(w, r.WithContext(ctx))
	})
}
