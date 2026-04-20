package kafka

import (
	"context"
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestHeaderCarrier_GetSetKeys(t *testing.T) {
	carrier := headerCarrier{}

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
	// Setup tracer with W3C propagator
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	// Create a span
	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	// Inject trace context into headers
	headers := injectTraceContext(ctx, nil)

	// Verify traceparent header was injected
	var traceparent string

	for _, h := range headers {
		if h.Key == "traceparent" {
			traceparent = string(h.Value)
			break
		}
	}

	require.NotEmpty(t, traceparent, "traceparent header should be injected")
	assert.Contains(t, traceparent, span.SpanContext().TraceID().String())
}

func TestExtractTraceLinks(t *testing.T) {
	// Setup tracer with W3C propagator
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	// Create a producer span and inject context
	ctx, producerSpan := tp.Tracer("test").Start(context.Background(), "producer-span")
	headers := injectTraceContext(ctx, nil)

	producerSpan.End()

	// Extract links from headers
	links := extractTraceLinks(headers)

	// Verify link to producer span
	require.Len(t, links, 1, "should have one link")
	assert.Equal(t, producerSpan.SpanContext().TraceID(), links[0].SpanContext.TraceID())
	assert.Equal(t, producerSpan.SpanContext().SpanID(), links[0].SpanContext.SpanID())
}

func TestExtractTraceLinks_NoHeaders(t *testing.T) {
	// Setup tracer
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	// Extract links from empty headers
	links := extractTraceLinks(nil)

	// Should return nil (orphan span)
	assert.Nil(t, links, "should return nil for empty headers")
}

func TestStartPublishSpan(t *testing.T) {
	// Setup tracer
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	// Start publish span
	ctx, span, headers := startPublishSpan(context.Background(), "test-topic")
	defer span.End()

	// Verify span created
	require.NotNil(t, span)
	assert.True(t, span.SpanContext().IsValid())

	// Verify context updated
	require.NotNil(t, ctx)

	// Verify headers contain trace context
	var hasTraceparent bool

	for _, h := range headers {
		if h.Key == "traceparent" {
			hasTraceparent = true
			break
		}
	}

	assert.True(t, hasTraceparent, "headers should contain traceparent")
}

func TestStartSubscribeSpan_WithLinks(t *testing.T) {
	// Setup tracer
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	// Create producer span and get headers
	_, producerSpan, headers := startPublishSpan(context.Background(), "test-topic")
	producerSpan.End()

	// Start subscribe span with headers
	_, subscribeSpan := startSubscribeSpan(context.Background(), "test-topic", headers)
	subscribeSpan.End()

	// Get recorded spans
	spans := exporter.GetSpans()
	require.GreaterOrEqual(t, len(spans), 2)

	// Find subscribe span and verify links
	var subSpan *tracetest.SpanStub

	for i := range spans {
		if spans[i].Name == "kafka-subscribe" {
			subSpan = &spans[i]
			break
		}
	}

	require.NotNil(t, subSpan, "subscribe span should exist")
	require.Len(t, subSpan.Links, 1, "subscribe span should have one link")
	assert.Equal(t, producerSpan.SpanContext().TraceID(), subSpan.Links[0].SpanContext.TraceID())
	assert.Equal(t, producerSpan.SpanContext().SpanID(), subSpan.Links[0].SpanContext.SpanID())

	// Subscribe span must also be a CHILD of the producer span: same trace ID
	// and parent span ID matches the producer's span ID. This is what makes a
	// single end-to-end trace visible in tracing UIs.
	assert.Equal(t, producerSpan.SpanContext().TraceID(), subSpan.SpanContext.TraceID(),
		"subscribe span should share the producer's trace ID")
	assert.Equal(t, producerSpan.SpanContext().SpanID(), subSpan.Parent.SpanID(),
		"subscribe span's parent should be the producer span")
}

func TestStartSubscribeSpan_NoLinks(t *testing.T) {
	// Setup tracer
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	// Start subscribe span without headers (orphan span)
	_, subscribeSpan := startSubscribeSpan(context.Background(), "test-topic", nil)
	subscribeSpan.End()

	// Get recorded spans
	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	// Verify no links
	assert.Empty(t, spans[0].Links, "orphan span should have no links")
}

func TestHeaderCarrier_ConvertFromKafkaHeaders(t *testing.T) {
	// Test conversion from actual kafka.Header slice
	kafkaHeaders := []kafka.Header{
		{Key: "traceparent", Value: []byte("00-trace-span-01")},
		{Key: "custom-header", Value: []byte("custom-value")},
	}

	carrier := headerCarrier(kafkaHeaders)

	assert.Equal(t, "00-trace-span-01", carrier.Get("traceparent"))
	assert.Equal(t, "custom-value", carrier.Get("custom-header"))
}
