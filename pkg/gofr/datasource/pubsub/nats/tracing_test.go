package nats

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// setupOTel installs a real in-memory tracer provider and TraceContext propagator,
// returning the exporter so callers can inspect recorded spans.
// All globals are restored when t finishes.
func setupOTel(t *testing.T) (*tracetest.InMemoryExporter, *sdktrace.TracerProvider) {
	t.Helper()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())

		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})

	return exporter, tp
}

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
	_, tp := setupOTel(t)

	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	headers := injectTraceContext(ctx, nil)

	require.NotNil(t, headers)

	traceparent := headers.Get("traceparent")
	require.NotEmpty(t, traceparent, "traceparent header should be injected")
	assert.Contains(t, traceparent, span.SpanContext().TraceID().String())
}

func TestInjectTraceContext_PreservesExistingHeaders(t *testing.T) {
	_, tp := setupOTel(t)

	ctx, span := tp.Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	existing := nats.Header{}
	existing.Set("custom-header", "custom-value")

	headers := injectTraceContext(ctx, existing)

	assert.Equal(t, "custom-value", headers.Get("custom-header"))

	traceparent := headers.Get("traceparent")
	assert.NotEmpty(t, traceparent, "traceparent should be injected alongside existing headers")
}

func TestExtractTraceLinks(t *testing.T) {
	_, tp := setupOTel(t)

	ctx, producerSpan := tp.Tracer("test").Start(context.Background(), "producer-span")
	headers := injectTraceContext(ctx, nil)

	producerSpan.End()

	links := extractTraceLinks(headers)

	require.Len(t, links, 1, "should have one link")
	assert.Equal(t, producerSpan.SpanContext().TraceID(), links[0].SpanContext.TraceID())
	assert.Equal(t, producerSpan.SpanContext().SpanID(), links[0].SpanContext.SpanID())
}

func TestExtractTraceLinks_NoHeaders(t *testing.T) {
	setupOTel(t)

	links := extractTraceLinks(nil)
	assert.Nil(t, links, "should return nil for nil headers")
}

func TestExtractTraceLinks_EmptyHeaders(t *testing.T) {
	setupOTel(t)

	links := extractTraceLinks(make(nats.Header))
	assert.Nil(t, links, "should return nil for empty headers")
}

func TestStartPublishSpan(t *testing.T) {
	_, tp := setupOTel(t)

	tracer := tp.Tracer(tracerName)

	ctx, span, headers := startPublishSpan(context.Background(), tracer, "test-subject")
	defer span.End()

	require.NotNil(t, span)
	assert.True(t, span.SpanContext().IsValid())
	require.NotNil(t, ctx)

	traceparent := headers.Get("traceparent")
	assert.NotEmpty(t, traceparent, "headers should contain traceparent")
}

func TestStartSubscribeSpan_WithLinks(t *testing.T) {
	exporter, tp := setupOTel(t)

	tracer := tp.Tracer(tracerName)

	_, producerSpan, headers := startPublishSpan(context.Background(), tracer, "test-subject")
	producerSpan.End()

	_, subscribeSpan := startSubscribeSpan(context.Background(), tracer, "test-subject", headers)
	subscribeSpan.End()

	spans := exporter.GetSpans()
	require.GreaterOrEqual(t, len(spans), 2)

	var subSpan *tracetest.SpanStub

	for i := range spans {
		if spans[i].Name == "nats-subscribe" {
			subSpan = &spans[i]
			break
		}
	}

	require.NotNil(t, subSpan, "subscribe span should exist")
	require.Len(t, subSpan.Links, 1, "subscribe span should have one link")
	assert.Equal(t, producerSpan.SpanContext().TraceID(), subSpan.Links[0].SpanContext.TraceID())
	assert.Equal(t, producerSpan.SpanContext().SpanID(), subSpan.Links[0].SpanContext.SpanID())

	// Subscribe span must also be a CHILD of the producer span: same trace ID
	// and parent span ID matches the producer's span ID.
	assert.Equal(t, producerSpan.SpanContext().TraceID(), subSpan.SpanContext.TraceID(),
		"subscribe span should share the producer's trace ID")
	assert.Equal(t, producerSpan.SpanContext().SpanID(), subSpan.Parent.SpanID(),
		"subscribe span's parent should be the producer span")
}

func TestStartSubscribeSpan_NoLinks(t *testing.T) {
	exporter, tp := setupOTel(t)

	tracer := tp.Tracer(tracerName)

	_, subscribeSpan := startSubscribeSpan(context.Background(), tracer, "test-subject", nil)
	subscribeSpan.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	assert.Empty(t, spans[0].Links, "orphan span should have no links")
}
