package mqtt

import (
	"context"
	"testing"

	"github.com/eclipse/paho.golang/paho"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func setupTestTracer(t *testing.T) *sdktrace.TracerProvider {
	t.Helper()

	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
	})

	return tp
}

func Test_userPropertyCarrier_GetSetKeys(t *testing.T) {
	props := paho.UserProperties{
		{Key: "key1", Value: "value1"},
		{Key: "key2", Value: "value2"},
	}

	carrier := &userPropertyCarrier{props: &props}

	// Test Get
	assert.Equal(t, "value1", carrier.Get("key1"))
	assert.Equal(t, "value2", carrier.Get("key2"))
	assert.Equal(t, "", carrier.Get("nonexistent"))

	// Test Keys
	keys := carrier.Keys()
	assert.ElementsMatch(t, []string{"key1", "key2"}, keys)

	// Test Set - update existing
	carrier.Set("key1", "updated")
	assert.Equal(t, "updated", carrier.Get("key1"))

	// Test Set - add new
	carrier.Set("key3", "value3")
	assert.Equal(t, "value3", carrier.Get("key3"))
	assert.Len(t, *carrier.props, 3)
}

func Test_userPropertyCarrier_NilProps(t *testing.T) {
	carrier := &userPropertyCarrier{props: nil}

	assert.Equal(t, "", carrier.Get("key"))
	assert.Nil(t, carrier.Keys())

	// Set on nil should not panic
	carrier.Set("key", "value")
}

func Test_injectTraceContext(t *testing.T) {
	setupTestTracer(t)

	ctx, span := otel.GetTracerProvider().Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	props := injectTraceContext(ctx, nil)

	// Should have injected traceparent
	carrier := &userPropertyCarrier{props: &props}
	traceparent := carrier.Get("traceparent")
	assert.NotEmpty(t, traceparent, "traceparent should be injected into User Properties")
}

func Test_injectTraceContext_WithExistingProps(t *testing.T) {
	setupTestTracer(t)

	ctx, span := otel.GetTracerProvider().Tracer("test").Start(context.Background(), "test-span")
	defer span.End()

	existingProps := paho.UserProperties{
		{Key: "custom-key", Value: "custom-value"},
	}

	props := injectTraceContext(ctx, existingProps)

	carrier := &userPropertyCarrier{props: &props}

	// Original property should be preserved
	assert.Equal(t, "custom-value", carrier.Get("custom-key"))

	// traceparent should be injected
	assert.NotEmpty(t, carrier.Get("traceparent"))
}

func Test_startPublishSpan(t *testing.T) {
	setupTestTracer(t)

	ctx, span, userProps := startPublishSpan(context.Background(), "test/topic")
	defer span.End()

	// Span should be valid
	assert.True(t, span.SpanContext().IsValid())
	assert.True(t, span.SpanContext().HasTraceID())
	assert.True(t, span.SpanContext().HasSpanID())

	// Context should contain the span
	spanFromCtx := trace.SpanFromContext(ctx)
	assert.Equal(t, span.SpanContext().TraceID(), spanFromCtx.SpanContext().TraceID())

	// User Properties should contain traceparent
	carrier := &userPropertyCarrier{props: &userProps}
	traceparent := carrier.Get("traceparent")
	assert.NotEmpty(t, traceparent)
	assert.Contains(t, traceparent, span.SpanContext().TraceID().String())
}

func Test_startSubscribeSpan_WithValidContext(t *testing.T) {
	setupTestTracer(t)

	// Simulate publisher side
	pubCtx, pubSpan, userProps := startPublishSpan(context.Background(), "test/topic")
	_ = pubCtx

	pubTraceID := pubSpan.SpanContext().TraceID()
	pubSpanID := pubSpan.SpanContext().SpanID()
	pubSpan.End()

	// Simulate subscriber side
	subCtx, subSpan := startSubscribeSpan(context.Background(), "test/topic", userProps)
	defer subSpan.End()

	// Subscriber span should be in the same trace as publisher
	assert.Equal(t, pubTraceID, subSpan.SpanContext().TraceID(),
		"subscriber should be in the same trace as publisher")

	// Context should contain the subscriber span
	spanFromCtx := trace.SpanFromContext(subCtx)
	assert.Equal(t, subSpan.SpanContext().SpanID(), spanFromCtx.SpanContext().SpanID())

	// Verify the span has links by checking it's a valid ReadWriteSpan
	if roSpan, ok := subSpan.(sdktrace.ReadOnlySpan); ok {
		links := roSpan.Links()
		require.Len(t, links, 1, "subscriber span should have exactly one link")
		assert.Equal(t, pubSpanID, links[0].SpanContext.SpanID(),
			"link should point to the publisher span")
		assert.Equal(t, pubTraceID, links[0].SpanContext.TraceID(),
			"link should be in the same trace")
	}
}

func Test_startSubscribeSpan_WithoutContext(t *testing.T) {
	setupTestTracer(t)

	// Subscribe with empty User Properties (no trace context)
	_, subSpan := startSubscribeSpan(context.Background(), "test/topic", nil)
	defer subSpan.End()

	// Span should still be valid
	assert.True(t, subSpan.SpanContext().IsValid())

	// Verify no links
	if roSpan, ok := subSpan.(sdktrace.ReadOnlySpan); ok {
		assert.Empty(t, roSpan.Links(), "subscriber span should have no links without trace context")
	}
}

func Test_startSubscribeSpan_WithEmptyUserProps(t *testing.T) {
	setupTestTracer(t)

	emptyProps := paho.UserProperties{}

	_, subSpan := startSubscribeSpan(context.Background(), "test/topic", emptyProps)
	defer subSpan.End()

	assert.True(t, subSpan.SpanContext().IsValid())

	if roSpan, ok := subSpan.(sdktrace.ReadOnlySpan); ok {
		assert.Empty(t, roSpan.Links())
	}
}
