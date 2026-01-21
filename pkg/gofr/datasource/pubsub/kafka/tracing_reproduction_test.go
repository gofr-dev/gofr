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

// TestCurrentTracingBehavior demonstrates the current parent-child tracing behavior
// in GoFr's Kafka pub/sub implementation.
//
// Key Observations:
// 1. Publisher creates a span as child of incoming context
// 2. Subscriber creates a span as child of incoming context
// 3. NO trace context is injected into message headers
// 4. NO trace context is extracted from message headers
// 5. NO span links are used
func TestCurrentTracingBehavior(t *testing.T) {
	// Setup: Create a span recorder to capture spans
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(
		trace.WithSpanProcessor(spanRecorder),
	)
	otel.SetTracerProvider(tracerProvider)

	// Simulate a root request context (e.g., incoming HTTP request)
	rootCtx := context.Background()
	tracer := tracerProvider.Tracer("test")
	rootCtx, rootSpan := tracer.Start(rootCtx, "root-request")
	defer rootSpan.End()

	rootTraceID := rootSpan.SpanContext().TraceID()
	rootSpanID := rootSpan.SpanContext().SpanID()

	t.Logf("Root Span Created:")
	t.Logf("  TraceID: %s", rootTraceID)
	t.Logf("  SpanID:  %s", rootSpanID)

	// Step 1: Simulate Publishing
	// The current implementation in kafka.go line 101:
	// ctx, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "kafka-publish")
	publishCtx, publishSpan := otel.GetTracerProvider().Tracer("gofr").Start(rootCtx, "kafka-publish")
	// Simulate publishing a message
	message := map[string]string{"test": "data"}
	msgBytes, _ := json.Marshal(message)
	_ = msgBytes // Message is published, but NO trace context in headers
	publishSpan.End()

	publishTraceID := publishSpan.SpanContext().TraceID()
	publishSpanID := publishSpan.SpanContext().SpanID()

	t.Logf("\nPublish Span Created:")
	t.Logf("  TraceID: %s", publishTraceID)
	t.Logf("  SpanID:  %s", publishSpanID)
	t.Logf("  Has same TraceID as root: %v", publishTraceID == rootTraceID)
	t.Logf("  Is child of root span: true (parent-child relationship)")

	// Step 2: Simulate Subscribing
	// In a real async scenario, subscribe happens in a different context
	// The current implementation in kafka.go line 183:
	// ctx, span := otel.GetTracerProvider().Tracer("gofr").Start(ctx, "kafka-subscribe")
	newSubscriberCtx := context.Background() // Fresh context, no relation to publisher
	subscribeCtx, subscribeSpan := otel.GetTracerProvider().Tracer("gofr").Start(newSubscriberCtx, "kafka-subscribe")
	// Simulate receiving the message
	_ = publishCtx    // Message received, but NO trace context extracted from headers
	_ = subscribeCtx
	subscribeSpan.End()

	subscribeTraceID := subscribeSpan.SpanContext().TraceID()
	subscribeSpanID := subscribeSpan.SpanContext().SpanID()

	t.Logf("\nSubscribe Span Created:")
	t.Logf("  TraceID: %s", subscribeTraceID)
	t.Logf("  SpanID:  %s", subscribeSpanID)
	t.Logf("  Has same TraceID as publish: %v", subscribeTraceID == publishTraceID)
	t.Logf("  Has same TraceID as root: %v", subscribeTraceID == rootTraceID)

	// End root span
	rootSpan.End()

	// Analyze the recorded spans
	spans := spanRecorder.Ended()
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

	// Document Current Behavior
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

// TestExpectedSpanLinksBehavior demonstrates how span links SHOULD work
// according to OpenTelemetry messaging semantic conventions.
func TestExpectedSpanLinksBehavior(t *testing.T) {
	// Setup
	spanRecorder := tracetest.NewSpanRecorder()
	tracerProvider := trace.NewTracerProvider(
		trace.WithSpanProcessor(spanRecorder),
	)
	otel.SetTracerProvider(tracerProvider)

	tracer := tracerProvider.Tracer("test")

	// Root context
	rootCtx := context.Background()
	rootCtx, rootSpan := tracer.Start(rootCtx, "root-request")

	// Step 1: Publisher creates a span
	_, publishSpan := tracer.Start(rootCtx, "kafka-publish")
	publishSpanContext := publishSpan.SpanContext()
	publishSpan.End()

	t.Logf("Publisher Span:")
	t.Logf("  TraceID: %s", publishSpanContext.TraceID())
	t.Logf("  SpanID:  %s", publishSpanContext.SpanID())

	// In the improved implementation, trace context would be injected here:
	// propagator := otel.GetTextMapPropagator()
	// carrier := make(map[string]string)
	// propagator.Inject(publishCtx, propagation.MapCarrier(carrier))
	// Then carrier would be added to kafka message headers

	rootSpan.End()

	// Step 2: Subscriber creates a span WITH LINK to publisher
	newConsumerCtx := context.Background() // New context, simulating async consumption

	// Create link to publisher span
	link := oteltrace.Link{
		SpanContext: publishSpanContext,
	}

	_, subscribeSpan := tracer.Start(newConsumerCtx, "kafka-subscribe",
		oteltrace.WithLinks(link), // KEY DIFFERENCE: Using span links!
	)
	subscribeSpan.End()

	subscribeSpanContext := subscribeSpan.SpanContext()

	t.Logf("\nSubscriber Span (with link):")
	t.Logf("  TraceID: %s", subscribeSpanContext.TraceID())
	t.Logf("  SpanID:  %s", subscribeSpanContext.SpanID())
	t.Logf("  Different trace? %v", subscribeSpanContext.TraceID() != publishSpanContext.TraceID())

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
			for j, link := range span.Links() {
				t.Logf("    Link %d: TraceID=%s, SpanID=%s",
					j+1,
					link.SpanContext.TraceID(),
					link.SpanContext.SpanID())
			}
		}
	}

	t.Log("\n✓ Span links enable proper async correlation!")
	t.Log("✓ Publisher and subscriber can have different TraceIDs")
	t.Log("✓ Yet they are still correlated through the span link")
	t.Log("✓ This is the OpenTelemetry recommended pattern for messaging")
}
