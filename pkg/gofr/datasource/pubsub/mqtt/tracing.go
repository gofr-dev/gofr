package mqtt

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "gofr-mqtt"

// MQTT is the one pub/sub backend GoFr instruments that cannot carry a trace context across the
// broker, so its spans are structured differently from Kafka, NATS, SQS and Google Pub/Sub.
//
// Those four have a header or attribute slot on the wire, so the producer injects W3C traceparent
// and the consumer extracts it, making the two ends one trace. An MQTT 3.1.1 PUBLISH packet has
// only a topic, a QoS, a retain flag and a payload — there is no slot for it. Carrying traceparent
// would mean either MQTT 5 user properties, which paho.mqtt.golang v1.5.1 does not implement, or
// wrapping the payload in an envelope, which would break every non-GoFr subscriber on the topic.
//
// So the consume span is rooted in the caller's context instead. That is the right shape here
// regardless: GoFr's MQTT Subscribe is pull-style — a caller asks for the next message — so the
// work the span measures genuinely belongs to the consuming request's trace, not the producer's.
// What is lost is only the edge between the two, and nothing may claim otherwise: an extract that
// can never find anything reports "no link" identically to a broker that dropped the context, and
// that is the failure this file exists to not have.

// Hoisted because Start is variadic: a slice literal at each call site is 2 heap allocations per
// published message, measured, on a path that is otherwise 2 allocations deep in total. They are
// read-only after init and never escape to a caller.
//
//nolint:gochecknoglobals // hoisted to keep the publish path at development's 144 B/2 allocs
var (
	producerSpanOpts = []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindProducer)}
	consumerSpanOpts = []trace.SpanStartOption{trace.WithSpanKind(trace.SpanKindConsumer)}
)

// startPublishSpan starts the producer span for one Publish.
func startPublishSpan(ctx context.Context, topic string) (context.Context, trace.Span) {
	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(ctx, "mqtt-publish", producerSpanOpts...)

	setMessagingAttributes(span, topic, "publish")

	return ctx, span
}

// startSubscribeSpan starts the consumer span for one Subscribe, under whatever span the caller is
// already in. See the note above for why it has no producer to link to.
func startSubscribeSpan(ctx context.Context, topic string) (context.Context, trace.Span) {
	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(ctx, "mqtt-subscribe", consumerSpanOpts...)

	setMessagingAttributes(span, topic, "receive")

	return ctx, span
}

// setMessagingAttributes applies the messaging semantic-convention attributes, but only to a span
// that is actually recording.
//
// The guard is what keeps this off the hot path of the default deployment. With no TRACE_EXPORTER
// configured GoFr still installs an SDK provider, sampling NeverSample, so every publish reaches
// here and builds attributes the SDK immediately discards — 456 bytes and 5 allocations per
// message, measured, against a publish that is otherwise 144 bytes and 2. Passing them to Start
// instead would be the more usual spelling and would pay that cost unconditionally.
//
// Setting them after Start rather than at it is safe for GoFr because its sampler is
// ParentBased(TraceIDRatioBased) (see pkg/gofr/otel.go), which decides on trace ID and parent
// alone. A sampler that inspected attributes would need them passed to Start.
func setMessagingAttributes(span trace.Span, topic, operation string) {
	if !span.IsRecording() {
		return
	}

	span.SetAttributes(
		attribute.String("messaging.system", "mqtt"),
		attribute.String("messaging.destination.name", topic),
		attribute.String("messaging.operation", operation),
	)
}
