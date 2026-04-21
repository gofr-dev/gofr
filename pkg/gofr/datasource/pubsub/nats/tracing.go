package nats

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "gofr-nats"

// headerCarrier implements propagation.TextMapCarrier for NATS message headers.
type headerCarrier nats.Header

// Get returns the first value for a given key from NATS message headers.
func (c headerCarrier) Get(key string) string {
	vals := nats.Header(c).Values(key)
	if len(vals) == 0 {
		return ""
	}

	return vals[0]
}

// Set sets a key-value pair in the NATS message headers.
func (c headerCarrier) Set(key, value string) {
	nats.Header(c).Set(key, value)
}

// Keys returns all keys in the NATS message headers.
func (c headerCarrier) Keys() []string {
	h := nats.Header(c)
	keys := make([]string, 0, len(h))

	for k := range h {
		keys = append(keys, k)
	}

	return keys
}

// injectTraceContext injects the current trace context into NATS message headers.
func injectTraceContext(ctx context.Context, headers nats.Header) nats.Header {
	if headers == nil {
		headers = make(nats.Header)
	}

	carrier := headerCarrier(headers)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	return headers
}

// startPublishSpan creates a new span for publishing with trace context injection.
// Returns the updated context, the span, and NATS headers with injected trace context.
func startPublishSpan(ctx context.Context, tracer trace.Tracer, subject string) (context.Context, trace.Span, nats.Header) {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", subject),
			attribute.String("messaging.operation", "publish"),
		),
	}

	ctx, span := tracer.Start(ctx, "nats-publish", opts...)

	headers := injectTraceContext(ctx, nil)

	return ctx, span, headers
}

// startSubscribeSpan creates a new span for subscribing.
// If a valid trace context is found in message headers, the consumer span
// becomes a child of the producer's span (same trace ID), AND a span link is
// attached so OTel-aware tools can still model fan-out. Otherwise, the span
// starts under whatever span (if any) is already in ctx.
func startSubscribeSpan(ctx context.Context, tracer trace.Tracer, topic string, headers nats.Header) (context.Context, trace.Span) {
	// Extract producer's trace context once and reuse for both parent and link
	// to avoid parsing the same carrier twice.
	parentCtx := ctx

	var links []trace.Link

	if len(headers) > 0 {
		carrier := headerCarrier(headers)
		extractedCtx := otel.GetTextMapPropagator().Extract(ctx, carrier)

		if spanCtx := trace.SpanContextFromContext(extractedCtx); spanCtx.IsValid() {
			parentCtx = extractedCtx
			links = []trace.Link{{SpanContext: spanCtx}}
		}
	}

	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.operation", "receive"),
		),
	}

	if len(links) > 0 {
		opts = append(opts, trace.WithLinks(links...))
	}

	ctx, span := tracer.Start(parentCtx, "nats-subscribe", opts...)

	return ctx, span
}
