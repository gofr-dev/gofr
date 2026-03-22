package mqtt

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestHeaderCarrier_GetSetKeys(t *testing.T) {
	carrier := make(headerCarrier)

	// Test Set
	carrier.Set("traceparent", "00-1234567890abcdef-fedcba0987654321-01")
	carrier.Set("tracestate", "foo=bar")

	// Test Get
	assert.Equal(t, "00-1234567890abcdef-fedcba0987654321-01", carrier.Get("traceparent"))
	assert.Equal(t, "foo=bar", carrier.Get("tracestate"))
	assert.Empty(t, carrier.Get("nonexistent"))

	// Test Keys
	keys := carrier.Keys()
	assert.Contains(t, keys, "traceparent")
	assert.Contains(t, keys, "tracestate")

	// Test Set updates existing key
	carrier.Set("traceparent", "00-updated-value")
	assert.Equal(t, "00-updated-value", carrier.Get("traceparent"))
}

func TestInjectTraceContext(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	attrs := injectTraceContext(ctx, nil)

	require.NotNil(t, attrs)

	traceparent, ok := attrs["traceparent"]
	require.True(t, ok, "traceparent should be injected")
	assert.Contains(t, traceparent, span.SpanContext().TraceID().String())
}

func TestExtractTraceLinks(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	ctx, producerSpan := tp.Tracer("test").Start(context.Background(), "producer-span")
	attrs := injectTraceContext(ctx, nil)

	producerSpan.End()

	links := extractTraceLinks(attrs)

	require.Len(t, links, 1, "should have one link")
	assert.Equal(t, producerSpan.SpanContext().TraceID(), links[0].SpanContext.TraceID())
	assert.Equal(t, producerSpan.SpanContext().SpanID(), links[0].SpanContext.SpanID())
}

func TestExtractTraceLinks_NoHeaders(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	assert.Nil(t, extractTraceLinks(nil), "should return nil for nil headers")
}

func TestStartPublishSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	ctx, span, headers := startPublishSpan(context.Background(), "test-topic")
	defer span.End()

	require.NotNil(t, span)
	assert.True(t, span.SpanContext().IsValid())
	require.NotNil(t, ctx)

	_, hasTraceparent := headers["traceparent"]
	assert.True(t, hasTraceparent, "headers should contain traceparent")
}

func TestStartSubscribeSpan_WithLinks(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	_, producerSpan, headers := startPublishSpan(context.Background(), "test-topic")
	producerSpan.End()

	_, subscribeSpan := startSubscribeSpan(context.Background(), "test-topic", headers)
	subscribeSpan.End()

	spans := exporter.GetSpans()
	require.GreaterOrEqual(t, len(spans), 2)

	var subSpan *tracetest.SpanStub

	for i := range spans {
		if spans[i].Name == "mqtt-subscribe" {
			subSpan = &spans[i]
			break
		}
	}

	require.NotNil(t, subSpan, "subscribe span should exist")
	require.Len(t, subSpan.Links, 1, "subscribe span should have one link")
	assert.Equal(t, producerSpan.SpanContext().TraceID(), subSpan.Links[0].SpanContext.TraceID())
	assert.Equal(t, producerSpan.SpanContext().SpanID(), subSpan.Links[0].SpanContext.SpanID())
}

func TestStartSubscribeSpan_NoLinks(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	_, subscribeSpan := startSubscribeSpan(context.Background(), "test-topic", nil)
	subscribeSpan.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	assert.Empty(t, spans[0].Links, "orphan span should have no links")
}

func TestWrapPayload(t *testing.T) {
	headers := map[string]string{
		"traceparent": "00-abc123-def456-01",
		"tracestate":  "foo=bar",
	}

	original := []byte("hello world")

	wrapped := wrapPayload(headers, original)

	var env traceEnvelope

	err := json.Unmarshal(wrapped, &env)
	require.NoError(t, err)
	assert.Equal(t, "1", env.Marker)
	assert.Equal(t, "00-abc123-def456-01", env.Traceparent)
	assert.Equal(t, "foo=bar", env.Tracestate)

	decoded, err := base64.StdEncoding.DecodeString(env.Data)
	require.NoError(t, err)
	assert.Equal(t, original, decoded)
}

func TestUnwrapPayload(t *testing.T) {
	headers := map[string]string{
		"traceparent": "00-abc123-def456-01",
		"tracestate":  "foo=bar",
	}

	original := []byte("hello world")

	wrapped := wrapPayload(headers, original)

	extractedHeaders, payload := unwrapPayload(wrapped)

	assert.Equal(t, original, payload)
	assert.Equal(t, "00-abc123-def456-01", extractedHeaders["traceparent"])
	assert.Equal(t, "foo=bar", extractedHeaders["tracestate"])
}

func TestUnwrapPayload_NonWrapped(t *testing.T) {
	// Plain text
	data := []byte("plain text message")
	headers, payload := unwrapPayload(data)
	assert.Nil(t, headers)
	assert.Equal(t, data, payload)

	// JSON without marker
	jsonData := []byte(`{"key":"value"}`)
	headers, payload = unwrapPayload(jsonData)
	assert.Nil(t, headers)
	assert.JSONEq(t, string(jsonData), string(payload))
}

func TestWrapUnwrap_RoundTrip(t *testing.T) {
	headers := map[string]string{
		"traceparent": "00-abc-def-01",
	}

	original := []byte("test payload with special chars: !@#$%^&*()")

	wrapped := wrapPayload(headers, original)

	_, payload := unwrapPayload(wrapped)

	assert.Equal(t, original, payload)
}
