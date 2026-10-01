package eventhub

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "gofr-eventhub"

// mapCarrier implements propagation.TextMapCarrier for EventHub properties map.
type mapCarrier map[string]any

func (c mapCarrier) Get(key string) string {
	if val, ok := c[key]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}

	return ""
}

func (c mapCarrier) Set(key, value string) {
	c[key] = value
}

func (c mapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}

	return keys
}

// startSubscribeSpan creates a new span for subscribing.
// If a valid trace context is found in properties, the consumer span links
// to the producer's span using OpenTelemetry trace links.
func startSubscribeSpan(ctx context.Context, topic string, properties map[string]any) (context.Context, trace.Span) {
	parentCtx := ctx
	var links []trace.Link

	if len(properties) > 0 {
		carrier := mapCarrier(properties)
		extractedCtx := otel.GetTextMapPropagator().Extract(ctx, carrier)

		if spanCtx := trace.SpanContextFromContext(extractedCtx); spanCtx.IsValid() {
			parentCtx = extractedCtx
			links = []trace.Link{{SpanContext: spanCtx}}
		}
	}

	opts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "eventhub"),
			attribute.String("messaging.destination.name", topic),
			attribute.String("messaging.operation", "subscribe"),
		),
	}

	if len(links) > 0 {
		opts = append(opts, trace.WithLinks(links...))
	}

	return otel.GetTracerProvider().Tracer(tracerName).Start(parentCtx, "eventhub-subscribe", opts...)
}
