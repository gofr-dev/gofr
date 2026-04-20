package google

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "gofr-gcp-pubsub"

// attributeCarrier implements propagation.TextMapCarrier for Google PubSub message attributes.
type attributeCarrier map[string]string

// Ensure attributeCarrier implements the interface at compile time.
var _ propagation.TextMapCarrier = attributeCarrier(nil)

// Get returns the value for a given key from the PubSub attributes.
func (c attributeCarrier) Get(key string) string {
	return c[key]
}

// Set sets a key-value pair in the PubSub attributes.
func (c attributeCarrier) Set(key, value string) {
	c[key] = value
}

// Keys returns all keys in the PubSub attributes.
func (c attributeCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}

	return keys
}

// injectTraceContext injects the current trace context into PubSub message attributes.
// This allows the consumer to extract the trace context and create span links.
func injectTraceContext(ctx context.Context, attrs map[string]string) map[string]string {
	if attrs == nil {
		attrs = make(map[string]string)
	}

	carrier := attributeCarrier(attrs)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	return attrs
}

// extractTraceLinks extracts the trace context from PubSub message attributes
// and returns span links to the producer span.
// If no trace context is found, returns nil (creating an orphan span).
func extractTraceLinks(attrs map[string]string) []trace.Link {
	if len(attrs) == 0 {
		return nil
	}

	carrier := attributeCarrier(attrs)

	// Extract the context from attributes
	extractedCtx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)

	// Get span context from extracted context
	spanCtx := trace.SpanContextFromContext(extractedCtx)

	// If valid span context exists, create a link to it
	if spanCtx.IsValid() {
		return []trace.Link{
			{
				SpanContext: spanCtx,
			},
		}
	}

	return nil
}

// startPublishSpan creates a new span for publishing with trace context injection.
// Returns the updated context, the span, and message attributes with injected trace context.
func startPublishSpan(ctx context.Context, topic string) (context.Context, trace.Span, map[string]string) {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "gcp_pubsub"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.operation", "publish"),
		),
	}

	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(ctx, "gcp-publish", opts...)

	// Inject trace context into message attributes
	attrs := injectTraceContext(ctx, nil)

	return ctx, span, attrs
}

// extractMessageAttrs extracts string map attributes from message metadata.
// Returns nil if metadata is nil or not of type map[string]string.
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
// If a valid trace context is found in message attributes, the consumer span
// becomes a child of the producer's span (same trace ID), AND a span link is
// attached so OTel-aware tools can still model fan-out. Otherwise, the span
// starts under whatever span (if any) is already in ctx.
func startSubscribeSpan(ctx context.Context, topic string, msgAttrs map[string]string) (context.Context, trace.Span) {
	// Extract producer's trace context once and reuse for both parent and link
	// to avoid parsing the same carrier twice.
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
			attribute.String("messaging.system", "gcp_pubsub"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.operation", "receive"),
		),
	}

	if len(links) > 0 {
		opts = append(opts, trace.WithLinks(links...))
	}

	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(parentCtx, "gcp-subscribe", opts...)

	return ctx, span
}
