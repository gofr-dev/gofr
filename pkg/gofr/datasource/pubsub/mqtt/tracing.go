package mqtt

import (
	"context"

	"github.com/eclipse/paho.golang/paho"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "gofr-mqtt"

// userPropertyCarrier implements propagation.TextMapCarrier for MQTT 5.0 User Properties.
type userPropertyCarrier struct {
	props *paho.UserProperties
}

// Ensure userPropertyCarrier implements the interface at compile time.
var _ propagation.TextMapCarrier = (*userPropertyCarrier)(nil)

// Get returns the value for a given key from the MQTT User Properties.
func (c *userPropertyCarrier) Get(key string) string {
	if c.props == nil {
		return ""
	}

	for _, p := range *c.props {
		if p.Key == key {
			return p.Value
		}
	}

	return ""
}

// Set sets a key-value pair in the MQTT User Properties.
func (c *userPropertyCarrier) Set(key, value string) {
	if c.props == nil {
		return
	}

	// Check if key exists and update it
	for i, p := range *c.props {
		if p.Key == key {
			(*c.props)[i].Value = value
			return
		}
	}

	// Key doesn't exist, append new property
	*c.props = append(*c.props, paho.UserProperty{Key: key, Value: value})
}

// Keys returns all keys in the MQTT User Properties.
func (c *userPropertyCarrier) Keys() []string {
	if c.props == nil {
		return nil
	}

	keys := make([]string, 0, len(*c.props))
	for _, p := range *c.props {
		keys = append(keys, p.Key)
	}

	return keys
}

// injectTraceContext injects the current trace context into MQTT 5.0 User Properties.
// This allows the consumer to extract the trace context and create span links.
func injectTraceContext(ctx context.Context, props paho.UserProperties) paho.UserProperties {
	if props == nil {
		props = paho.UserProperties{}
	}

	carrier := &userPropertyCarrier{props: &props}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	return props
}

// startPublishSpan creates a new span for publishing with trace context injection.
// Returns the updated context, the span, and User Properties with injected trace context.
func startPublishSpan(ctx context.Context, topic string) (context.Context, trace.Span, paho.UserProperties) {
	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "mqtt"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.operation", "publish"),
		),
	}

	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(ctx, "mqtt-publish", opts...)

	// Inject trace context into User Properties
	userProps := injectTraceContext(ctx, nil)

	return ctx, span, userProps
}

// startSubscribeSpan creates a new span for subscribing.
// If a valid trace context is found in User Properties, the consumer span
// becomes a child of the producer's span (same trace ID), AND a span link is
// attached so OTel-aware tools can still model fan-out semantics. Otherwise,
// the span starts under whatever span (if any) is already in ctx.
func startSubscribeSpan(ctx context.Context, topic string, userProps paho.UserProperties) (context.Context, trace.Span) {
	// Extract producer's trace context once and reuse for both parent and link
	// to avoid parsing the same carrier twice.
	parentCtx := ctx

	var links []trace.Link

	if len(userProps) > 0 {
		carrier := &userPropertyCarrier{props: &userProps}
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
