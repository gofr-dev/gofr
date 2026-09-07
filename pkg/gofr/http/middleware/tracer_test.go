package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	otelTrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

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
	tracerSpanCache.reset()

	for i := range 500 {
		spanMetaFor(http.MethodGet, "/users/{id}", true)
		spanMetaFor(http.MethodGet, "/attacker/"+strconv.Itoa(i), false)
	}

	require.Equal(t, int64(1), tracerSpanCache.len(),
		"one template must yield one entry, and unmatched paths must never be cached")
}

// TestSpanMetaCacheRejectsUndefinedMethods is the regression test for a
// remotely-triggerable unbounded-memory DoS.
//
// The cache's safety argument was "only bounded route templates are cached", and
// that argument was dead in a real GoFr app. gofr.go registers a
// PathPrefix("/") catch-all, so mux.CurrentRoute is set for every request and
// GetPathTemplate() returns "/" even for a path nothing matched -- making the
// templated guard true always. net/http then accepts any RFC 7230 token as a
// method, so an unauthenticated client streaming M00001, M00002, ... minted a
// permanent *spanMeta each and grew resident memory without bound. The
// pre-cache fmt.Sprintf path retained no state at all, so this was introduced
// by the cache, not inherited.
func TestSpanMetaCacheRejectsUndefinedMethods(t *testing.T) {
	tracerSpanCache.reset()

	// The exact attack: distinct made-up methods against the catch-all template.
	for i := range 5000 {
		spanMetaFor("M"+strconv.Itoa(i), "/", true)
	}

	require.Zero(t, tracerSpanCache.len(),
		"an undefined method must never mint a cache entry")

	// The requests still work -- they just rebuild, as every request did before
	// the cache existed.
	meta := spanMetaFor("M00001", "/", true)
	require.Equal(t, "M00001 /", meta.name)
}

// TestSpanMetaCacheStillCachesStandardMethods pins that the method filter did
// not break the optimization it protects.
func TestSpanMetaCacheStillCachesStandardMethods(t *testing.T) {
	tracerSpanCache.reset()

	for _, m := range []string{
		http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete, http.MethodConnect,
		http.MethodOptions, http.MethodTrace,
	} {
		first := spanMetaFor(m, "/users/{id}", true)
		require.Same(t, first, spanMetaFor(m, "/users/{id}", true),
			"a standard method must still be served from the cache")
	}

	require.Equal(t, int64(9), tracerSpanCache.len())
}

// TestRouteCacheStopsAtItsLimit pins the backstop. The method filter is the
// primary bound, but it is an argument about reachability; the cap is a number,
// and process memory should rest on the number too.
func TestRouteCacheStopsAtItsLimit(t *testing.T) {
	var c routeCache[int]

	for i := range routeCacheLimit + 500 {
		c.store(routeKey{method: http.MethodGet, route: "/r/" + strconv.Itoa(i)}, i)
	}

	require.Equal(t, int64(routeCacheLimit), c.len(),
		"the cache must refuse to grow past its limit")

	// Past the cap, a miss is a miss -- the caller rebuilds rather than erroring.
	_, ok := c.load(routeKey{method: http.MethodGet, route: "/r/" + strconv.Itoa(routeCacheLimit+100)})
	require.False(t, ok)
}

// TestTracerNonRecordingPathSkipsTheWriterWrapper covers the branch this PR
// optimizes, which had no test at all: with a non-recording provider the
// StatusResponseWriter must not be allocated, and the handler must see the
// writer the server handed in.
func TestTracerNonRecordingPathSkipsTheWriterWrapper(t *testing.T) {
	otel.SetTracerProvider(noop.NewTracerProvider())

	var got http.ResponseWriter

	router := mux.NewRouter()
	router.Use(Tracer)
	router.NewRoute().Methods(http.MethodGet).Path("/x").
		HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { got = w })

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody))

	require.NotNil(t, got)
	assert.IsType(t, &httptest.ResponseRecorder{}, got,
		"a non-recording span must not pay for the status-capturing wrapper")
}

// TestSpanMetaContentIsCorrect pins that caching did not change what is emitted.
func TestSpanMetaContentIsCorrect(t *testing.T) {
	meta := spanMetaFor(http.MethodPost, "/orders/{id}", true)

	require.Equal(t, "POST /orders/{id}", meta.name)

	// Read the attributes back the way the SDK does, by resolving the start
	// options, rather than from a field the middleware itself never consults.
	cfg := otelTrace.NewSpanStartConfig(meta.startOpts...)
	attrs := cfg.Attributes()
	require.Len(t, attrs, 2)
	require.Equal(t, attribute.Key("http.request.method"), attrs[0].Key)
	require.Equal(t, "POST", attrs[0].Value.AsString())
	require.Equal(t, attribute.Key("http.route"), attrs[1].Key)
	require.Equal(t, "/orders/{id}", attrs[1].Value.AsString())

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
				if m == nil || m.name == "" {
					bad.Add(1)

					continue
				}

				cfg := otelTrace.NewSpanStartConfig(m.startOpts...)
				if len(cfg.Attributes()) != 2 {
					bad.Add(1)
				}
			}
		}(g)
	}

	wg.Wait()

	require.Zero(t, bad.Load(), "cache returned an incomplete spanMeta under contention")
}

// TestHeaderCarrierMatchesHeaderCarrier pins that the carrier resolves exactly
// what propagation.HeaderCarrier resolves, for the propagation headers and for
// an unrecognized key that must fall through to the ordinary lookup.
func TestHeaderCarrierMatchesHeaderCarrier(t *testing.T) {
	h := http.Header{}
	// Set with the canonical spellings; the carrier is still queried with the
	// lowercase names the propagators actually use, which is the point.
	h.Set("Traceparent", "00-"+w3cFixtureTraceID+"-"+w3cFixtureParentSpan+"-01")
	h.Set("Tracestate", "vendor=value")
	h.Set("Baggage", "k=v")
	h.Set("X-Custom-Propagator", "custom")

	ours := headerCarrier(h)
	theirs := propagation.HeaderCarrier(h)

	for _, key := range []string{"traceparent", "tracestate", "baggage", "X-Custom-Propagator", "absent"} {
		require.Equal(t, theirs.Get(key), ours.Get(key), "key %q must resolve identically", key)
	}

	require.ElementsMatch(t, theirs.Keys(), ours.Keys())
}

// TestTracerPropagatesIncomingTraceContext is the feature guard: an incoming
// traceparent must still be adopted as the parent of the server span.
func TestTracerPropagatesIncomingTraceContext(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	otel.SetTracerProvider(trace.NewTracerProvider(trace.WithSyncer(exporter)))
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Both globals are restored. Without this the recording provider and the
	// Baggage-less propagator leak into every later test and benchmark in the
	// package -- tests run before benchmarks, so BenchmarkTracer would measure
	// the RECORDING path while documenting the non-recording one, and feed this
	// exporter for the rest of the run.
	t.Cleanup(func() {
		otel.SetTracerProvider(noop.NewTracerProvider())
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{}))
	})

	router := mux.NewRouter()
	router.Use(Tracer)
	router.NewRoute().Methods(http.MethodGet).Path("/x").
		HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody)
	req.Header.Set("Traceparent", "00-"+w3cFixtureTraceID+"-"+w3cFixtureParentSpan+"-01")

	router.ServeHTTP(httptest.NewRecorder(), req)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)
	require.Equal(t, w3cFixtureTraceID, spans[0].SpanContext.TraceID().String(),
		"the incoming trace ID must be adopted")
	require.Equal(t, w3cFixtureParentSpan, spans[0].Parent.SpanID().String(),
		"the incoming span must become the parent")
}

// tracerBenchWriter keeps a header map and discards the rest, so the benchmark measures the
// middleware rather than a recorder.
type tracerBenchWriter struct{ h http.Header }

func (w *tracerBenchWriter) Header() http.Header       { return w.h }
func (*tracerBenchWriter) Write(b []byte) (int, error) { return len(b), nil }
func (*tracerBenchWriter) WriteHeader(int)             {}

// BenchmarkTracer measures the tracing middleware on the path every request takes.
//
// The default provider is non-recording, which is the common production case for a service that
// samples: the span is still started on every request, so whatever the middleware builds up front is
// paid for whether or not anything records it. Allocations are the metric that matters.
func BenchmarkTracer(b *testing.B) {
	// Pin the non-recording provider this benchmark is about. Package state is
	// shared and tests run first, so inheriting whatever a previous test left
	// installed would silently measure the recording path instead.
	otel.SetTracerProvider(noop.NewTracerProvider())

	b.Cleanup(func() { otel.SetTracerProvider(noop.NewTracerProvider()) })

	router := mux.NewRouter()
	router.Use(Tracer)
	router.NewRoute().Methods(http.MethodGet).Path("/users/{id}").
		Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/users/42", http.NoBody)
	w := &tracerBenchWriter{h: make(http.Header, 4)}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		clear(w.h)
		router.ServeHTTP(w, req)
	}
}

// BenchmarkTracer_Recording is the case the per-route cache targets: a provider that actually
// records, so the span name and attributes built by the middleware are used rather than discarded.
//
// BenchmarkTracer above covers the opposite case, a non-recording provider, because a sampling
// service runs both and the middleware must not be quietly worse in either.
func BenchmarkTracer_Recording(b *testing.B) {
	tp := trace.NewTracerProvider(
		trace.WithResource(resource.Empty()),
		trace.WithSampler(trace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)

	b.Cleanup(func() { otel.SetTracerProvider(noop.NewTracerProvider()) })

	router := mux.NewRouter()
	router.Use(Tracer)
	router.NewRoute().Methods(http.MethodGet).Path("/users/{id}").
		Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/users/42", http.NoBody)
	w := &tracerBenchWriter{h: make(http.Header, 4)}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		clear(w.h)
		router.ServeHTTP(w, req)
	}
}

// TestHeaderCarrierIsAFaithfulValuesGetter is the regression test for silent
// baggage loss.
//
// headerCarrier replaces propagation.HeaderCarrier to avoid canonicalizing the
// lookup key on every request. The stdlib type it replaces satisfies
// propagation.ValuesGetter, and propagation.Baggage.Extract type-asserts to that
// interface to combine ALL values of the Baggage header. Implementing only
// Get/Set/Keys failed the assertion, so Extract fell back to the single-value Get
// and dropped every baggage member after the first -- silently, and only when a
// request carried more than one Baggage header, which is legal per W3C and is
// what proxies and service meshes commonly emit.
//
// The pre-existing equivalence test only compared Get and Keys, so it could not
// catch this.
func TestHeaderCarrierIsAFaithfulValuesGetter(t *testing.T) {
	require.Implements(t, (*propagation.ValuesGetter)(nil), headerCarrier{},
		"a carrier replacing propagation.HeaderCarrier must satisfy every interface it does")

	tests := []struct {
		name    string
		values  []string
		wantLen int
	}{
		{"multiple Baggage headers", []string{"k1=v1", "k2=v2", "k3=v3"}, 3},
		{"two Baggage headers", []string{"a=1", "b=2"}, 2},
		{"single header carrying several members", []string{"a=1,b=2"}, 2},
		{"single header single member", []string{"only=1"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.Header{}
			for _, v := range tt.values {
				h.Add("Baggage", v)
			}

			prop := propagation.Baggage{}

			std := baggage.FromContext(prop.Extract(t.Context(), propagation.HeaderCarrier(h)))
			got := baggage.FromContext(prop.Extract(t.Context(), headerCarrier(h)))

			require.Len(t, std.Members(), tt.wantLen, "sanity: the stdlib carrier must see every member")
			assert.Len(t, got.Members(), tt.wantLen,
				"headerCarrier must extract exactly what the stdlib carrier does")

			// Compare member-by-member, since Baggage.String() ordering is not stable.
			for _, m := range std.Members() {
				assert.Equal(t, m.Value(), got.Member(m.Key()).Value(),
					"member %q must survive extraction", m.Key())
			}
		})
	}
}

// TestHeaderCarrierValuesMatchesStdlib pins Values itself across the key shapes
// the propagators actually use, including the canonicalization fast path.
func TestHeaderCarrierValuesMatchesStdlib(t *testing.T) {
	h := http.Header{}
	h.Add("Baggage", "a=1")
	h.Add("Baggage", "b=2")
	h.Set("Traceparent", "00-"+w3cFixtureTraceID+"-"+w3cFixtureParentSpan+"-01")
	h.Add("X-Custom", "one")
	h.Add("X-Custom", "two")

	for _, key := range []string{headerBaggage, headerTraceparent, headerTracestate, "x-custom", "absent"} {
		assert.Equal(t, propagation.HeaderCarrier(h).Values(key), headerCarrier(h).Values(key),
			"Values(%q) must match the stdlib carrier", key)
	}
}

// BenchmarkTracer_WithPropagationHeaders is the variant that actually exercises
// headerCarrier.
//
// BenchmarkTracer and BenchmarkTracer_Recording send a bare request, so the
// propagators find nothing and the canonicalization this PR avoids never runs --
// their alloc delta comes from the per-route cache and the IsRecording gate
// alone. A request carrying traceparent/tracestate/baggage is what a service
// behind a mesh or an upstream GoFr service actually receives, and it is the only
// shape where the carrier's saving is visible.
func BenchmarkTracer_WithPropagationHeaders(b *testing.B) {
	otel.SetTracerProvider(noop.NewTracerProvider())
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{}))

	b.Cleanup(func() { otel.SetTracerProvider(noop.NewTracerProvider()) })

	router := mux.NewRouter()
	router.Use(Tracer)
	router.NewRoute().Methods(http.MethodGet).Path("/users/{id}").
		Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/users/42", http.NoBody)
	req.Header.Set("Traceparent", "00-"+w3cFixtureTraceID+"-"+w3cFixtureParentSpan+"-01")
	req.Header.Set("Tracestate", "vendor=value")
	req.Header.Add("Baggage", "tenant=acme")
	req.Header.Add("Baggage", "region=eu")

	w := &tracerBenchWriter{h: make(http.Header, 4)}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		clear(w.h)
		router.ServeHTTP(w, req)
	}
}

// Characterization suite for the Tracer HTTP middleware.
//
// Every identifier below is prefixed with `tracerChar` so this file can be
// merged with sibling _test.go additions in the same package without
// collisions. Nothing here is aspirational: every assertion pins the CURRENT
// behavior of pkg/gofr/http/middleware/tracer.go as observed against the
// unmodified source.
// ---------------------------------------------------------------------------

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

	// The scope name embeds version.Framework: "gofr-dev" on a development build, "gofr-v1.60.0" on
	// a release build. That moving part is the contract — a consumer filtering on the scope name has
	// to expect it to change every release — so the assertion derives the expected value rather than
	// pinning a literal, which would fail on every release branch.
	assert.Equal(t, "gofr-"+version.Framework, spans[0].InstrumentationScope().Name,
		"scope name is the gofr- prefix plus version.Framework")
	assert.NotEqual(t, "gofr-", spans[0].InstrumentationScope().Name,
		"the version must actually be appended, not an empty string")
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
