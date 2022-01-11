package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
)

func Tracer(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Start context and Tracing
		ctx := r.Context()

		// TODO - version has to be injected
		tr := otel.GetTracerProvider().Tracer("gofr")
		ctx, span := tr.Start(ctx, fmt.Sprintf("%s %s", strings.ToUpper(r.Method), r.URL.Path))
		defer span.End()

		inner.ServeHTTP(w, r.WithContext(ctx))
	})
}
