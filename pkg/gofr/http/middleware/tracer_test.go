package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	otelTrace "go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/version"
)

// W3C TraceContext fixture values reused across the propagation tests.
// Sourced from the W3C Trace Context spec's example traceparent header
// (https://www.w3.org/TR/trace-context/).
const (
	w3cFixtureTraceID    = "4bf92f3577b34da6a3ce929d0e0e4736"
	w3cFixtureParentSpan = "00f067aa0ba902b7" // spellchecker:disable-line
)

type MockHandlerForTracing struct{}

// ServeHTTP is used for testing if the request context has traceId.
func (*MockHandlerForTracing) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	traceID := otelTrace.SpanFromContext(req.Context()).SpanContext().TraceID().String()
	_, _ = w.Write([]byte(traceID))
}

func TestTrace(_ *testing.T) {
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	handler := Tracer(&MockHandlerForTracing{})
	req := httptest.NewRequest(http.MethodGet, "/dummy", http.NoBody)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)
}

// installPropagators wires up the W3C TraceContext + Baggage propagator
// pair that production GoFr installs in initTracer (otel.go). Returns a
// cleanup that restores the previous propagator so tests do not leak
// global state.
func installPropagators(t *testing.T) {
	t.Helper()

	prev := otel.GetTextMapPropagator()

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	t.Cleanup(func() { otel.SetTextMapPropagator(prev) })
}

// TestTracePropagation_Inbound asserts that the Tracer middleware
// extracts an incoming W3C traceparent header and the handler observes
// a span context whose trace ID matches the parent.
//
// This is the contract every PR touching the tracing path must keep:
// distributed traces stay continuous across hops.
func TestTracePropagation_Inbound(t *testing.T) {
	installPropagators(t)

	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	var got otelTrace.SpanContext

	handler := Tracer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = otelTrace.SpanContextFromContext(r.Context())
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/dummy", http.NoBody)
	req.Header.Set("Traceparent", "00-"+w3cFixtureTraceID+"-"+w3cFixtureParentSpan+"-01")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.True(t, got.IsValid(), "no span context observed in handler")
	assert.Equal(t, w3cFixtureTraceID, got.TraceID().String(),
		"trace ID did not propagate from inbound traceparent")
	assert.True(t, got.IsSampled(),
		"sampled flag did not propagate from inbound traceparent (sampled=01)")
}

// TestTracePropagation_NoInboundHeader asserts that a request without
// a traceparent header gets a new (valid) trace ID assigned by the
// SDK — the middleware must not crash or skip span creation.
func TestTracePropagation_NoInboundHeader(t *testing.T) {
	installPropagators(t)

	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	var got otelTrace.SpanContext

	handler := Tracer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = otelTrace.SpanContextFromContext(r.Context())
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/dummy", http.NoBody)

	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.True(t, got.IsValid(), "expected a new valid span context when no inbound header")
	assert.NotEqual(t, "00000000000000000000000000000000", got.TraceID().String())
}

// TestTracePropagation_BaggageInbound asserts that W3C Baggage from the
// inbound request is parsed onto the request context and visible to the
// handler. Required for downstream services to see the baggage members
// the upstream set.
func TestTracePropagation_BaggageInbound(t *testing.T) {
	installPropagators(t)

	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	var got baggage.Baggage

	handler := Tracer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = baggage.FromContext(r.Context())
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/dummy", http.NoBody)
	req.Header.Set("Baggage", "tenant=acme,region=us-east")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.Equal(t, 2, got.Len(), "baggage members were not extracted")
	assert.Equal(t, "acme", got.Member("tenant").Value())
	assert.Equal(t, "us-east", got.Member("region").Value())
}

// TestTracePropagation_Outbound asserts that the W3C propagator that
// GoFr installs is able to inject a traceparent header onto an
// outbound request. This is the same code path the HTTP service client
// uses (service/new.go calls otel.GetTextMapPropagator().Inject) — so
// if this test passes, outbound services carry the trace.
func TestTracePropagation_Outbound(t *testing.T) {
	installPropagators(t)

	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	// Construct a context carrying a known sampled span context — what a
	// handler would have after the Tracer middleware extracted an
	// inbound traceparent.
	scfg := otelTrace.SpanContextConfig{
		TraceID:    otelTrace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     otelTrace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: otelTrace.FlagsSampled,
		Remote:     true,
	}
	ctx := otelTrace.ContextWithSpanContext(context.Background(), otelTrace.NewSpanContext(scfg))

	bag, err := baggage.Parse("tenant=acme")
	require.NoError(t, err)

	ctx = baggage.ContextWithBaggage(ctx, bag)

	outbound := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://downstream/api", http.NoBody)
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(outbound.Header))

	tp1 := outbound.Header.Get("Traceparent")
	require.NotEmpty(t, tp1, "outbound request missing traceparent header")
	assert.Contains(t, tp1, "0102030405060708090a0b0c0d0e0f10",
		"outbound traceparent does not carry the parent trace ID")

	assert.Equal(t, "tenant=acme", outbound.Header.Get("Baggage"),
		"outbound request did not inject baggage")
}

// TestTracePropagation_BaggageRoundTrip exercises a full inbound→handler→outbound
// loop and asserts every baggage member set upstream survives the trip. This is
// the contract Phase-C PR-17 (drop otelhttp.NewHandler wrap) must keep: when we
// stop relying on otelhttp's propagation glue and depend solely on the GoFr
// tracer middleware + W3C propagator pair installed in PR-15, baggage round-trip
// must remain byte-for-byte stable on the wire.
func TestTracePropagation_BaggageRoundTrip(t *testing.T) {
	installPropagators(t)

	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	cases := []struct {
		name    string
		header  string
		members map[string]string
	}{
		{
			name:    "single member",
			header:  "tenant=acme",
			members: map[string]string{"tenant": "acme"},
		},
		{
			name:    "multiple members",
			header:  "tenant=acme,region=us-east,version=v1",
			members: map[string]string{"tenant": "acme", "region": "us-east", "version": "v1"},
		},
		{
			name:    "values with hyphens and dots",
			header:  "service=user-api,env=prod.eu",
			members: map[string]string{"service": "user-api", "env": "prod.eu"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var observed baggage.Baggage

			handler := Tracer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				observed = baggage.FromContext(r.Context())
			}))

			inbound := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
			inbound.Header.Set("Baggage", tc.header)

			handler.ServeHTTP(httptest.NewRecorder(), inbound)

			require.Equal(t, len(tc.members), observed.Len(),
				"handler did not observe all baggage members from inbound header %q", tc.header)

			for k, want := range tc.members {
				got := observed.Member(k).Value()
				assert.Equal(t, want, got, "inbound baggage member %q lost or rewritten", k)
			}

			// Inject the same baggage back into an outbound request — round-trip.
			outbound := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://downstream/api", http.NoBody)
			ctx := baggage.ContextWithBaggage(context.Background(), observed)
			otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(outbound.Header))

			outHdr := outbound.Header.Get("Baggage")
			require.NotEmpty(t, outHdr, "outbound request missing baggage header")

			parsed, err := baggage.Parse(outHdr)
			require.NoError(t, err, "outbound baggage header is not parseable: %q", outHdr)

			for k, want := range tc.members {
				got := parsed.Member(k).Value()
				assert.Equal(t, want, got, "outbound baggage member %q lost or rewritten", k)
			}
		})
	}
}

// TestTracer_EmitsOTelHTTPSemconvAttributes asserts that the Tracer
// middleware emits the OTel HTTP semconv ≥ v1.21 attribute keys:
// http.request.method, http.route, http.response.status_code. A future
// PR that regresses these back to the deprecated v1.4-era keys
// (http.method, http.status_code) breaks downstream dashboards built
// against the current semconv and must fail this test.
//
// We also assert the span name follows the OTel HTTP convention
// "METHOD /route-template" (was the static "gofr-router" before this
// stack removed the otelhttp.NewHandler wrap).
func TestTracer_EmitsOTelHTTPSemconvAttributes(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(rec))
	otel.SetTracerProvider(tp)

	t.Cleanup(func() { _ = tp.Shutdown(t.Context()) })

	router := mux.NewRouter()
	router.Handle("/users/{id}", Tracer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) },
	))).Methods(http.MethodGet)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users/42", http.NoBody)
	router.ServeHTTP(httptest.NewRecorder(), req)

	spans := rec.Ended()
	require.Len(t, spans, 1, "expected exactly one span recorded")

	got := spans[0]

	// Span name must follow "METHOD /route-template" semconv guidance —
	// concrete URL (/users/42) would explode cardinality.
	assert.Equal(t, "GET /users/{id}", got.Name(),
		"span name must follow OTel HTTP semconv 'METHOD /route'")

	attrs := make(map[attribute.Key]attribute.Value, len(got.Attributes()))
	for _, kv := range got.Attributes() {
		attrs[kv.Key] = kv.Value
	}

	require.Contains(t, attrs, attribute.Key("http.request.method"),
		"missing http.request.method (current semconv); legacy http.method is deprecated")
	require.Contains(t, attrs, attribute.Key("http.route"),
		"missing http.route")
	require.Contains(t, attrs, attribute.Key("http.response.status_code"),
		"missing http.response.status_code (current semconv); legacy http.status_code is deprecated")

	assert.NotContains(t, attrs, attribute.Key("http.method"),
		"deprecated http.method key must not be emitted alongside http.request.method")
	assert.NotContains(t, attrs, attribute.Key("http.status_code"),
		"deprecated http.status_code key must not be emitted alongside http.response.status_code")

	assert.Equal(t, "GET", attrs[attribute.Key("http.request.method")].AsString())
	assert.Equal(t, "/users/{id}", attrs[attribute.Key("http.route")].AsString())
	assert.Equal(t, int64(http.StatusCreated), attrs[attribute.Key("http.response.status_code")].AsInt64())
}

// ---------------------------------------------------------------------------
// Characterization suite for the Tracer HTTP middleware.
//
// Every identifier below is prefixed with `tracerChar` so this file can be
// merged with sibling _test.go additions in the same package without
// collisions. Nothing here is aspirational: every assertion pins the CURRENT
// behavior of pkg/gofr/http/middleware/tracer.go as observed against the
// unmodified source.
// ---------------------------------------------------------------------------

// tracerCharScopeName returns the exact instrumentation-scope (tracer) name
// the middleware resolves at chain-build time.
func tracerCharScopeName() string { return "gofr-" + version.Framework }

// tracerCharInstallRecordingTP installs an sdktrace provider with an in-memory
// span recorder and restores the previously installed global provider on
// cleanup. The provider MUST be installed before Tracer() is called because
// Tracer resolves otel.Tracer(...) once at chain-build time.
func tracerCharInstallRecordingTP(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	prev := otel.GetTracerProvider()
	rec := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(trace.WithSpanProcessor(rec))

	otel.SetTracerProvider(tp)

	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())

		otel.SetTracerProvider(prev)
	})

	return rec
}

// tracerCharInstallNeverSampleTP installs exactly the provider GoFr's
// initTracer builds when no exporter is configured (otel.go), plus a recorder
// so tests can prove nothing is ever exported.
func tracerCharInstallNeverSampleTP(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	prev := otel.GetTracerProvider()
	rec := tracetest.NewSpanRecorder()
	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.NeverSample()),
		trace.WithSpanProcessor(rec),
	)

	otel.SetTracerProvider(tp)

	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())

		otel.SetTracerProvider(prev)
	})

	return rec
}

// tracerCharAttrs returns the span's attributes sorted by key so an exact
// expected slice can be compared — an added, renamed or retyped attribute
// then fails the comparison.
func tracerCharAttrs(s trace.ReadOnlySpan) []attribute.KeyValue {
	attrs := s.Attributes()
	out := make([]attribute.KeyValue, len(attrs))
	copy(out, attrs)

	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })

	return out
}

// tracerCharLogger captures the RequestLog values emitted by the Logging
// middleware.
type tracerCharLogger struct {
	logs   []*RequestLog
	errors []*RequestLog
}

func (l *tracerCharLogger) Log(args ...any) {
	if rl, ok := args[0].(*RequestLog); ok {
		l.logs = append(l.logs, rl)
	}
}

func (l *tracerCharLogger) Error(args ...any) {
	if rl, ok := args[0].(*RequestLog); ok {
		l.errors = append(l.errors, rl)
	}
}

// last returns the single RequestLog captured on either channel.
func (l *tracerCharLogger) last() *RequestLog {
	if len(l.errors) > 0 {
		return l.errors[len(l.errors)-1]
	}

	if len(l.logs) > 0 {
		return l.logs[len(l.logs)-1]
	}

	return nil
}

// Test_TracerContract_InstrumentationScopeName pins the exact tracer name.
func Test_TracerContract_InstrumentationScopeName(t *testing.T) {
	rec := tracerCharInstallRecordingTP(t)

	handler := Tracer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/scope", http.NoBody)

	handler.ServeHTTP(httptest.NewRecorder(), req)

	spans := rec.Ended()
	require.Len(t, spans, 1)

	assert.Equal(t, "gofr-dev", tracerCharScopeName(),
		"version.Framework changed; the scope name below moves with it")
	assert.Equal(t, tracerCharScopeName(), spans[0].InstrumentationScope().Name)
	assert.Empty(t, spans[0].InstrumentationScope().Version,
		"middleware passes no WithInstrumentationVersion")
	assert.Empty(t, spans[0].InstrumentationScope().SchemaURL)
}

// Test_TracerContract_SpanName pins "METHOD /route-template" across the mux
// route-resolution paths.
func Test_TracerContract_SpanName(t *testing.T) {
	tests := []struct {
		name  string
		build func(h http.Handler) http.Handler
		// method/target of the inbound request.
		method string
		target string
		want   string
	}{
		{
			name: "mux matched route uses path template",
			build: func(h http.Handler) http.Handler {
				r := mux.NewRouter()
				r.Handle("/users/{id}", h).Methods(http.MethodGet)

				return r
			},
			method: http.MethodGet,
			target: "/users/42",
			want:   "GET /users/{id}",
		},
		{
			name: "mux PathPrefix-only route still yields a template",
			build: func(h http.Handler) http.Handler {
				r := mux.NewRouter()
				r.PathPrefix("/static").Handler(h)

				return r
			},
			method: http.MethodGet,
			target: "/static/css/app.css",
			want:   "GET /static",
		},
		{
			name: "mux route without any path matcher falls back to URL.Path",
			build: func(h http.Handler) http.Handler {
				r := mux.NewRouter()
				r.Methods(http.MethodGet).Handler(h)

				return r
			},
			method: http.MethodGet,
			target: "/no/path/matcher",
			want:   "GET /no/path/matcher",
		},
		{
			name: "unmatched route (404 handler, CurrentRoute nil) falls back to URL.Path",
			build: func(h http.Handler) http.Handler {
				r := mux.NewRouter()
				r.Handle("/known", http.NotFoundHandler())
				r.NotFoundHandler = h

				return r
			},
			method: http.MethodGet,
			target: "/definitely/unknown",
			want:   "GET /definitely/unknown",
		},
		{
			name:   "lowercase inbound method is upper-cased",
			build:  func(h http.Handler) http.Handler { return h },
			method: "get",
			target: "/lower",
			want:   "GET /lower",
		},
		{
			name:   "standalone handler (no mux) uses URL.Path",
			build:  func(h http.Handler) http.Handler { return h },
			method: http.MethodDelete,
			target: "/standalone",
			want:   "DELETE /standalone",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := tracerCharInstallRecordingTP(t)

			srv := tc.build(Tracer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
			req := httptest.NewRequestWithContext(t.Context(), tc.method, tc.target, http.NoBody)

			srv.ServeHTTP(httptest.NewRecorder(), req)

			spans := rec.Ended()
			require.Len(t, spans, 1)
			assert.Equal(t, tc.want, spans[0].Name())
			// http.route always equals the route portion of the span name.
			assert.Equal(t, strings.TrimPrefix(tc.want, strings.ToUpper(tc.method)+" "),
				tracerCharAttrs(spans[0])[2].Value.AsString())
		})
	}
}

// Test_TracerContract_ExactAttributeSet pins the complete attribute set —
// keys, values AND value types — for a matched route.
func Test_TracerContract_ExactAttributeSet(t *testing.T) {
	rec := tracerCharInstallRecordingTP(t)

	router := mux.NewRouter()
	router.Handle("/users/{id}", Tracer(http.HandlerFunc(
		func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusCreated) },
	))).Methods(http.MethodGet)

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users/42", http.NoBody)
	router.ServeHTTP(httptest.NewRecorder(), req)

	spans := rec.Ended()
	require.Len(t, spans, 1)

	want := []attribute.KeyValue{
		attribute.String("http.request.method", "GET"),
		attribute.Int("http.response.status_code", http.StatusCreated),
		attribute.String("http.route", "/users/{id}"),
	}

	got := tracerCharAttrs(spans[0])
	assert.Equal(t, want, got, "complete attribute set (sorted by key) changed")

	// Pin the value TYPES explicitly so an int->string retype fails loudly.
	assert.Equal(t, attribute.STRING, got[0].Value.Type())
	assert.Equal(t, attribute.INT64, got[1].Value.Type())
	assert.Equal(t, attribute.STRING, got[2].Value.Type())
	assert.Equal(t, int64(http.StatusCreated), got[1].Value.AsInt64())
}

// Test_TracerContract_StatusCodeAttribute pins http.response.status_code
// across the ways a handler can (not) set a status.
func Test_TracerContract_StatusCodeAttribute(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		want    int64
	}{
		{
			name:    "explicit WriteHeader 404",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) },
			want:    http.StatusNotFound,
		},
		{
			name:    "explicit WriteHeader 500",
			handler: func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
			want:    http.StatusInternalServerError,
		},
		{
			name:    "body only, implicit 200 via StatusResponseWriter.Write",
			handler: func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("hello")) },
			want:    http.StatusOK,
		},
		{
			name:    "handler does nothing, Status() normalizes 0 to 200",
			handler: func(http.ResponseWriter, *http.Request) {},
			want:    http.StatusOK,
		},
		{
			name: "WriteHeader then Write keeps the explicit status",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte("ok"))
			},
			want: http.StatusAccepted,
		},
		{
			name: "duplicate WriteHeader keeps the first status",
			handler: func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusTeapot)
				w.WriteHeader(http.StatusInternalServerError)
			},
			want: http.StatusTeapot,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := tracerCharInstallRecordingTP(t)

			handler := Tracer(tc.handler)
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/status", http.NoBody)

			handler.ServeHTTP(httptest.NewRecorder(), req)

			spans := rec.Ended()
			require.Len(t, spans, 1)

			attrs := tracerCharAttrs(spans[0])
			require.Len(t, attrs, 3)
			assert.Equal(t, attribute.Key("http.response.status_code"), attrs[1].Key)
			assert.Equal(t, tc.want, attrs[1].Value.AsInt64())
		})
	}
}

// Test_TracerContract_SpanKindStatusAndEvents pins that the middleware emits
// an Internal span with an Unset status and no events — even for a 5xx.
func Test_TracerContract_SpanKindStatusAndEvents(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			rec := tracerCharInstallRecordingTP(t)

			handler := Tracer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
			}))
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/kind", http.NoBody)

			handler.ServeHTTP(httptest.NewRecorder(), req)

			spans := rec.Ended()
			require.Len(t, spans, 1)

			got := spans[0]
			assert.Equal(t, otelTrace.SpanKindInternal, got.SpanKind(),
				"middleware never calls trace.WithSpanKind, so the SDK default (Internal) applies")
			assert.Equal(t, codes.Unset, got.Status().Code, "no SetStatus call anywhere in the middleware")
			assert.Empty(t, got.Status().Description)
			assert.Empty(t, got.Events(), "middleware records no events (no RecordError)")
			assert.Empty(t, got.Links())
			assert.True(t, got.EndTime().After(got.StartTime()) || got.EndTime().Equal(got.StartTime()))
		})
	}
}

// Test_TracerContract_ReusesStatusResponseWriter pins the ResponseWriter
// wrapping behavior in all three chain arrangements.
func Test_TracerContract_ReusesStatusResponseWriter(t *testing.T) {
	t.Run("chained after Logging it reuses the existing StatusResponseWriter", func(t *testing.T) {
		tracerCharInstallRecordingTP(t)

		var fromLogging, inHandler http.ResponseWriter

		capture := func(inner http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fromLogging = w
				inner.ServeHTTP(w, r)
			})
		}

		handler := Logging(LogProbes{}, &tracerCharLogger{})(
			capture(Tracer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				inHandler = w
			}))))

		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/reuse", http.NoBody)
		handler.ServeHTTP(httptest.NewRecorder(), req)

		loggingSRW, ok := fromLogging.(*StatusResponseWriter)
		require.True(t, ok, "Logging must hand a *StatusResponseWriter to the next middleware")

		handlerSRW, ok := inHandler.(*StatusResponseWriter)
		require.True(t, ok)
		assert.Same(t, loggingSRW, handlerSRW, "Tracer must not double-wrap after Logging")
	})

	t.Run("standalone it wraps locally exactly once", func(t *testing.T) {
		tracerCharInstallRecordingTP(t)

		var inHandler http.ResponseWriter

		handler := Tracer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			inHandler = w
		}))

		rr := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/wrap", http.NoBody)
		handler.ServeHTTP(rr, req)

		srw, ok := inHandler.(*StatusResponseWriter)
		require.True(t, ok, "Tracer must wrap when it is not preceded by Logging")
		assert.NotSame(t, http.ResponseWriter(rr), inHandler)
		assert.Same(t, rr, srw.Unwrap(), "exactly one layer of wrapping")
	})

	t.Run("production order (Tracer outer, Logging inner) double-wraps", func(t *testing.T) {
		tracerCharInstallRecordingTP(t)

		var layers []http.ResponseWriter

		// The unwrap chain must be read INSIDE the handler: Logging returns its
		// StatusResponseWriter to a sync.Pool on the way out and nils the
		// embedded ResponseWriter, so the chain is unreadable afterwards.
		handler := Tracer(Logging(LogProbes{}, &tracerCharLogger{})(
			http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				for cur := w; cur != nil; {
					layers = append(layers, cur)

					srw, ok := cur.(*StatusResponseWriter)
					if !ok {
						break
					}

					cur = srw.Unwrap()
				}
			})))

		rr := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/double", http.NoBody)
		handler.ServeHTTP(rr, req)

		// Production wires r.Use(Tracer, Logging, ...), so Tracer is the OUTER
		// middleware: its type assertion always fails, it wraps locally, and
		// Logging then wraps that wrapper again. Two StatusResponseWriter
		// layers are therefore live on every production request — which is what
		// tracer.go's comment now describes.
		require.Len(t, layers, 3, "expected raw recorder wrapped by two StatusResponseWriters")
		assert.IsType(t, &StatusResponseWriter{}, layers[0])
		assert.IsType(t, &StatusResponseWriter{}, layers[1])
		assert.Same(t, rr, layers[2])
	})
}

// Test_TracerContract_InboundTraceparent pins W3C trace-context continuation.
func Test_TracerContract_InboundTraceparent(t *testing.T) {
	installPropagators(t)

	rec := tracerCharInstallRecordingTP(t)

	var handlerBaggage baggage.Baggage

	handler := Tracer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		handlerBaggage = baggage.FromContext(r.Context())
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/inbound", http.NoBody)
	req.Header.Set("Traceparent", "00-"+w3cFixtureTraceID+"-"+w3cFixtureParentSpan+"-01")
	req.Header.Set("Baggage", "tenant=acme,region=us-east")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	spans := rec.Ended()
	require.Len(t, spans, 1)

	got := spans[0]

	// Trace ID is inherited verbatim from the fixture header.
	assert.Equal(t, w3cFixtureTraceID, got.SpanContext().TraceID().String())
	// The span's own ID is fresh and non-deterministic — pin validity/shape only.
	assert.True(t, got.SpanContext().SpanID().IsValid())
	assert.Len(t, got.SpanContext().SpanID().String(), 16)
	assert.NotEqual(t, w3cFixtureParentSpan, got.SpanContext().SpanID().String())

	assert.Equal(t, w3cFixtureParentSpan, got.Parent().SpanID().String())
	assert.Equal(t, w3cFixtureTraceID, got.Parent().TraceID().String())
	assert.True(t, got.Parent().IsRemote(), "parent must be marked remote")
	assert.True(t, got.Parent().IsSampled())

	require.Equal(t, 2, handlerBaggage.Len(), "baggage must survive into the handler context")
	assert.Equal(t, "acme", handlerBaggage.Member("tenant").Value())
	assert.Equal(t, "us-east", handlerBaggage.Member("region").Value())
}

// Test_TracerContract_NoInboundTraceparentIsRoot pins that a request without a
// traceparent produces a fresh root span.
func Test_TracerContract_NoInboundTraceparentIsRoot(t *testing.T) {
	installPropagators(t)

	rec := tracerCharInstallRecordingTP(t)

	handler := Tracer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/root", http.NoBody)

	handler.ServeHTTP(httptest.NewRecorder(), req)

	spans := rec.Ended()
	require.Len(t, spans, 1)

	got := spans[0]
	assert.False(t, got.Parent().IsValid(), "expected a root span")
	assert.Equal(t, zeroTraceID, got.Parent().TraceID().String())
	assert.True(t, got.SpanContext().TraceID().IsValid())
	assert.Len(t, got.SpanContext().TraceID().String(), 32)
	assert.NotEqual(t, zeroTraceID, got.SpanContext().TraceID().String())
}

// Test_TracerContract_NeverSampleKeepsValidIDs is the key regression guard for
// GoFr's default (no exporter configured) deployment: initTracer installs an
// sdktrace provider with NeverSample — NOT a noop provider — precisely so the
// span context handed to handlers still carries a valid TraceID/SpanID that
// X-Correlation-ID and the trace_id log field can use.
func Test_TracerContract_NeverSampleKeepsValidIDs(t *testing.T) {
	installPropagators(t)

	rec := tracerCharInstallNeverSampleTP(t)

	var (
		sc        otelTrace.SpanContext
		recording bool
	)

	handler := Tracer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		span := otelTrace.SpanFromContext(r.Context())
		sc = span.SpanContext()
		recording = span.IsRecording()
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/never", http.NoBody)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.True(t, sc.IsValid(), "NeverSample must still yield a valid span context")
	assert.True(t, sc.TraceID().IsValid())
	assert.True(t, sc.SpanID().IsValid())
	assert.NotEqual(t, zeroTraceID, sc.TraceID().String())
	assert.NotEqual(t, zeroSpanID, sc.SpanID().String())
	assert.Len(t, sc.TraceID().String(), 32)
	assert.Len(t, sc.SpanID().String(), 16)
	assert.False(t, sc.IsSampled(), "NeverSample must clear the sampled flag")
	assert.False(t, recording, "a dropped span must not be recording")
	assert.Empty(t, rec.Ended(), "nothing may be exported under NeverSample")
}

// Test_TracerContract_NeverSampleWithInboundTraceparent pins that NeverSample
// is NOT parent-based: an inbound sampled=01 traceparent still gets dropped,
// though the trace ID is inherited so the trace stays correlatable.
func Test_TracerContract_NeverSampleWithInboundTraceparent(t *testing.T) {
	installPropagators(t)

	rec := tracerCharInstallNeverSampleTP(t)

	var sc otelTrace.SpanContext

	handler := Tracer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		sc = otelTrace.SpanFromContext(r.Context()).SpanContext()
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/never-parent", http.NoBody)
	req.Header.Set("Traceparent", "00-"+w3cFixtureTraceID+"-"+w3cFixtureParentSpan+"-01")

	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.True(t, sc.IsValid())
	assert.Equal(t, w3cFixtureTraceID, sc.TraceID().String())
	assert.False(t, sc.IsSampled())
	assert.Empty(t, rec.Ended())
}

// tracerCharChain builds the two possible orderings of Logging and Tracer.
func tracerCharChain(loggingOuter bool, lg logger, h http.Handler) http.Handler {
	logging := Logging(LogProbes{}, lg)

	if loggingOuter {
		return logging(Tracer(h))
	}

	return Tracer(logging(h))
}

// Test_TracerContract_CorrelationIDWithLogging characterizes the interaction
// between Tracer and the Logging middleware's X-Correlation-ID header and
// trace_id/span_id log fields, for BOTH chain orders and BOTH provider
// configurations.
//
// FINDING: production (pkg/gofr/http_server.go) registers
// r.Use(middleware.Tracer, middleware.Logging(...)) — with gorilla/mux the
// FIRST registered middleware is the OUTERMOST, so production runs
// Tracer-outer / Logging-inner. That is the order in which Logging observes
// the span Tracer just started, and the correlation ID is real. The
// Logging-outer order (which reads the span context BEFORE Tracer starts a
// span) yields the all-zeros constants instead.
func Test_TracerContract_CorrelationIDWithLogging(t *testing.T) {
	tests := []struct {
		name         string
		loggingOuter bool
		neverSample  bool
		traceparent  bool
		wantZeroIDs  bool
	}{
		{name: "production order, recording provider", wantZeroIDs: false},
		{name: "production order, NeverSample provider", neverSample: true, wantZeroIDs: false},
		{name: "production order, inbound traceparent", traceparent: true, wantZeroIDs: false},
		{name: "logging outer, recording provider", loggingOuter: true, wantZeroIDs: true},
		{name: "logging outer, NeverSample provider", loggingOuter: true, neverSample: true, wantZeroIDs: true},
		{name: "logging outer, inbound traceparent", loggingOuter: true, traceparent: true, wantZeroIDs: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			installPropagators(t)

			if tc.neverSample {
				tracerCharInstallNeverSampleTP(t)
			} else {
				tracerCharInstallRecordingTP(t)
			}

			lg := &tracerCharLogger{}

			var inHandler otelTrace.SpanContext

			handler := tracerCharChain(tc.loggingOuter, lg,
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					inHandler = otelTrace.SpanFromContext(r.Context()).SpanContext()
					_, _ = w.Write([]byte("ok"))
				}))

			rr := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/corr", http.NoBody)

			if tc.traceparent {
				req.Header.Set("Traceparent", "00-"+w3cFixtureTraceID+"-"+w3cFixtureParentSpan+"-01")
			}

			handler.ServeHTTP(rr, req)

			rl := lg.last()
			require.NotNil(t, rl, "Logging must emit a RequestLog")
			assert.Equal(t, http.StatusOK, rl.Response)

			corr := rr.Header().Get("X-Correlation-ID")
			assert.Equal(t, rl.TraceID, corr, "header and log field always agree")

			if tc.wantZeroIDs {
				assert.Equal(t, zeroTraceID, rl.TraceID,
					"Logging read the span context before Tracer started a span")
				assert.Equal(t, zeroSpanID, rl.SpanID)

				return
			}

			require.True(t, inHandler.IsValid())
			assert.Equal(t, inHandler.TraceID().String(), rl.TraceID,
				"log trace_id must equal the span Tracer started")
			assert.Equal(t, inHandler.SpanID().String(), rl.SpanID)
			assert.Len(t, rl.TraceID, 32)
			assert.Len(t, rl.SpanID, 16)

			if tc.traceparent {
				assert.Equal(t, w3cFixtureTraceID, rl.TraceID,
					"inbound traceparent must surface as the correlation ID")
			}
		})
	}
}

// Test_TracerContract_StatusFlowsThroughLoggingChain pins that the span's
// http.response.status_code is correct in the production chain order, where
// Tracer's own StatusResponseWriter sits outside Logging's.
func Test_TracerContract_StatusFlowsThroughLoggingChain(t *testing.T) {
	rec := tracerCharInstallRecordingTP(t)

	lg := &tracerCharLogger{}
	handler := Tracer(Logging(LogProbes{}, lg)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		})))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/chain", http.NoBody)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	spans := rec.Ended()
	require.Len(t, spans, 1)

	attrs := tracerCharAttrs(spans[0])
	assert.Equal(t, int64(http.StatusServiceUnavailable), attrs[1].Value.AsInt64())
	assert.Equal(t, codes.Unset, spans[0].Status().Code, "5xx does not mark the span as errored")

	require.Len(t, lg.errors, 1, "5xx is logged via Error")
	assert.Equal(t, http.StatusServiceUnavailable, lg.errors[0].Response)
}
