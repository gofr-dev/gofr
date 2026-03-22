package mqtt

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "gofr-mqtt"

// traceEnvelope is the JSON structure used to wrap payloads with trace context.
// MQTT v3.1.1 does not support message-level headers, so trace context is embedded in the payload.
type traceEnvelope struct {
	Marker      string `json:"_gofr_trace"`
	Traceparent string `json:"tp,omitempty"`
	Tracestate  string `json:"ts,omitempty"`
	Data        string `json:"d"`
}

// headerCarrier implements propagation.TextMapCarrier for trace context propagation.
type headerCarrier map[string]string

// Ensure headerCarrier implements the interface at compile time.
var _ propagation.TextMapCarrier = headerCarrier(nil)

// Get returns the value for a given key.
func (c headerCarrier) Get(key string) string {
	return c[key]
}

// Set sets a key-value pair.
func (c headerCarrier) Set(key, value string) {
	c[key] = value
}

// Keys returns all keys.
func (c headerCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}

	return keys
}

// injectTraceContext injects the current trace context into a string map.
func injectTraceContext(ctx context.Context, headers map[string]string) map[string]string {
	if headers == nil {
		headers = make(map[string]string)
	}

	carrier := headerCarrier(headers)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	return headers
}

// extractTraceLinks extracts the trace context from headers and returns span links to the producer span.
// If no trace context is found, returns nil (creating an orphan span).
func extractTraceLinks(headers map[string]string) []trace.Link {
	if len(headers) == 0 {
		return nil
	}

	carrier := headerCarrier(headers)

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

// startPublishSpan creates a new span for publishing with trace context injection.
// Returns the updated context, span, and headers with injected trace context.
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

	headers := injectTraceContext(ctx, nil)

	return ctx, span, headers
}

// startSubscribeSpan creates a new span for subscribing with links to the producer span.
// If trace context exists in headers, creates a span linked to the producer.
// Otherwise, creates an orphan span (new trace).
func startSubscribeSpan(ctx context.Context, topic string, headers map[string]string) (context.Context, trace.Span) {
	links := extractTraceLinks(headers)

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

// wrapPayload wraps the original payload with trace headers into a JSON envelope.
// MQTT v3.1.1 has no message-level headers, so this embeds trace context in the payload.
func wrapPayload(traceHeaders map[string]string, payload []byte) []byte {
	env := traceEnvelope{
		Marker:      "1",
		Traceparent: traceHeaders["traceparent"],
		Tracestate:  traceHeaders["tracestate"],
		Data:        base64.StdEncoding.EncodeToString(payload),
	}

	data, err := json.Marshal(env)
	if err != nil {
		return payload
	}

	return data
}

// unwrapPayload extracts trace headers and the original payload from a JSON envelope.
// If the data is not a wrapped payload, returns (nil, originalData) for backward compatibility.
func unwrapPayload(data []byte) (headers map[string]string, payload []byte) {
	var env traceEnvelope

	if err := json.Unmarshal(data, &env); err != nil {
		return nil, data
	}

	if env.Marker != "1" {
		return nil, data
	}

	payload, err := base64.StdEncoding.DecodeString(env.Data)
	if err != nil {
		return nil, data
	}

	headers = make(map[string]string)

	if env.Traceparent != "" {
		headers["traceparent"] = env.Traceparent
	}

	if env.Tracestate != "" {
		headers["tracestate"] = env.Tracestate
	}

	return headers, payload
}
