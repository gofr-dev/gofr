package mqtt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

// recorder installs a tracer provider that keeps every finished span, and returns it.
func recorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)

	t.Cleanup(func() {
		require.NoError(t, tp.Shutdown(context.Background()))
	})

	return rec
}

func attrOf(t *testing.T, s sdktrace.ReadOnlySpan, key string) string {
	t.Helper()

	for _, kv := range s.Attributes() {
		if kv.Key == attribute.Key(key) {
			return kv.Value.AsString()
		}
	}

	return ""
}

func TestStartPublishSpan(t *testing.T) {
	rec := recorder(t)

	tracer := otel.GetTracerProvider().Tracer("caller")
	callerCtx, caller := tracer.Start(context.Background(), "caller-request")

	ctx, span := startPublishSpan(callerCtx, "test-topic")
	span.End()
	caller.End()

	require.NotNil(t, ctx)

	spans := rec.Ended()
	require.Len(t, spans, 2)

	pub := spans[0]
	assert.Equal(t, "mqtt-publish", pub.Name())
	assert.Equal(t, trace.SpanKindProducer, pub.SpanKind())
	assert.Equal(t, "mqtt", attrOf(t, pub, "messaging.system"))
	assert.Equal(t, "test-topic", attrOf(t, pub, "messaging.destination.name"))
	assert.Equal(t, "publish", attrOf(t, pub, "messaging.operation"))

	// The producer span belongs to whatever request asked for the publish.
	assert.Equal(t, caller.SpanContext().TraceID(), pub.SpanContext().TraceID())
	assert.Equal(t, caller.SpanContext().SpanID(), pub.Parent().SpanID())
}

func TestStartSubscribeSpan_JoinsCallersTrace(t *testing.T) {
	rec := recorder(t)

	tracer := otel.GetTracerProvider().Tracer("caller")
	callerCtx, caller := tracer.Start(context.Background(), "consumer-request")

	_, span := startSubscribeSpan(callerCtx, "test-topic")
	span.End()
	caller.End()

	spans := rec.Ended()
	require.Len(t, spans, 2)

	sub := spans[0]
	assert.Equal(t, "mqtt-subscribe", sub.Name())
	assert.Equal(t, trace.SpanKindConsumer, sub.SpanKind())
	assert.Equal(t, "mqtt", attrOf(t, sub, "messaging.system"))
	assert.Equal(t, "test-topic", attrOf(t, sub, "messaging.destination.name"))
	assert.Equal(t, "receive", attrOf(t, sub, "messaging.operation"))

	// GoFr's MQTT Subscribe is pull-style, so the consume belongs to the caller's trace.
	assert.Equal(t, caller.SpanContext().TraceID(), sub.SpanContext().TraceID())
	assert.Equal(t, caller.SpanContext().SpanID(), sub.Parent().SpanID())
}

func TestStartSubscribeSpan_WithoutCallerSpanIsRoot(t *testing.T) {
	rec := recorder(t)

	_, span := startSubscribeSpan(context.Background(), "test-topic")
	span.End()

	spans := rec.Ended()
	require.Len(t, spans, 1)

	assert.True(t, spans[0].SpanContext().IsValid())
	assert.False(t, spans[0].Parent().IsValid())
}

// TestSubscribeSpan_HasNoProducerLink pins the documented limitation in tracing.go: an MQTT 3.1.1
// PUBLISH has nowhere to carry traceparent, so there is no producer context to link to and the
// consume span must not claim one. An earlier revision of this package extracted from
// pubsub.Message.MetaData, which only ever holds qos, retained and messageID — the extract could
// never succeed, and the tests passed only because they fed it attributes the broker never saw.
//
// If MQTT 5 user properties are added later this test should be replaced by one asserting the link
// survives a real round trip, not deleted to make a green build.
func TestSubscribeSpan_HasNoProducerLink(t *testing.T) {
	rec := recorder(t)

	tracer := otel.GetTracerProvider().Tracer("producer")
	prodCtx, producer := tracer.Start(context.Background(), "producer-request")

	_, pub := startPublishSpan(prodCtx, "test-topic")
	pub.End()
	producer.End()

	// A consumer in a different trace, which is what a separate process is.
	_, sub := startSubscribeSpan(context.Background(), "test-topic")
	sub.End()

	var subSpan sdktrace.ReadOnlySpan

	for _, s := range rec.Ended() {
		if s.Name() == "mqtt-subscribe" {
			subSpan = s
		}
	}

	require.NotNil(t, subSpan)
	assert.Empty(t, subSpan.Links())
	assert.NotEqual(t, producer.SpanContext().TraceID(), subSpan.SpanContext().TraceID())
}

// TestSpanAttributes_SkippedWhenNotRecording covers the NeverSample path that a GoFr app with no
// TRACE_EXPORTER runs in: the span is real and carries a valid ID (correlation IDs depend on that),
// but it records nothing, so the attributes are skipped rather than built and discarded.
func TestSpanAttributes_SkippedWhenNotRecording(t *testing.T) {
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()))
	otel.SetTracerProvider(tp)

	t.Cleanup(func() {
		require.NoError(t, tp.Shutdown(context.Background()))
	})

	for name, start := range map[string]func(context.Context, string) (context.Context, trace.Span){
		"publish":   startPublishSpan,
		"subscribe": startSubscribeSpan,
	} {
		t.Run(name, func(t *testing.T) {
			_, span := start(context.Background(), "test-topic")
			defer span.End()

			assert.False(t, span.IsRecording())
			assert.True(t, span.SpanContext().IsValid(), "correlation IDs depend on a valid span context")
		})
	}
}

// BenchmarkStartPublishSpan guards the allocation cost of instrumenting a publish.
//
// TracingOff is the default deployment — an SDK provider sampling NeverSample — and is the one that
// matters: it is on the path of every published message whether or not the app traces anything.
// Building the messaging attributes unconditionally costs 456 B and 5 allocations more per message
// there, all of it discarded at the sampler, which is why setMessagingAttributes guards on
// IsRecording. Against a live broker one publish is ~7.9 us at QoS 0, so this is invisible in
// latency and shows up only as garbage.
func BenchmarkStartPublishSpan(b *testing.B) {
	for name, sampler := range map[string]sdktrace.Sampler{
		"TracingOff": sdktrace.NeverSample(),
		"TracingOn":  sdktrace.AlwaysSample(),
	} {
		b.Run(name, func(b *testing.B) {
			tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sampler))
			otel.SetTracerProvider(tp)

			b.Cleanup(func() {
				require.NoError(b, tp.Shutdown(context.Background()))
			})

			ctx := context.Background()

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				_, span := startPublishSpan(ctx, "bench-topic")
				span.End()
			}
		})
	}
}
