package sqs

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "gofr-sqs"

// attributeCarrier implements propagation.TextMapCarrier for SQS message attributes.
type attributeCarrier map[string]types.MessageAttributeValue

// Get returns the string value for a given key from SQS message attributes.
func (c attributeCarrier) Get(key string) string {
	if attr, ok := c[key]; ok && attr.StringValue != nil {
		return *attr.StringValue
	}

	return ""
}

// Set sets a key-value pair in the SQS message attributes as a String type.
func (c attributeCarrier) Set(key, value string) {
	c[key] = types.MessageAttributeValue{
		DataType:    aws.String("String"),
		StringValue: aws.String(value),
	}
}

// Keys returns all keys in the SQS message attributes.
func (c attributeCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}

	return keys
}

// injectTraceContext injects the current trace context into SQS message attributes.
// This allows the consumer to extract the trace context and create span links.
func injectTraceContext(ctx context.Context, attrs map[string]types.MessageAttributeValue) map[string]types.MessageAttributeValue {
	if attrs == nil {
		attrs = make(map[string]types.MessageAttributeValue)
	}

	carrier := attributeCarrier(attrs)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	return attrs
}

// startPublishSpan creates a new span for publishing with trace context injection.
// Returns the updated context, the span, and message attributes with injected trace context.
func startPublishSpan(ctx context.Context, topic string) (context.Context, trace.Span, map[string]types.MessageAttributeValue) {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "aws_sqs"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.operation", "publish"),
		),
	}

	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(ctx, "sqs-publish", opts...)

	// Inject trace context into message attributes
	attrs := injectTraceContext(ctx, nil)

	return ctx, span, attrs
}

// startSubscribeSpan creates a new span for subscribing.
// If a valid trace context is found in message attributes, the consumer span
// becomes a child of the producer's span (same trace ID), AND a span link is
// attached so OTel-aware tools can still model fan-out. Otherwise, the span
// starts under whatever span (if any) is already in ctx.
func startSubscribeSpan(ctx context.Context, topic string, msgAttrs map[string]types.MessageAttributeValue) (context.Context, trace.Span) {
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
			attribute.String("messaging.system", "aws_sqs"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.operation", "receive"),
		),
	}

	if len(links) > 0 {
		opts = append(opts, trace.WithLinks(links...))
	}

	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(parentCtx, "sqs-subscribe", opts...)

	return ctx, span
}
