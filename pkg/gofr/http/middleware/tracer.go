package middleware

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/trace"
)

func Tracer(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start context and Tracing
		ctx := r.Context()
		// TODO - version has to be injected
		ctx, span := global.TracerProvider().Tracer("gofr",
			trace.WithInstrumentationVersion("v0.1")).Start(ctx, fmt.Sprintf("gofr-middleware %s %s", r.Method, r.URL.Path))
		defer span.End()

		inner.ServeHTTP(w, r.WithContext(ctx))
	})
}
