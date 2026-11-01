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

	"gofr.dev/pkg/gofr/internal/quietspan"
)

type seenSpan struct {
	sc        otelTrace.SpanContext
	recording bool
	provider  otelTrace.TracerProvider
}

func serveQuietTraced(t *testing.T, header map[string]string) seenSpan {
	t.Helper()

	var seen seenSpan

	h := Tracer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		s := otelTrace.SpanFromContext(r.Context())
		seen = seenSpan{sc: s.SpanContext(), recording: s.IsRecording(), provider: s.TracerProvider()}
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody)
	for k, v := range header {
		req.Header.Set(k, v)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return seen
}

// With GoFr's NeverSample provider registered, the handler sees the span the SDK path gives it:
// same trace ID, flags, trace state, remoteness, recording state and provider.
func TestTracer_QuietSpanMatchesSDKPath(t *testing.T) {
	tp := trace.NewTracerProvider(trace.WithSampler(trace.NeverSample()))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	t.Cleanup(func() { quietspan.Forget(tp) })

	for name, header := range map[string]map[string]string{
		"no parent": nil,
		"parent": {
			"traceparent": "00-" + w3cFixtureTraceID + "-" + w3cFixtureParentSpan + "-01",
			"tracestate":  "k=v",
		},
	} {
		t.Run(name, func(t *testing.T) {
			quietspan.Forget(tp)

			want := serveQuietTraced(t, header)

			quietspan.Register(tp)
			require.True(t, quietspan.Owns(t.Context(), tp))
			got := serveQuietTraced(t, header)

			if header != nil {
				assert.Equal(t, w3cFixtureTraceID, got.sc.TraceID().String())
				assert.Equal(t, want.sc.TraceID(), got.sc.TraceID())
			}

			assert.True(t, got.sc.IsValid())
			assert.NotEqual(t, want.sc.SpanID(), got.sc.SpanID())
			assert.Equal(t, want.sc.TraceFlags(), got.sc.TraceFlags())
			assert.Equal(t, want.sc.TraceState(), got.sc.TraceState())
			assert.Equal(t, want.sc.IsRemote(), got.sc.IsRemote())
			assert.False(t, got.recording)
			assert.Equal(t, want.recording, got.recording)
			assert.Same(t, want.provider, got.provider)
		})
	}
}
