package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	otelTrace "go.opentelemetry.io/otel/trace"

	gofrhttp "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/internal/quietspan"
	"gofr.dev/pkg/gofr/internal/stablemw"
)

var errFrameFixture = errors.New("bad <thing>")

// parityLogger records every entry the middleware logs, in order, rendered as JSON.
type parityLogger struct {
	mu      sync.Mutex
	entries []string
}

func (l *parityLogger) Log(a ...any)   { l.add("LOG", a) }
func (l *parityLogger) Error(a ...any) { l.add("ERROR", a) }

func (l *parityLogger) add(level string, a []any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, v := range a {
		if p, ok := v.(panicLog); ok {
			// The stack trace names the frames the panic went through, which is what the frame
			// changes; that it is there is what the log promises.
			p.StackTrace = fmt.Sprint(p.StackTrace != "")
			v = p
		}

		b, err := json.Marshal(v)
		if err != nil {
			b = []byte(err.Error())
		}

		// The times differ from run to run, and a duration under a microsecond is left out.
		b = frameTime.ReplaceAll(b, []byte(`"start_time":"<time>"`))
		b = frameDuration.ReplaceAll(b, nil)

		l.entries = append(l.entries, fmt.Sprintf("%s %T %s", level, v, b))
	}
}

// parityMetrics records every metric call the middleware makes.
type parityMetrics struct {
	mu    sync.Mutex
	calls []string
}

func (m *parityMetrics) add(kind, name string, value float64, labels []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, fmt.Sprintf("%s %s positive=%v %q", kind, name, value > 0, labels))
}

func (m *parityMetrics) IncrementCounter(_ context.Context, name string, labels ...string) {
	m.add("counter", name, 1, labels)
}

func (m *parityMetrics) DeltaUpDownCounter(_ context.Context, name string, value float64, labels ...string) {
	m.add("updown", name, value, labels)
}

func (m *parityMetrics) RecordHistogram(_ context.Context, name string, value float64, labels ...string) {
	m.add("histogram", name, value, labels)
}

func (m *parityMetrics) SetGauge(name string, value float64, labels ...string) {
	m.add("gauge", name, value, labels)
}

// frameRequest is one request of the parity test.
type frameRequest struct {
	method, path string
	header       map[string]string
}

// frameObserved is everything a request through the framework chain can be seen to do.
type frameObserved struct {
	Status  int
	Header  http.Header
	Body    string
	Handler string // what the handler saw: its writer, and the span in its context
	Logs    []string
	Metrics []string
	Spans   []string
}

func serveFrame(t *testing.T, fused bool, sr *tracetest.SpanRecorder, reqs []frameRequest) []frameObserved {
	t.Helper()
	t.Setenv(gofrhttp.RouterEnvVar, gofrhttp.MatcherMux)

	logs, mets := &parityLogger{}, &parityMetrics{}

	r := gofrhttp.NewRouter()

	layers := []func(http.Handler) http.Handler{
		stablemw.Mark(Tracer),
		stablemw.Mark(Logging(LogProbes{Disabled: true, Paths: []string{"/.well-known/alive"}}, logs)),
		stablemw.Mark(CORS(map[string]string{
			"access-control-allow-origin":  "https://a.example,https://b.example",
			"Access-Control-Allow-Headers": "X-Custom",
		}, r.RegisteredRoutes)),
		stablemw.Mark(Metrics(mets)),
	}

	if !fused {
		layers = slices.Insert(layers, 1, func(next http.Handler) http.Handler {
			return http.HandlerFunc(next.ServeHTTP)
		})
	}

	for _, l := range layers {
		r.Use(l)
	}

	// A user middleware after the framework chain, as UseMiddleware adds them.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-User", "1")
			next.ServeHTTP(w, req)
		})
	})

	var handlerSaw string

	see := func(w http.ResponseWriter, req *http.Request) {
		s := otelTrace.SpanFromContext(req.Context())
		_, isSRW := w.(*StatusResponseWriter)
		handlerSaw = fmt.Sprintf("writer=%T srw=%v recording=%v valid=%v trace=%s span=%s frame=%v",
			w, isSRW, s.IsRecording(), s.SpanContext().IsValid(), s.SpanContext().TraceID(), s.SpanContext().SpanID(),
			inFrame())
	}

	for path, h := range frameRoutes(see) {
		r.Add(http.MethodGet, path, h)
		r.Add(http.MethodOptions, path, h)
	}

	*r.RegisteredRoutes = []string{http.MethodGet, http.MethodOptions}

	out := make([]frameObserved, 0, len(reqs))

	for _, fr := range reqs {
		handlerSaw = ""
		logs.entries, mets.calls = nil, nil

		if sr != nil {
			_ = sr.Shutdown(t.Context())
		}

		req := httptest.NewRequestWithContext(t.Context(), fr.method, fr.path, http.NoBody)
		req.RemoteAddr = "192.0.2.1:1234"

		for k, v := range fr.header {
			req.Header.Set(k, v)
		}

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		out = append(out, frameObserved{
			Status:  rec.Code,
			Header:  rec.Header(),
			Body:    rec.Body.String(),
			Handler: handlerSaw,
			Logs:    logs.entries,
			Metrics: mets.calls,
			Spans:   endedSpans(sr),
		})
	}

	return out
}

// frameRoutes are the parity test's routes: each calls see first, to record what the handler saw.
func frameRoutes(see http.HandlerFunc) map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"/ok": func(w http.ResponseWriter, req *http.Request) {
			see(w, req)
			w.Header().Set("X-A", "a")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte("hi"))
		},
		"/nothing": see,
		"/fail": func(w http.ResponseWriter, req *http.Request) {
			see(w, req)
			w.WriteHeader(http.StatusInternalServerError)
		},
		"/panic-string": func(w http.ResponseWriter, req *http.Request) {
			see(w, req)
			panic("boom")
		},
		"/panic-error": func(w http.ResponseWriter, req *http.Request) {
			see(w, req)
			w.WriteHeader(http.StatusAccepted)
			panic(errFrameFixture)
		},
		"/panic-other": func(w http.ResponseWriter, req *http.Request) {
			see(w, req)
			panic(42)
		},
		"/graphql": func(w http.ResponseWriter, req *http.Request) {
			see(w, req)
			_, _ = w.Write([]byte("{}"))
		},
		"/.well-known/alive": func(w http.ResponseWriter, req *http.Request) {
			see(w, req)
			_, _ = w.Write([]byte("up"))
		},
		"/users/{id}": func(w http.ResponseWriter, req *http.Request) {
			see(w, req)
			w.WriteHeader(http.StatusNotFound)
		},
	}
}

// inFrame reports whether the caller runs inside frameHandler.ServeHTTP.
func inFrame() bool {
	pcs := make([]uintptr, 64)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(1, pcs)])

	for {
		f, more := frames.Next()
		if strings.HasSuffix(f.Function, ".(*frameHandler).ServeHTTP") {
			return true
		}

		if !more {
			return false
		}
	}
}

func endedSpans(sr *tracetest.SpanRecorder) []string {
	if sr == nil {
		return nil
	}

	var out []string

	for _, s := range sr.Ended() {
		attrs := make([]string, 0, len(s.Attributes()))
		for _, a := range s.Attributes() {
			attrs = append(attrs, string(a.Key)+"="+a.Value.String())
		}

		slices.Sort(attrs)

		out = append(out, fmt.Sprintf("name=%q kind=%v trace=%s span=%s parent=%s status=%v attrs=%v",
			s.Name(), s.SpanKind(), s.SpanContext().TraceID(), s.SpanContext().SpanID(), s.Parent().SpanID(),
			s.Status(), attrs))
	}

	return out
}

var (
	frameHexID    = regexp.MustCompile(`\b[0-9a-f]{32}\b|\b[0-9a-f]{16}\b`)
	frameTime     = regexp.MustCompile(`"start_time":"[^"]*"`)
	frameDuration = regexp.MustCompile(`"response_time":\d+,`)
)

// normalise replaces the random trace and span IDs by their order of first appearance, so equal
// IDs stay equal and different ones different.
func normalise(t *testing.T, o []frameObserved) string {
	t.Helper()

	b, err := json.MarshalIndent(o, "", " ")
	require.NoError(t, err)

	ids := map[string]string{}

	return frameHexID.ReplaceAllStringFunc(string(b), func(id string) string {
		if strings.Trim(id, "0") == "" {
			return id
		}

		if _, ok := ids[id]; !ok {
			ids[id] = fmt.Sprintf("<id%d>", len(ids))
		}

		return ids[id]
	})
}

func frameRequests() []frameRequest {
	return []frameRequest{
		{http.MethodGet, "/ok", nil},
		{http.MethodGet, "/ok", map[string]string{"Origin": "https://b.example", "User-Agent": "ua"}},
		{http.MethodGet, "/ok", map[string]string{"Origin": "https://evil.example"}},
		{http.MethodGet, "/ok", map[string]string{
			"traceparent": "00-" + w3cFixtureTraceID + "-" + w3cFixtureParentSpan + "-01",
			"tracestate":  "k=v",
		}},
		{http.MethodGet, "/ok", map[string]string{"X-Forwarded-For": "203.0.113.9, 10.0.0.1"}},
		{http.MethodOptions, "/ok", map[string]string{"Origin": "https://a.example"}},
		{http.MethodGet, "/nothing", nil},
		{http.MethodGet, "/fail", nil},
		{http.MethodGet, "/panic-string", nil},
		{http.MethodGet, "/panic-error", nil},
		{http.MethodGet, "/panic-other", nil},
		{http.MethodGet, "/graphql", nil},
		{http.MethodGet, "/.well-known/alive", nil},
		{http.MethodOptions, "/.well-known/alive", nil},
		{http.MethodGet, "/users/42?q=1", nil},
		{http.MethodGet, "/missing", nil},
		{"BREW", "/ok", nil},
	}
}

// TestFrame_MatchesLayeredChain serves the same requests through GoFr's framework chain served
// from one frame and through the same four layers kept apart, under GoFr's own NeverSample
// provider (the span the router hosts), an unregistered NeverSample provider (the SDK's span) and a
// recording provider, and requires everything observable to be the same: status, headers, body,
// what the handler saw, every log entry and metric call, and every span.
func TestFrame_MatchesLayeredChain(t *testing.T) {
	for _, mode := range []string{"gofr-quiet", "sdk-quiet", "recording"} {
		t.Run(mode, func(t *testing.T) {
			got := map[string][]frameObserved{}

			// The two ways to compose GoFr's framework chain: as GoFr's server does, which Tracer
			// serves from one frame, and with a pass-through layer after Tracer, which keeps the four
			// layers apart.
			for name, fused := range map[string]bool{"fused": true, "layered": false} {
				var sr *tracetest.SpanRecorder

				switch mode {
				case "gofr-quiet":
					quietProvider(t, true)
				case "sdk-quiet":
					quietProvider(t, false)
				default:
					sr = recordingProvider(t)
				}

				got[name] = serveFrame(t, fused, sr, frameRequests())
			}

			for i, o := range got["fused"] {
				if o.Handler != "" {
					assert.Contains(t, o.Handler, "frame=true", "request %d is served by the frame", i)
					assert.Contains(t, got["layered"][i].Handler, "frame=false", "request %d is not", i)
				}
			}

			want := strings.ReplaceAll(normalise(t, got["layered"]), "frame=false", "frame=<f>")
			assert.Equal(t, want, strings.ReplaceAll(normalise(t, got["fused"]), "frame=true", "frame=<f>"))
		})
	}
}

func recordingProvider(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	prevTP, prevProp := otel.GetTracerProvider(), otel.GetTextMapPropagator()

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()), sdktrace.WithSpanProcessor(sr))

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})

	return sr
}

// TestFrame_FusesOnlyTheFrameworkChain checks Tracer serves from one frame exactly when its inner
// handler is Logging, CORS and Metrics composed in that order, and that the frame is a Claimant.
func TestFrame_FusesOnlyTheFrameworkChain(t *testing.T) {
	logs, mets := &parityLogger{}, &parityMetrics{}
	routes := &[]string{}

	logging, cors, metrics := Logging(LogProbes{}, logs), CORS(nil, routes), Metrics(mets)
	h := http.NewServeMux()

	fused := Tracer(logging(cors(metrics(h))))
	require.IsType(t, &frameHandler{}, fused)
	assert.Implements(t, (*quietspan.Claimant)(nil), fused)
	assert.Same(t, h, fused.(*frameHandler).inner)

	for name, inner := range map[string]http.Handler{
		"handler":            h,
		"logging only":       logging(h),
		"logging, cors":      logging(cors(h)),
		"cors first":         cors(logging(metrics(h))),
		"metrics first":      metrics(cors(logging(h))),
		"user layer between": logging(http.HandlerFunc(cors(metrics(h)).ServeHTTP)),
	} {
		assert.IsType(t, &tracerHandler{}, Tracer(inner), name)
	}
}
