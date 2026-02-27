package google

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestAttributeCarrier_GetSetKeys(t *testing.T) {
	carrier := make(attributeCarrier)

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
	require.True(t, ok, "traceparent attribute should be injected")
	assert.Contains(t, traceparent, span.SpanContext().TraceID().String())
}

func TestInjectTraceContext_PreservesExistingAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	existing := map[string]string{
		"custom-attr": "custom-value",
	}

	attrs := injectTraceContext(ctx, existing)

	assert.Equal(t, "custom-value", attrs["custom-attr"])

	_, ok := attrs["traceparent"]
	assert.True(t, ok, "traceparent should be injected alongside existing attributes")
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

func TestExtractTraceLinks_NoAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	links := extractTraceLinks(nil)
	assert.Nil(t, links, "should return nil for nil attributes")
}

func TestExtractTraceLinks_EmptyAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	links := extractTraceLinks(make(map[string]string))
	assert.Nil(t, links, "should return nil for empty attributes")
}

func TestStartPublishSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	ctx, span, attrs := startPublishSpan(context.Background(), "test-topic")
	defer span.End()

	require.NotNil(t, span)
	assert.True(t, span.SpanContext().IsValid())
	require.NotNil(t, ctx)

	_, hasTraceparent := attrs["traceparent"]
	assert.True(t, hasTraceparent, "attributes should contain traceparent")
}

func TestStartSubscribeSpan_WithLinks(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	_, producerSpan, attrs := startPublishSpan(context.Background(), "test-topic")
	producerSpan.End()

	_, subscribeSpan := startSubscribeSpan(context.Background(), "test-topic", attrs)
	subscribeSpan.End()

	spans := exporter.GetSpans()
	require.GreaterOrEqual(t, len(spans), 2)

	var subSpan *tracetest.SpanStub

	for i := range spans {
		if spans[i].Name == "gcp-subscribe" {
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
