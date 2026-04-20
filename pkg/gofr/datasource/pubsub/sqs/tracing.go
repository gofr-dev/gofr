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

// extractTraceLinks extracts the trace context from SQS message attributes
// and returns span links to the producer span.
// If no trace context is found, returns nil (creating an orphan span).
func extractTraceLinks(attrs map[string]types.MessageAttributeValue) []trace.Link {
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
// If trace context exists in message attributes, the consumer span becomes a
// child of the producer's span (same trace ID), AND a span link is attached
// so OTel-aware tools can still model fan-out. Otherwise, creates an orphan span.
func startSubscribeSpan(ctx context.Context, topic string, msgAttrs map[string]types.MessageAttributeValue) (context.Context, trace.Span) {
	// Extract producer's trace context from attributes and use it as the parent.
	parentCtx := ctx
	if len(msgAttrs) > 0 {
		carrier := attributeCarrier(msgAttrs)
		parentCtx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	}

	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "aws_sqs"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.operation", "receive"),
		),
	}

	if links := extractTraceLinks(msgAttrs); len(links) > 0 {
		opts = append(opts, trace.WithLinks(links...))
	}

	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(parentCtx, "sqs-subscribe", opts...)

	return ctx, span
}
