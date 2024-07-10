package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type MockHandlerForTracing struct{}

// ServeHTTP is used for testing if the request context has traceId.
func (*MockHandlerForTracing) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	traceID := otelTrace.SpanFromContext(req.Context()).SpanContext().TraceID().String()
	_, _ = w.Write([]byte(traceID))
}

func TestTrace(_ *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	handler := Tracer(&MockHandlerForTracing{})
	req := httptest.NewRequest(http.MethodGet, "/dummy", http.NoBody)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)
}
