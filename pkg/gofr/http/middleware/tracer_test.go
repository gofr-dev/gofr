package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
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
