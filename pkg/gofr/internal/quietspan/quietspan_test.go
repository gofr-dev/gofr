package quietspan

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func neverSample(t *testing.T) *sdktrace.TracerProvider {
	t.Helper()

	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()))

	t.Cleanup(func() { Forget(tp) })

	return tp
}

func parents(t *testing.T) map[string]context.Context {
	t.Helper()

	prop := propagation.TraceContext{}
	inbound := func(tp, ts string) context.Context {
		c := propagation.MapCarrier{"traceparent": tp}
		if ts != "" {
			c["tracestate"] = ts
		}

		return prop.Extract(context.Background(), c)
	}

	return map[string]context.Context{
		"root":      context.Background(),
		"sampled":   inbound("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01", "k=v,a=b"),
		"unsampled": inbound("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-00", ""),
		"flags":     inbound("00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-03", "x=1"),
	}
}

// Start gives the span the SDK's tracer gives under the same parent: the same trace ID (or a
// new valid one), flags, trace state, remoteness, recording state and provider, and a new
// valid span ID.
func TestStart_SameSpanAsSDK(t *testing.T) {
	tp := neverSample(t)
	Register(tp)
	require.True(t, Owns(context.Background(), tp))

	for name, parent := range parents(t) {
		t.Run(name, func(t *testing.T) {
			sdkCtx, sdkSpan := tp.Tracer("t").Start(parent, "n")
			ctx := Start(parent, tp)
			got := trace.SpanFromContext(ctx)
			want, have := sdkSpan.SpanContext(), got.SpanContext()
			psc := trace.SpanContextFromContext(parent)

			if psc.TraceID().IsValid() {
				assert.Equal(t, want.TraceID(), have.TraceID())
			} else {
				assert.True(t, have.TraceID().IsValid())
				assert.NotEqual(t, want.TraceID(), have.TraceID())
			}

			assert.True(t, have.SpanID().IsValid())
			assert.NotEqual(t, psc.SpanID(), have.SpanID())
			assert.Equal(t, want.TraceFlags(), have.TraceFlags())
			assert.Equal(t, want.TraceState(), have.TraceState())
			assert.Equal(t, want.IsRemote(), have.IsRemote())
			assert.Equal(t, sdkSpan.IsRecording(), got.IsRecording())
			assert.Same(t, sdkSpan.TracerProvider(), got.TracerProvider())
			assert.Equal(t, want, trace.SpanContextFromContext(sdkCtx))
			assert.Equal(t, have, trace.SpanContextFromContext(ctx))

			// A span started under either continues the same trace from it.
			_, child := tp.Tracer("t").Start(ctx, "c")
			assert.Equal(t, have.TraceID(), child.SpanContext().TraceID())
			assert.Equal(t, have.TraceFlags(), child.SpanContext().TraceFlags())

			// The span's methods are no-ops, as the SDK's non-recording span's are.
			got.SetAttributes()
			got.SetName("x")
			got.End()
			assert.False(t, got.IsRecording())
		})
	}
}

type ctxKey struct{}

func TestStart_PassesContextThrough(t *testing.T) {
	tp := neverSample(t)
	Register(tp)

	parent, cancel := context.WithCancel(context.WithValue(context.Background(), ctxKey{}, "v"))
	ctx := Start(parent, tp)

	assert.Equal(t, "v", ctx.Value(ctxKey{}))
	cancel()
	<-ctx.Done()
	assert.Equal(t, context.Canceled, ctx.Err())
}

func TestOwns(t *testing.T) {
	tp := neverSample(t)
	other := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()))

	assert.False(t, Owns(context.Background(), tp), "not registered")

	Register(tp)
	assert.True(t, Owns(context.Background(), tp))
	assert.False(t, Owns(context.Background(), other))
	assert.False(t, Owns(context.Background(), trace.NewNoopTracerProvider())) //nolint:staticcheck // any other provider type

	recorder := sdktrace.NewTracerProvider()
	recCtx, rec := recorder.Tracer("t").Start(context.Background(), "p")
	require.True(t, rec.IsRecording())
	assert.False(t, Owns(recCtx, tp), "a recording parent is the SDK's to count")

	Forget(other)
	assert.True(t, Owns(context.Background(), tp), "Forget leaves another provider registered")

	Forget(tp)
	assert.False(t, Owns(context.Background(), tp))

	Register(tp)
	Register(nil)
	assert.False(t, Owns(context.Background(), tp))
}

func TestRegister_RefusesUnderSelfObservability(t *testing.T) {
	for _, k := range []string{"OTEL_GO_X_OBSERVABILITY", "OTEL_GO_X_SELF_OBSERVABILITY"} {
		t.Run(k, func(t *testing.T) {
			t.Setenv(k, "TRUE")

			tp := neverSample(t)
			Register(tp)
			assert.False(t, Owns(context.Background(), tp))
		})
	}
}

func TestSpanKeyProbed(t *testing.T) {
	require.NotNil(t, spanKey)
	assert.True(t, roundTrips())
}

// Start allocates once, for the span that is also the context.
func TestStart_OneAllocation(t *testing.T) {
	if raceEnabled {
		t.Skip("allocation counts are not stable under the race detector")
	}

	tp := neverSample(t)
	Register(tp)

	parent := parents(t)["sampled"]

	// The result is kept, as Tracer keeps it: a discarded span from an inlined Start would never
	// reach the heap.
	assert.InDelta(t, 1, testing.AllocsPerRun(100, func() { startSink = Start(parent, tp) }), 0.01)
}

var startSink context.Context //nolint:gochecknoglobals // keeps Start's result reachable in TestStart_OneAllocation.

// hostCtx is a request context holding a Host, as GoFr's router builds one.
type hostCtx struct {
	context.Context
	h Host
}

func (c *hostCtx) QuietHost() *Host { return &c.h }

func (c *hostCtx) Value(key any) any {
	if v, ok := c.h.Lookup(key); ok {
		return v
	}

	return c.Context.Value(key)
}

// The hazard of putting the span into a context that already exists: anything holding that
// context before Tracer claimed the slot sees the span afterwards, where a context of the span's
// own would have left it untouched. This is why only an armed slot can be claimed, and why the
// router arms it only when Tracer is the handler it calls.
func TestHost_ClaimIsVisibleToEveryHolder(t *testing.T) {
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()))
	ctx := &hostCtx{Context: context.Background()}
	heldOutside := context.Context(ctx)

	assert.False(t, Claim(ctx, tp), "an unarmed slot is never claimed")
	assert.False(t, trace.SpanContextFromContext(heldOutside).IsValid())

	ctx.h.Arm()
	require.True(t, Claim(ctx, tp))
	assert.False(t, Claim(ctx, tp), "a slot is claimed once")

	assert.True(t, trace.SpanContextFromContext(heldOutside).IsValid(), "the holder now sees the span")
	assert.Same(t, tp, trace.SpanFromContext(ctx).TracerProvider())
	assert.False(t, trace.SpanFromContext(ctx).IsRecording())
	assert.False(t, Claim(context.Background(), tp), "a context without a slot")
}
