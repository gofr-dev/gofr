package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/trace"
	otelTrace "go.opentelemetry.io/otel/trace"
)

type MockHandlerForTracing struct{}

// ServeHTTP is used for testing if the request context has traceId
func (r *MockHandlerForTracing) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	traceID := otelTrace.SpanFromContext(req.Context()).SpanContext().TraceID().String()
	_, _ = w.Write([]byte(traceID))
}

func TestTrace(t *testing.T) {
	url := "http://localhost:2005/api/v2/spans"
	exporter, _ := zipkin.New(url)
	batcher := trace.NewBatchSpanProcessor(exporter)

	tp := trace.NewTracerProvider(trace.WithSampler(trace.AlwaysSample()), trace.WithSpanProcessor(batcher))

	otel.SetTracerProvider(tp)

	req := httptest.NewRequest("GET", "/dummy", nil)
	req = req.WithContext(context.WithValue(context.Background(), "path", ""))
	req.Header.Set("X-Correlation-ID", "123e4567e89b12d3a456426655440000")
	//req = req.WithContext(context.WithValue(context.Background(), "X-Correlation-ID", "123e4567-e89b-12d3-a456-426655440000"))
	handler := Trace("Gofr-App", "dev", "zipkin")(&MockHandlerForTracing{})

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	traceID := recorder.Body.String()

	if traceID == "" {
		t.Errorf("Failed to get traceId")
	}

	// if tracing has failed then the traceId is usually '00000000000000000000000000000000'
	// which is not an empty string, hence conversion to int is required to check if tracing id is correct.
	id, err := strconv.Atoi(traceID)

	if err == nil && id == 0 {
		t.Errorf("Incorrect tracingId")
	}
}

func TestGetTraceID(t *testing.T) {
	tests := []struct {
		desc          string
		traceID       string
		correlationID string
		output        string
	}{
		{"get trace ID", "12334", "", "12334"},
		{"get correlation ID", "", "12345", "12345"},
	}

	for i, v := range tests {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("X-B3-TraceId", v.traceID)
		req.Header.Set("X-Correlation-ID", v.correlationID)
		res := getTraceID(req)
		assert.Equal(t, res, v.output, "Test case [%v] Failed.", i)
	}
}
