package middleware

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// Trace is a middleware which starts a span and the newly added context can be propagated and used for tracing
func Trace(appName, appVersion, tracerExporter string) func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			cID, err := trace.TraceIDFromHex(getTraceID(r))
			if err == nil {
				ctx = trace.ContextWithSpanContext(ctx, trace.SpanContextFromContext(r.Context()).WithTraceID(cID))
			}

			ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))

			tracer := otel.Tracer("gofr", trace.WithInstrumentationVersion(appVersion))

			path := ctx.Value("path")
			if path == "" {
				path = r.URL.Path
			}

			ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s %s", appName, r.Method, path),
				trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(semconv.ServiceNameKey.String(appName),
					semconv.TelemetrySDKNameKey.String(tracerExporter)))

			defer func() {
				//nolint // cannot create custom type as it will result in import cycle
				*r = *r.Clone(context.WithValue(r.Context(), "path", ""))

				span.End()
			}()

			inner.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// getTraceID is used to fetch the correlationID from request header.
func getTraceID(r *http.Request) string {
	if id := r.Header.Get("X-B3-TraceId"); id != "" {
		return id
	}

	return r.Header.Get("X-Correlation-ID")
}
