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

// Get returns the value for a given key from the MQTT metadata.
func (c attributeCarrier) Get(key string) string {
	return c[key]
}

// Set sets a key-value pair in the MQTT metadata.
func (c attributeCarrier) Set(key, value string) {
	c[key] = value
}

// Keys returns all keys in the MQTT metadata.
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

// startPublishSpan creates a new span for publishing with trace context injection.
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

	// Inject trace context into message attributes
	attrs := injectTraceContext(ctx, nil)

	return ctx, span, attrs
}

// extractMessageAttrs extracts string map attributes from message metadata.
func extractMessageAttrs(metaData any) map[string]string {
	if metaData == nil {
		return nil
	}

	if attrs, ok := metaData.(map[string]string); ok {
		return attrs
	}

	return nil
}

// startSubscribeSpan creates a new span for subscribing.
func startSubscribeSpan(ctx context.Context, topic string, msgAttrs map[string]string) (context.Context, trace.Span) {
	parentCtx := ctx

	var links []trace.Link

	if len(msgAttrs) > 0 {
		carrier := attributeCarrier(msgAttrs)
		extractedCtx := otel.GetTextMapPropagator().Extract(ctx, carrier)

		if spanCtx := trace.SpanContextFromContext(extractedCtx); spanCtx.IsValid() {
			parentCtx = extractedCtx
			links = []trace.Link{{SpanContext: spanCtx}}
		}
	}

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

	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(parentCtx, "mqtt-subscribe", opts...)

	return ctx, span
}
