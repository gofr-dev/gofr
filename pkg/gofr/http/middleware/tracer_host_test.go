package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
	otelTrace "go.opentelemetry.io/otel/trace"

	gofrhttp "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/internal/quietspan"
	"gofr.dev/pkg/gofr/internal/stablemw"
)

// quietProvider installs a NeverSample provider and a W3C propagator for the test, registered with
// quietspan when register is set.
func quietProvider(t *testing.T, register bool) *trace.TracerProvider {
	t.Helper()

	prevTP, prevProp := otel.GetTracerProvider(), otel.GetTextMapPropagator()

	tp := trace.NewTracerProvider(trace.WithSampler(trace.NeverSample()))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	if register {
		quietspan.Register(tp)
	}

	t.Cleanup(func() {
		quietspan.Forget(tp)
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})

	return tp
}

type hostSeen struct {
	inner, outerAfter seenSpan
}

// serveHosted serves GET /x through a mux-mode router whose chain is outer (when not nil), then
// Tracer, and reports the span the handler saw and the one outer's request carried after the
// handler returned.
func serveHosted(t *testing.T, outer func(http.Handler) http.Handler, header map[string]string) hostSeen {
	t.Helper()
	t.Setenv(gofrhttp.RouterEnvVar, gofrhttp.MatcherMux)

	var seen hostSeen

	r := gofrhttp.NewRouter()

	if outer != nil {
		r.Use(func(next http.Handler) http.Handler {
			h := outer(next)

			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				h.ServeHTTP(w, req)

				s := otelTrace.SpanFromContext(req.Context())
				seen.outerAfter = seenSpan{sc: s.SpanContext(), recording: s.IsRecording(), provider: s.TracerProvider()}
			})
		})
	}

	r.Use(stablemw.Mark(Tracer))

	r.Add(http.MethodGet, "/x", http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		s := otelTrace.SpanFromContext(req.Context())
		seen.inner = seenSpan{sc: s.SpanContext(), recording: s.IsRecording(), provider: s.TracerProvider()}
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody)
	for k, v := range header {
		req.Header.Set(k, v)
	}

	r.ServeHTTP(httptest.NewRecorder(), req)

	return seen
}

func passThroughMW(next http.Handler) http.Handler { return next }

// With Tracer the first layer the router calls, the span lands in the router's own context node.
// The handler sees what the SDK path gives it: same trace ID, flags, state, remoteness, recording
// state and provider, and a span ID of its own.
func TestTracer_HostedSpanMatchesSDKPath(t *testing.T) {
	for name, header := range map[string]map[string]string{
		"no parent": nil,
		"parent": {
			"traceparent": "00-" + w3cFixtureTraceID + "-" + w3cFixtureParentSpan + "-01",
			"tracestate":  "k=v",
		},
	} {
		t.Run(name, func(t *testing.T) {
			tp := quietProvider(t, false)
			want := serveHosted(t, nil, header)

			quietspan.Register(tp)

			got := serveHosted(t, nil, header)

			require.True(t, got.inner.sc.IsValid())
			assert.Equal(t, want.inner.sc.TraceFlags(), got.inner.sc.TraceFlags())
			assert.Equal(t, want.inner.sc.TraceState().String(), got.inner.sc.TraceState().String())
			assert.Equal(t, want.inner.sc.IsRemote(), got.inner.sc.IsRemote())
			assert.Equal(t, want.inner.recording, got.inner.recording)
			assert.Same(t, tp, got.inner.provider)

			if header != nil {
				assert.Equal(t, w3cFixtureTraceID, got.inner.sc.TraceID().String())
				assert.NotEqual(t, w3cFixtureParentSpan, got.inner.sc.SpanID().String())
			}
		})
	}
}

// The hazard a shared node carries: whatever holds the request outside Tracer would see the span
// once Tracer put it into that node, where before it saw none. So the router arms the slot only
// when Tracer is the handler it calls. A layer outside Tracer, however thin, leaves it unarmed, and
// that layer's request carries no span after the handler returns, exactly as on the SDK path.
func TestTracer_OuterLayerNeverSeesTheSpan(t *testing.T) {
	for _, register := range []bool{false, true} {
		quietProvider(t, register)

		got := serveHosted(t, passThroughMW, nil)

		assert.True(t, got.inner.sc.IsValid(), "register=%t", register)
		assert.False(t, got.outerAfter.sc.IsValid(), "register=%t: the outer layer's context has no span", register)
	}
}

// Two Tracers in one chain: the second cannot claim the claimed slot and starts its span the
// ordinary way, as a child in the same trace.
func TestTracer_NestedTracerStartsAChild(t *testing.T) {
	quietProvider(t, true)

	var outerSC, innerSC otelTrace.SpanContext

	h := Tracer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		outerSC = otelTrace.SpanContextFromContext(req.Context())

		Tracer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
			innerSC = otelTrace.SpanContextFromContext(req.Context())
		})).ServeHTTP(w, req)
	}))

	t.Setenv(gofrhttp.RouterEnvVar, gofrhttp.MatcherMux)

	r := gofrhttp.NewRouter()
	r.Use(func(http.Handler) http.Handler { return h })
	r.Add(http.MethodGet, "/x", http.NotFoundHandler())

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody))

	require.True(t, outerSC.IsValid())
	require.True(t, innerSC.IsValid())
	assert.Equal(t, outerSC.TraceID(), innerSC.TraceID())
	assert.NotEqual(t, outerSC.SpanID(), innerSC.SpanID())
}

// Hosting the span saves the span's own context node and the request copy that carried it.
func TestTracer_HostedSpanAllocations(t *testing.T) {
	if raceEnabled {
		t.Skip("allocation counts are not stable under the race detector")
	}

	quietProvider(t, true)
	t.Setenv(gofrhttp.RouterEnvVar, gofrhttp.MatcherMux)

	build := func(outer bool) *gofrhttp.Router {
		r := gofrhttp.NewRouter()
		if outer {
			r.Use(stablemw.Mark(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) { next.ServeHTTP(w, req) })
			}))
		}

		// Marked, as GoFr's own server marks them, so each chain is built once.
		r.Use(stablemw.Mark(Tracer))
		r.Add(http.MethodGet, "/x", http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

		return r
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody)
	w := httptest.NewRecorder()

	hosted, unhosted := build(false), build(true)
	withHost := testing.AllocsPerRun(200, func() { hosted.ServeHTTP(w, req) })
	without := testing.AllocsPerRun(200, func() { unhosted.ServeHTTP(w, req) })

	assert.InDelta(t, without-2, withHost, 0.01, "hosted %.0f, unhosted %.0f", withHost, without)
}
