package middleware

import (
	"net/http"
	"net/textproto"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	gofrhttp "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/version"
)

// methodKV constructs the http.request.method attribute (OTel HTTP
// semconv ≥ v1.21 stable). Previously this used the v1.4 "http.method"
// attribute key — that key is now deprecated in upstream semconv and
// downstream dashboards built against current semconv versions miss the
// attribute if we keep emitting the old key.
//
// Allocation note: the returned KeyValue + the underlying string are
// hoisted into the variadic ...attribute.KeyValue slice passed to
// trace.WithAttributes below, which the compiler escape-analyzes onto
// the heap (verified via go build -gcflags='-m=2'). Factoring the
// constructor into its own function does not avoid the alloc — it is
// still useful as a single source of truth for the attribute key.
func methodKV(method string) attribute.KeyValue {
	return attribute.String("http.request.method", method)
}

// buildSpanName renders the OTel HTTP semconv span name ("GET /users/{id}").
//
// Concatenation rather than fmt.Sprintf. Both cost the one unavoidable
// allocation for the result string, so this is a CPU saving only, not an
// allocation saving: Sprintf walks a format string and boxes two arguments that
// are already strings.
//
// No before/after figure is quoted here: the fmt.Sprintf baseline it would be
// measured against is gone, so nothing in the tree can reproduce one.
func buildSpanName(method, route string) string {
	return method + " " + route
}

// routeTemplate returns the matched route template (e.g. "/users/{id}") for the request, otherwise
// the raw URL path. Used for the span name and http.route attribute so tracing cardinality stays
// bounded by route count, not request count.
//
// The bool reports whether a template was resolved, which is what makes the span-name cache safe:
// only a templated route may be cached, since a raw path is unbounded and would grow the cache once
// per distinct request. gofrhttp.RouteTemplate resolves the template under both the trie and the
// default mux router.
func routeTemplate(r *http.Request) (route string, templated bool) {
	if t := gofrhttp.RouteTemplate(r); t != "" {
		return t, true
	}

	return r.URL.Path, false
}

// spanMeta is the immutable per-route span data: the semconv span name and the
// attribute slice handed to trace.WithAttributes. Both are pure functions of
// (method, route), so they are provider-independent and safe to share process
// wide even across TracerProvider replacement.
type spanMeta struct {
	name  string
	attrs []attribute.KeyValue
	// startOpts is the fully built []trace.SpanStartOption handed to Start.
	// trace.WithAttributes allocates an option wrapper AND the variadic option
	// slice on every call, so caching the attribute slice alone still left two
	// allocations per request. The option is immutable once constructed, so it
	// is safe to share.
	startOpts []trace.SpanStartOption
}

// tracerSpanCache maps routeKey -> *spanMeta.
//
// Keyed on the ROUTE TEMPLATE ("/users/{id}"), never a concrete path. That alone
// was once believed to bound it by routes x methods, but neither half of that
// product is bounded by itself in a real GoFr app -- see cacheableMethod, and
// routeCache for the cap that backs the argument up.
//
// It is package level because the middleware chain is rebuilt around the matched
// handler on every request, so a cache owned by the closure would be discarded
// each time and never serve a second request.
//
//nolint:gochecknoglobals // rationale above: a per-closure cache would never be reused.
var tracerSpanCache routeCache

// spanMetaFor returns the span name and attributes for a route, building them on
// first use.
//
// templated reports whether route came from a matched route template; when false
// the value is a raw request path and must not enter the cache. That guard is
// necessary but NOT sufficient -- GoFr's catch-all makes it true for every
// request, including ones no route matched -- so the method is filtered too, and
// the cache itself is capped. An entry that cannot be cached is simply rebuilt,
// which is what every request did before this cache existed.
func spanMetaFor(method, route string, templated bool) *spanMeta {
	if !templated || !cacheableMethod(method) {
		return newSpanMeta(method, route)
	}

	key := routeKey{method: method, route: route}
	if v, ok := tracerSpanCache.load(key); ok {
		meta, _ := v.(*spanMeta)

		return meta
	}

	meta := newSpanMeta(method, route)
	tracerSpanCache.store(key, meta)

	return meta
}

func newSpanMeta(method, route string) *spanMeta {
	attrs := []attribute.KeyValue{
		methodKV(method),
		attribute.String("http.route", route),
	}

	return &spanMeta{
		name:      buildSpanName(method, route),
		attrs:     attrs,
		startOpts: []trace.SpanStartOption{trace.WithAttributes(attrs...)},
	}
}

// The lowercase header names the W3C propagators look up. They are the keys the
// propagators pass to the carrier, not the spellings that go on the wire.
const (
	headerTraceparent = "traceparent"
	headerTracestate  = "tracestate"
	headerBaggage     = "baggage"
)

// canonicalPropagationKeys maps the lowercase header names the W3C propagators
// look up to their canonical spellings.
//
// http.Header.Get canonicalizes whatever key it is handed, and none of these
// names is already canonical, so each lookup allocated the canonical string --
// on every request, whether or not the header was even present. The spellings
// are constants, so they are resolved once here.
//
//nolint:gochecknoglobals // immutable, process-wide header constants.
var canonicalPropagationKeys = map[string]string{
	headerTraceparent: textproto.CanonicalMIMEHeaderKey(headerTraceparent),
	headerTracestate:  textproto.CanonicalMIMEHeaderKey(headerTracestate),
	headerBaggage:     textproto.CanonicalMIMEHeaderKey(headerBaggage),
}

// headerCarrier adapts http.Header for the OTel propagators without paying to
// canonicalize the lookup key on every request.
//
// A key it does not recognize falls through to the ordinary lookup, so a custom
// propagator using its own header names keeps working unchanged.
type headerCarrier http.Header

func (c headerCarrier) Get(key string) string {
	canonical, ok := canonicalPropagationKeys[key]
	if !ok {
		return http.Header(c).Get(key)
	}

	if v := c[canonical]; len(v) > 0 {
		return v[0]
	}

	return ""
}

func (c headerCarrier) Set(key, value string) { http.Header(c).Set(key, value) }

// Values returns every value for key, satisfying propagation.ValuesGetter.
//
// Not optional. propagation.Baggage.Extract type-asserts the carrier to
// ValuesGetter and, when it matches, combines ALL values of the Baggage header;
// the stdlib propagation.HeaderCarrier this type replaces satisfies it. Without
// this method the assertion fails and Extract silently falls back to the
// single-value Get, dropping every baggage member after the first whenever a
// request carries more than one Baggage header -- which is legal per W3C and is
// what proxies and service meshes commonly emit. GoFr installs
// propagation.Baggage in its default composite propagator, so that path is live.
//
// Measured against the stdlib carrier with three Baggage headers: stdlib
// extracted 3 members, this carrier extracted 1 before the method existed.
//
// The canonical-key fast path is deliberately not used here. Baggage is not one
// of the keys canonicalPropagationKeys covers, and a carrier that replaces a
// stdlib one has to be a faithful drop-in first and an optimization second.
func (c headerCarrier) Values(key string) []string { return http.Header(c).Values(key) }

func (c headerCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}

	return keys
}

// Tracer is a middleware that starts a new OpenTelemetry trace span for each
// request and records http.request.method, http.route, and
// http.response.status_code attributes on it, following the current OTel
// HTTP semantic conventions (≥ v1.21).
//
// Behavioral change vs prior versions: GoFr used to wrap routes in
// otelhttp.NewHandler("gofr-router") which produced spans with the static
// name "gofr-router". Spans are now named "METHOD /route-template" (e.g.
// "GET /users/{id}") per the OTel HTTP semconv span-name guidance. Users
// with dashboards or alerts filtering on span.name == "gofr-router" must
// update their filters.
//
// The tracer is resolved once at chain-build time (after App.New has installed
// the real provider via initTracer; see factory.go) and captured in the
// per-request closure — otel.GetTracerProvider().Tracer(name) is a mutex-
// guarded map lookup under the SDK provider, so resolving once saves that
// lookup on every request.
func Tracer(inner http.Handler) http.Handler {
	tr := otel.Tracer("gofr-" + version.Framework)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// extract the traceID and spanID from the headers and create a new context for the same
		// this context will make a new span using the traceID and link the incoming SpanID as
		// its parentID, thus connecting two spans
		ctx = otel.GetTextMapPropagator().Extract(ctx, headerCarrier(r.Header))

		method := strings.ToUpper(r.Method)
		// Prefer the gorilla/mux route template (e.g. "/users/{id}") so the
		// span name and http.route attribute do not explode to one unique
		// value per concrete path (e.g. "/users/42"). Fall back to URL.Path
		// when no route matched (404 / unknown route).
		route, templated := routeTemplate(r)
		meta := spanMetaFor(method, route, templated)

		// The attribute slice is passed to Start, not applied afterwards: a
		// sampler can read attributes when deciding, so moving them after the
		// span exists would change sampling for anyone sampling on http.route.
		ctxOut, span := tr.Start(ctx, meta.name, meta.startOpts...)
		defer span.End()

		// The response status is only ever read to put it on the span, so the
		// writer is only wrapped when the span will keep it.
		//
		// Tracer is the outermost middleware, so the wrapper Logging installs
		// does not exist yet and this type assertion always failed -- meaning a
		// StatusResponseWriter was allocated on every request and, on the
		// default deployment, never read. GoFr installs an SDK provider with
		// NeverSample when no TRACE_EXPORTER is configured, so no span records.
		//
		// Recording spans are unaffected: they still get the wrapper and the
		// attribute, pinned by TestTracerStatusAttributeStillRecorded.
		if span.IsRecording() {
			srw, ok := w.(*StatusResponseWriter)
			if !ok {
				srw = &StatusResponseWriter{ResponseWriter: w}
				w = srw
			}

			// Status() normalizes the zero-default to http.StatusOK when the
			// handler called neither WriteHeader nor Write -- net/http emits an
			// implicit 200 in that case, so the span attribute must report 200
			// rather than be omitted, or worse recorded as 0.
			defer func(s trace.Span, rw *StatusResponseWriter) {
				s.SetAttributes(attribute.Int("http.response.status_code", rw.Status()))
			}(span, srw)
		}

		inner.ServeHTTP(w, r.WithContext(ctxOut))
	})
}
