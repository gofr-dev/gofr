package mqtt

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "gofr-mqtt"

// attributeCarrier implements propagation.TextMapCarrier for MQTT message metadata.
type attributeCarrier map[string]string

// Ensure attributeCarrier implements the interface at compile time.
var _ propagation.TextMapCarrier = attributeCarrier(nil)

// Get returns the value for a given key from the MQTT message metadata.
func (c attributeCarrier) Get(key string) string {
	return c[key]
}

// Set sets a key-value pair in the MQTT message metadata.
func (c attributeCarrier) Set(key, value string) {
	c[key] = value
}

// Keys returns all keys in the MQTT message metadata.
func (c attributeCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}

	return keys
}

// injectTraceContext injects the current trace context into MQTT message metadata.
func injectTraceContext(ctx context.Context, attrs map[string]string) map[string]string {
	if attrs == nil {
		attrs = make(map[string]string)
	}

	carrier := attributeCarrier(attrs)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	return attrs
}

// extractTraceLinks extracts the trace context from MQTT message metadata
// and returns span links to the producer span.
// If no trace context is found, returns nil (creating an orphan span).
func extractTraceLinks(attrs map[string]string) []trace.Link {
	if len(attrs) == 0 {
		return nil
	}

	carrier := attributeCarrier(attrs)

	extractedCtx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)

	spanCtx := trace.SpanContextFromContext(extractedCtx)

	if spanCtx.IsValid() {
		return []trace.Link{
			{
				SpanContext: spanCtx,
			},
		}
	}

	return nil
}

// startPublishSpan creates a new producer span for publishing.
// Returns the updated context, the span, and message metadata with injected trace context.
// Note: MQTT 3.1.1 does not support user properties, so the returned metadata
// cannot be transmitted to the broker. This is retained for MQTT 5.0 readiness.
func startPublishSpan(ctx context.Context, topic string) (context.Context, trace.Span, map[string]string) {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "mqtt"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.operation", "publish"),
		),
	}

	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(ctx, "mqtt-publish", opts...)

	attrs := injectTraceContext(ctx, nil)

	return ctx, span, attrs
}

// startSubscribeSpan creates a new consumer span for subscribing with links to the producer span.
// If trace context exists in message metadata, creates a span linked to the producer.
// Otherwise, creates an orphan span (new trace).
func startSubscribeSpan(ctx context.Context, topic string, msgAttrs map[string]string) (context.Context, trace.Span) {
	links := extractTraceLinks(msgAttrs)

	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "mqtt"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.operation", "receive"),
		),
	}

	if len(links) > 0 {
		opts = append(opts, trace.WithLinks(links...))
	}

	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(ctx, "mqtt-subscribe", opts...)

	return ctx, span
}
