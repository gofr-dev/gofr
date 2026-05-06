package sqs

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
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

func TestAttributeCarrier_GetNilStringValue(t *testing.T) {
	carrier := attributeCarrier{
		"key-no-string": {
			DataType: aws.String("String"),
			// StringValue is nil
		},
	}

	assert.Empty(t, carrier.Get("key-no-string"))
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

	traceparentAttr, ok := attrs["traceparent"]
	require.True(t, ok, "traceparent attribute should be injected")
	require.NotNil(t, traceparentAttr.StringValue)
	assert.Contains(t, *traceparentAttr.StringValue, span.SpanContext().TraceID().String())
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

	existing := map[string]types.MessageAttributeValue{
		"custom-attr": {
			DataType:    aws.String("String"),
			StringValue: aws.String("custom-value"),
		},
	}

	attrs := injectTraceContext(ctx, existing)

	// Existing attribute should still be present
	assert.Equal(t, "custom-value", *attrs["custom-attr"].StringValue)

	// Traceparent should also be present
	_, ok := attrs["traceparent"]
	assert.True(t, ok, "traceparent should be injected alongside existing attributes")
}

func TestStartPublishSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	ctx, span, attrs := startPublishSpan(context.Background(), "test-queue")
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

	// Create producer span and get attributes
	_, producerSpan, attrs := startPublishSpan(context.Background(), "test-queue")
	producerSpan.End()

	// Start subscribe span with attributes
	_, subscribeSpan := startSubscribeSpan(context.Background(), "test-queue", attrs)
	subscribeSpan.End()

	// Get recorded spans
	spans := exporter.GetSpans()
	require.GreaterOrEqual(t, len(spans), 2)

	// Find subscribe span and verify links
	var subSpan *tracetest.SpanStub

	for i := range spans {
		if spans[i].Name == "sqs-subscribe" {
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

	// Subscribe span must inherit the producer's sampling decision via ParentBased
	// — without this, head-based sampling (TRACER_RATIO) would drop halves of a trace.
	assert.Equal(t, producerSpan.SpanContext().TraceFlags(), subSpan.SpanContext.TraceFlags(),
		"subscribe span should inherit the producer's trace flags")
}

func TestStartSubscribeSpan_NoLinks(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	_, subscribeSpan := startSubscribeSpan(context.Background(), "test-queue", nil)
	subscribeSpan.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	assert.Empty(t, spans[0].Links, "orphan span should have no links")
}

func TestStartSubscribeSpan_InvalidTraceparent(t *testing.T) {
	// Non-empty attrs with a malformed traceparent must not produce a parent
	// or a link — the code falls back to an orphan span.
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	attrs := map[string]types.MessageAttributeValue{
		"traceparent": {
			DataType:    aws.String("String"),
			StringValue: aws.String("not-a-valid-traceparent"),
		},
	}

	_, subscribeSpan := startSubscribeSpan(context.Background(), "test-queue", attrs)
	subscribeSpan.End()

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Empty(t, spans[0].Links, "invalid traceparent should produce no link")
	assert.False(t, spans[0].Parent.IsValid(), "invalid traceparent should produce no parent")
}
