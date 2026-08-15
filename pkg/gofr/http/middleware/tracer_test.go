package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	otelTrace "go.opentelemetry.io/otel/trace"
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

// TestBuildSpanName pins the OTel HTTP semconv span-name format. The
// construction changes; the content must not.
func TestBuildSpanName(t *testing.T) {
	tests := []struct {
		name   string
		method string
		route  string
		want   string
	}{
		{"templated route", http.MethodGet, "/users/{id}", "GET /users/{id}"},
		{"root", http.MethodPost, "/", "POST /"},
		{"unmatched falls back to path", http.MethodDelete, "/nope/deeper", "DELETE /nope/deeper"},
		{"empty route", http.MethodGet, "", "GET "},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, buildSpanName(tc.method, tc.route))
		})
	}
}

// spanNameSink defeats dead-code elimination: without a consumed result the
// compiler may delete the very call being measured, and the benchmark then
// reports the cost of nothing.
//
//nolint:gochecknoglobals // standard Go benchmarking idiom; a local would be optimized away.
var spanNameSink string

// TestBuildSpanNameAllocs pins the win. fmt.Sprintf costs 3 allocations per
// request (the result string plus two boxed interface arguments); plain
// concatenation costs exactly the one unavoidable result string.
func TestBuildSpanNameAllocs(t *testing.T) {
	method, route := http.MethodGet, "/users/{id}"

	got := testing.AllocsPerRun(1000, func() {
		spanNameSink = buildSpanName(method, route)
	})

	require.LessOrEqual(t, got, 1.0,
		"span name must cost at most the one unavoidable result-string allocation")
}

// TestTracerSpanNameEndToEnd proves the span actually emitted through the
// middleware still carries the route-template name, not a concrete path.
func TestTracerSpanNameEndToEnd(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	otel.SetTracerProvider(trace.NewTracerProvider(trace.WithSyncer(exporter)))

	router := mux.NewRouter()
	router.Use(Tracer)
	router.NewRoute().Methods(http.MethodGet).Path("/users/{id}").
		HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	router.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users/42", http.NoBody))

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Equal(t, "GET /users/{id}", spans[0].Name)

	var route string

	for _, a := range spans[0].Attributes {
		if a.Key == "http.route" {
			route = a.Value.AsString()
		}
	}

	require.Equal(t, "/users/{id}", route, "http.route must stay the template, not the concrete path")
}

// BenchmarkBuildSpanName is the before/after evidence for this change.
func BenchmarkBuildSpanName(b *testing.B) {
	method, route := http.MethodGet, "/users/{id}"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		spanNameSink = buildSpanName(method, route)
	}
}

// TestTracerStatusAttributeStillRecorded is the guard against "optimizing" by
// silently dropping telemetry: a sampled span must still carry the status code.
func TestTracerStatusAttributeStillRecorded(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	otel.SetTracerProvider(trace.NewTracerProvider(trace.WithSyncer(exporter)))

	router := mux.NewRouter()
	router.Use(Tracer)
	router.NewRoute().Methods(http.MethodGet).Path("/teapot").
		HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })

	router.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/teapot", http.NoBody))

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	var found bool

	for _, a := range spans[0].Attributes {
		if a.Key == "http.response.status_code" {
			require.Equal(t, int64(http.StatusTeapot), a.Value.AsInt64())

			found = true
		}
	}

	require.True(t, found, "a sampled span must still record http.response.status_code")
}

// TestTracerImplicit200StillRecorded pins the normalization PR #3431 established:
// a handler that never calls WriteHeader still reports 200, not 0.
func TestTracerImplicit200StillRecorded(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	otel.SetTracerProvider(trace.NewTracerProvider(trace.WithSyncer(exporter)))

	router := mux.NewRouter()
	router.Use(Tracer)
	router.NewRoute().Methods(http.MethodGet).Path("/silent").
		HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})

	router.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/silent", http.NoBody))

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	for _, a := range spans[0].Attributes {
		if a.Key == "http.response.status_code" {
			require.Equal(t, int64(http.StatusOK), a.Value.AsInt64())
		}
	}
}

// TestSpanMetaCacheIsBounded is the safety property for the cache: a route
// template must produce ONE entry no matter how many concrete paths hit it, and
// an unmatched (attacker-controlled) path must produce none at all.
func TestSpanMetaCacheIsBounded(t *testing.T) {
	tracerSpanCache.Range(func(k, _ any) bool { tracerSpanCache.Delete(k); return true })

	for i := range 500 {
		spanMetaFor(http.MethodGet, "/users/{id}", true)
		spanMetaFor(http.MethodGet, "/attacker/"+strconv.Itoa(i), false)
	}

	var n int

	tracerSpanCache.Range(func(_, _ any) bool { n++; return true })

	require.Equal(t, 1, n,
		"one template must yield one entry, and unmatched paths must never be cached")
}

// TestSpanMetaContentIsCorrect pins that caching did not change what is emitted.
func TestSpanMetaContentIsCorrect(t *testing.T) {
	meta := spanMetaFor(http.MethodPost, "/orders/{id}", true)

	require.Equal(t, "POST /orders/{id}", meta.name)
	require.Len(t, meta.attrs, 2)
	require.Equal(t, attribute.Key("http.request.method"), meta.attrs[0].Key)
	require.Equal(t, "POST", meta.attrs[0].Value.AsString())
	require.Equal(t, attribute.Key("http.route"), meta.attrs[1].Key)
	require.Equal(t, "/orders/{id}", meta.attrs[1].Value.AsString())

	// A second call must return the identical cached instance.
	require.Same(t, meta, spanMetaFor(http.MethodPost, "/orders/{id}", true))
}

// TestSpanMetaConcurrent runs the cache under contention; the race detector is
// the point of this test.
func TestSpanMetaConcurrent(t *testing.T) {
	var (
		wg  sync.WaitGroup
		bad atomic.Int64
	)

	// require calls FailNow, which must only run on the test goroutine, so the
	// workers record failures and the assertion happens after Wait.
	for g := range 16 {
		wg.Add(1)

		go func(g int) {
			defer wg.Done()

			for range 100 {
				m := spanMetaFor(http.MethodGet, "/c/"+strconv.Itoa(g%4)+"/{id}", true)
				if m == nil || m.name == "" || len(m.attrs) != 2 {
					bad.Add(1)
				}
			}
		}(g)
	}

	wg.Wait()

	require.Zero(t, bad.Load(), "cache returned an incomplete spanMeta under contention")
}
