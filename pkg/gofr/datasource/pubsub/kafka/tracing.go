package kafka

import (
	"context"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "gofr.dev/pkg/gofr/datasource/pubsub/kafka"

// headerCarrier implements propagation.TextMapCarrier for Kafka headers.
type headerCarrier []kafka.Header

// Get returns the value for a given key from the Kafka headers.
func (c headerCarrier) Get(key string) string {
	for _, h := range c {
		if h.Key == key {
			return string(h.Value)
		}
	}

	return ""
}

// Set sets a key-value pair in the Kafka headers.
func (c *headerCarrier) Set(key, value string) {
	// Check if key exists and update it
	for i, h := range *c {
		if h.Key == key {
			(*c)[i].Value = []byte(value)
			return
		}
	}

	// Key doesn't exist, append new header
	*c = append(*c, kafka.Header{Key: key, Value: []byte(value)})
}

// Keys returns all keys in the Kafka headers.
func (c headerCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for _, h := range c {
		keys = append(keys, h.Key)
	}

	return keys
}

// injectTraceContext injects the current trace context into Kafka message headers.
// This allows the consumer to extract the trace context and create span links.
func injectTraceContext(ctx context.Context, headers []kafka.Header) []kafka.Header {
	carrier := headerCarrier(headers)
	otel.GetTextMapPropagator().Inject(ctx, &carrier)

	return carrier
}

// extractTraceLinks extracts the trace context from Kafka message headers
// and returns span links to the producer span.
// If no trace context is found, returns empty links (creating an orphan span).
func extractTraceLinks(headers []kafka.Header) []trace.Link {
	carrier := headerCarrier(headers)

	// Extract the context from headers
	extractedCtx := otel.GetTextMapPropagator().Extract(context.Background(), &carrier)

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

	// No valid trace context found, return empty links (orphan span)
	return nil
}

// startPublishSpan creates a new span for publishing with trace context injection.
// Returns the updated context for logging and the headers with injected trace context.
func startPublishSpan(ctx context.Context, topic string) (context.Context, trace.Span, []kafka.Header) {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.operation", "publish"),
		),
	}

	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(ctx, "kafka-publish", opts...)

	// Inject trace context into headers
	headers := injectTraceContext(ctx, nil)

	return ctx, span, headers
}

// startSubscribeSpan creates a new span for subscribing with links to the producer span.
// If trace context exists in headers, creates a span linked to the producer.
// Otherwise, creates an orphan span (new trace).
func startSubscribeSpan(ctx context.Context, topic string, msgHeaders []kafka.Header) (context.Context, trace.Span) {
	// Extract links from message headers
	links := extractTraceLinks(msgHeaders)

	// Create span with links if any exist
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "kafka"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.operation", "receive"),
		),
	}

	if len(links) > 0 {
		opts = append(opts, trace.WithLinks(links...))
	}

	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(ctx, "kafka-subscribe", opts...)

	return ctx, span
}
