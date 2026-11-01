// Package quietspan starts the request span GoFr's NeverSample provider would start, without the
// SDK's per-span work.
//
// With no TRACE_EXPORTER, GoFr installs an SDK TracerProvider whose sampler is NeverSample. Its
// Start allocates a copy of the start options' attributes for the sampler, which ignores them;
// boxes a non-recording span; and stores that span in a context value. All three are thrown away
// unread: the span records nothing, and only its SpanContext is ever used, for X-Correlation-ID and
// the log's trace and span IDs. Start produces the same span state in one allocation.
//
// The span is what the SDK produces for that provider: a new span ID; the parent's trace ID, or
// a new one when the parent has none; the parent's trace flags without the sampled bit; the
// parent's trace state; not remote; IsRecording false and every method a no-op; TracerProvider
// returning the SDK provider. IDs come from the same generator the SDK's default uses
// (math/rand/v2, the same byte layout, the same retry on an all-zero ID).
package quietspan

import (
	"context"
	"encoding/binary"
	"math/rand/v2"
	"os"
	"strings"
	"sync/atomic"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
)

// owned is the NeverSample provider GoFr built, if Register accepted it.
var owned atomic.Pointer[sdktrace.TracerProvider] //nolint:gochecknoglobals // the one provider GoFr installs.

// spanKey is the key trace.SpanFromContext reads. otel does not export it, so it is learned once
// by asking SpanFromContext about a context that records the key it is asked for.
var spanKey = probeSpanKey() //nolint:gochecknoglobals // set once from otel's own lookup; read-only.

type keyProbe struct {
	context.Context
	key any
}

func (p *keyProbe) Value(key any) any {
	p.key = key

	return nil
}

func probeSpanKey() any {
	p := &keyProbe{Context: context.Background()}
	_ = trace.SpanFromContext(p)

	return p.key
}

// Register records tp, a provider whose only sampler is NeverSample and which has no span
// processor, as one Start may stand in for. It refuses, leaving Owns false, when the SDK's
// self-observability is switched on (the SDK then counts every span it starts) or when the span
// key could not be learned or does not round-trip.
func Register(tp *sdktrace.TracerProvider) {
	if tp == nil || selfObservability() || !roundTrips() {
		owned.Store(nil)

		return
	}

	owned.Store(tp)
}

// Forget undoes Register for tp, as its shutdown must: a shut-down provider's tracers are the
// SDK's to answer for.
func Forget(tp *sdktrace.TracerProvider) {
	owned.CompareAndSwap(tp, nil)
}

// Owns reports whether Start may stand in for tp's tracer under ctx: tp is the registered
// provider, and ctx carries no recording span (the SDK counts a recording parent's children).
func Owns(ctx context.Context, tp trace.TracerProvider) bool {
	p, ok := tp.(*sdktrace.TracerProvider)

	return ok && p != nil && p == owned.Load() && !trace.SpanFromContext(ctx).IsRecording()
}

// selfObservability mirrors the SDK's experimental OTEL_GO_X_OBSERVABILITY switch, under both of
// the names it reads.
func selfObservability() bool {
	for _, k := range []string{"OTEL_GO_X_OBSERVABILITY", "OTEL_GO_X_SELF_OBSERVABILITY"} {
		if strings.EqualFold(os.Getenv(k), "true") {
			return true
		}
	}

	return false
}

func roundTrips() bool {
	if spanKey == nil {
		return false
	}

	s := &span{Context: context.Background()}

	return trace.SpanFromContext(s) == trace.Span(s)
}

// Start returns ctx carrying a new span from tp, as tp's tracer would start it under ctx. tp
// must be the registered provider.
func Start(ctx context.Context, tp *sdktrace.TracerProvider) context.Context {
	psc := trace.SpanContextFromContext(ctx)

	tid := psc.TraceID()
	if !tid.IsValid() {
		tid = newTraceID()
	}

	return &span{
		Context: ctx,
		tp:      tp,
		sc: trace.NewSpanContext(trace.SpanContextConfig{
			TraceID:    tid,
			SpanID:     newSpanID(),
			TraceFlags: psc.TraceFlags() &^ trace.FlagsSampled,
			TraceState: psc.TraceState(),
		}),
	}
}

func newTraceID() trace.TraceID {
	var tid trace.TraceID

	for !tid.IsValid() {
		binary.NativeEndian.PutUint64(tid[:8], rand.Uint64()) //nolint:gosec // the SDK's own generator.
		binary.NativeEndian.PutUint64(tid[8:], rand.Uint64()) //nolint:gosec // the SDK's own generator.
	}

	return tid
}

func newSpanID() trace.SpanID {
	var sid trace.SpanID

	for !sid.IsValid() {
		binary.NativeEndian.PutUint64(sid[:], rand.Uint64()) //nolint:gosec // the SDK's own generator.
	}

	return sid
}

// span is both the request context and the non-recording span it carries.
type span struct {
	embedded.Span
	context.Context

	tp *sdktrace.TracerProvider
	sc trace.SpanContext
}

var _ trace.Span = (*span)(nil)

// Value answers the span key with the span itself, as trace.ContextWithSpan's value would.
func (s *span) Value(key any) any {
	if key == spanKey {
		return trace.Span(s)
	}

	return s.Context.Value(key)
}

func (s *span) SpanContext() trace.SpanContext        { return s.sc }
func (*span) IsRecording() bool                       { return false }
func (*span) SetStatus(codes.Code, string)            {}
func (*span) SetError(bool)                           {}
func (*span) SetAttributes(...attribute.KeyValue)     {}
func (*span) End(...trace.SpanEndOption)              {}
func (*span) RecordError(error, ...trace.EventOption) {}
func (*span) AddEvent(string, ...trace.EventOption)   {}
func (*span) AddLink(trace.Link)                      {}
func (*span) SetName(string)                          {}
func (s *span) TracerProvider() trace.TracerProvider  { return s.tp }
