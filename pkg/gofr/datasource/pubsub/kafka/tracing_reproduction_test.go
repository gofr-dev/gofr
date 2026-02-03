package kafka

import (
	"context"
	"encoding/json"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// setupTestTracing creates a tracer provider with span recorder for testing.
func setupTestTracing() (*tracetest.SpanRecorder, *trace.TracerProvider) {
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(
		trace.WithSpanProcessor(spanRecorder),
	)
	otel.SetTracerProvider(tracerProvider)

	return spanRecorder, tracerProvider
}

// logSpanDetails logs span information for debugging.
func logSpanDetails(t *testing.T, spans []trace.ReadOnlySpan) {
	t.Helper()

	t.Logf("\n=== Span Analysis ===")
	t.Logf("Total spans recorded: %d", len(spans))

	for i, span := range spans {
		t.Logf("\nSpan %d:", i+1)
		t.Logf("  Name: %s", span.Name())
		t.Logf("  TraceID: %s", span.SpanContext().TraceID())
		t.Logf("  SpanID:  %s", span.SpanContext().SpanID())

		if span.Parent().IsValid() {
			t.Logf("  Parent SpanID: %s", span.Parent().SpanID())
		} else {
			t.Logf("  Parent: None (root span)")
		}

		t.Logf("  Links: %v", span.Links())
	}
}

// logCurrentBehavior logs documentation about current behavior.
func logCurrentBehavior(t *testing.T) {
	t.Helper()

	t.Log("\n=== CURRENT BEHAVIOR (Parent-Child) ===")
	t.Log("✗ Publish span is CHILD of root request context")
	t.Log("✗ Subscribe span starts a NEW trace (different TraceID)")
	t.Log("✗ NO correlation between publisher and subscriber")
	t.Log("✗ NO trace context propagation via message headers")
	t.Log("✗ Async pub/sub pattern breaks distributed tracing")

	t.Log("\n=== EXPECTED BEHAVIOR (Span Links) ===")
	t.Log("✓ Publisher should inject trace context into message headers (W3C format)")
	t.Log("✓ Subscriber should extract trace context from headers")
	t.Log("✓ Subscriber should create span with LINK to publisher's span")
	t.Log("✓ Enables proper correlation in async, decoupled systems")
	t.Log("✓ Supports fan-out (1→many) and fan-in (many→1) patterns")
}

// TestCurrentTracingBehavior demonstrates the current parent-child tracing behavior
// in GoFr's Kafka pub/sub implementation.
//
// Key Observations:
// 1. Publisher creates a span as child of incoming context
// 2. Subscriber creates a span as child of incoming context
// 3. NO trace context is injected into message headers
// 4. NO trace context is extracted from message headers
// 5. NO span links are used.
func TestCurrentTracingBehavior(t *testing.T) {
	spanRecorder, tracerProvider := setupTestTracing()
	tracer := tracerProvider.Tracer("test")

	// Simulate a root request context
	rootCtx, rootSpan := tracer.Start(context.Background(), "root-request")
	defer rootSpan.End()

	t.Logf("Root Span Created:")
	t.Logf("  TraceID: %s", rootSpan.SpanContext().TraceID())
	t.Logf("  SpanID:  %s", rootSpan.SpanContext().SpanID())

	// Step 1: Simulate Publishing
	publishCtx, publishSpan := otel.GetTracerProvider().Tracer("gofr").Start(rootCtx, "kafka-publish")
	message := map[string]string{"test": "data"}
	msgBytes, _ := json.Marshal(message)
	_ = msgBytes
	publishSpan.End()

	t.Logf("\nPublish Span Created:")
	t.Logf("  TraceID: %s", publishSpan.SpanContext().TraceID())
	t.Logf("  SpanID:  %s", publishSpan.SpanContext().SpanID())
	t.Logf("  Has same TraceID as root: %v", publishSpan.SpanContext().TraceID() == rootSpan.SpanContext().TraceID())
	t.Logf("  Is child of root span: true (parent-child relationship)")

	// Step 2: Simulate Subscribing
	newSubscriberCtx := context.Background()
	subscribeCtx, subscribeSpan := otel.GetTracerProvider().Tracer("gofr").Start(newSubscriberCtx, "kafka-subscribe")
	_ = publishCtx
	_ = subscribeCtx
	subscribeSpan.End()

	t.Logf("\nSubscribe Span Created:")
	t.Logf("  TraceID: %s", subscribeSpan.SpanContext().TraceID())
	t.Logf("  SpanID:  %s", subscribeSpan.SpanContext().SpanID())
	t.Logf("  Has same TraceID as publish: %v", subscribeSpan.SpanContext().TraceID() == publishSpan.SpanContext().TraceID())
	t.Logf("  Has same TraceID as root: %v", subscribeSpan.SpanContext().TraceID() == rootSpan.SpanContext().TraceID())

	rootSpan.End()

	logSpanDetails(t, spanRecorder.Ended())
	logCurrentBehavior(t)
}

// TestExpectedSpanLinksBehavior demonstrates how span links SHOULD work
// according to OpenTelemetry messaging semantic conventions.
func TestExpectedSpanLinksBehavior(t *testing.T) {
	spanRecorder, tracerProvider := setupTestTracing()
	tracer := tracerProvider.Tracer("test")

	// Root context
	rootCtx, rootSpan := tracer.Start(context.Background(), "root-request")

	// Step 1: Publisher creates a span
	_, publishSpan := tracer.Start(rootCtx, "kafka-publish")
	publishSpanContext := publishSpan.SpanContext()
	publishSpan.End()

	t.Logf("Publisher Span:")
	t.Logf("  TraceID: %s", publishSpanContext.TraceID())
	t.Logf("  SpanID:  %s", publishSpanContext.SpanID())

	rootSpan.End()

	// Step 2: Subscriber creates a span WITH LINK to publisher
	newConsumerCtx := context.Background()
	link := oteltrace.Link{SpanContext: publishSpanContext}

	_, subscribeSpan := tracer.Start(newConsumerCtx, "kafka-subscribe",
		oteltrace.WithLinks(link),
	)
	subscribeSpan.End()

	t.Logf("\nSubscriber Span (with link):")
	t.Logf("  TraceID: %s", subscribeSpan.SpanContext().TraceID())
	t.Logf("  SpanID:  %s", subscribeSpan.SpanContext().SpanID())
	t.Logf("  Different trace? %v", subscribeSpan.SpanContext().TraceID() != publishSpanContext.TraceID())

	// Analyze spans
	spans := spanRecorder.Ended()

	t.Logf("\n=== Span Analysis ===")

	for i, span := range spans {
		t.Logf("\nSpan %d:", i+1)
		t.Logf("  Name: %s", span.Name())
		t.Logf("  TraceID: %s", span.SpanContext().TraceID())
		t.Logf("  SpanID:  %s", span.SpanContext().SpanID())

		if span.Parent().IsValid() {
			t.Logf("  Parent SpanID: %s", span.Parent().SpanID())
		} else {
			t.Logf("  Parent: None")
		}

		if len(span.Links()) > 0 {
			t.Logf("  Links:")

			for j, l := range span.Links() {
				t.Logf("    Link %d: TraceID=%s, SpanID=%s",
					j+1,
					l.SpanContext.TraceID(),
					l.SpanContext.SpanID())
			}
		}
	}

	t.Log("\n✓ Span links enable proper async correlation!")
	t.Log("✓ Publisher and subscriber can have different TraceIDs")
	t.Log("✓ Yet they are still correlated through the span link")
	t.Log("✓ This is the OpenTelemetry recommended pattern for messaging")
}
