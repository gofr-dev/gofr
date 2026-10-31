package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	otelTrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/embedded"
	"go.opentelemetry.io/otel/trace/noop"
)

// installTP installs tp as the global provider for the rest of the test.
func installTP(t *testing.T, tp otelTrace.TracerProvider) {
	t.Helper()

	prev := otel.GetTracerProvider()

	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
}

func serveTraced(t *testing.T) {
	t.Helper()

	h := Tracer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody))
}

func TestFrameworkTracerIsTheProvidersTracer(t *testing.T) {
	tp := trace.NewTracerProvider()

	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	assert.Same(t, tp.Tracer(frameworkTracerName), frameworkTracer(tp))
	assert.Same(t, tp.Tracer(frameworkTracerName), frameworkTracer(tp), "a memoised call returns the same tracer")
}

// A provider installed after requests were already served must get the next request's span, as it
// did when the tracer was looked up on every request.
func TestTracerFollowsProviderReplacement(t *testing.T) {
	first, second := tracetest.NewSpanRecorder(), tracetest.NewSpanRecorder()
	tp1 := trace.NewTracerProvider(trace.WithSpanProcessor(first))
	tp2 := trace.NewTracerProvider(trace.WithSpanProcessor(second))

	t.Cleanup(func() {
		_ = tp1.Shutdown(context.Background())
		_ = tp2.Shutdown(context.Background())
	})

	installTP(t, tp1)
	serveTraced(t)
	serveTraced(t)

	otel.SetTracerProvider(tp2)
	serveTraced(t)

	assert.Len(t, first.Ended(), 2)
	require.Len(t, second.Ended(), 1)
	assert.Equal(t, frameworkTracerName, second.Ended()[0].InstrumentationScope().Name)
}

// uncomparableTP is a provider whose dynamic type cannot be compared with ==.
type uncomparableTP struct {
	embedded.TracerProvider

	calls *atomic.Int64
	_     []int
}

func (p uncomparableTP) Tracer(name string, opts ...otelTrace.TracerOption) otelTrace.Tracer {
	p.calls.Add(1)

	return noop.NewTracerProvider().Tracer(name, opts...)
}

func TestFrameworkTracerUncomparableProvider(t *testing.T) {
	calls := &atomic.Int64{}
	tp := uncomparableTP{calls: calls}

	installTP(t, tp)

	assert.NotPanics(t, func() {
		serveTraced(t)
		serveTraced(t)
	})
	assert.Equal(t, int64(2), calls.Load(), "an uncomparable provider is asked on every request")

	// A memo left by a comparable provider must not make the comparison panic either.
	installTP(t, noop.NewTracerProvider())
	serveTraced(t)
	installTP(t, tp)
	assert.NotPanics(t, func() { serveTraced(t) })
}

func TestFrameworkTracerConcurrentReplacement(t *testing.T) {
	providers := []otelTrace.TracerProvider{trace.NewTracerProvider(), trace.NewTracerProvider(), noop.NewTracerProvider()}

	installTP(t, providers[0])

	var wg sync.WaitGroup

	for i := range 8 {
		wg.Go(func() {
			for j := range 200 {
				if i == 0 && j%10 == 0 {
					otel.SetTracerProvider(providers[j/10%len(providers)])
				}

				serveTraced(t)
			}
		})
	}

	wg.Wait()

	for _, tp := range providers {
		if s, ok := tp.(*trace.TracerProvider); ok {
			_ = s.Shutdown(context.Background())
		}
	}
}

// BenchmarkTracer_Parallel is GoFr's default: an SDK provider that never samples, behind
// gorilla/mux, which builds the chain -- and so calls Tracer -- on every request.
func BenchmarkTracer_Parallel(b *testing.B) {
	tp := trace.NewTracerProvider(trace.WithSampler(trace.NeverSample()))
	prev := otel.GetTracerProvider()

	otel.SetTracerProvider(tp)
	b.Cleanup(func() {
		otel.SetTracerProvider(prev)

		_ = tp.Shutdown(context.Background())
	})

	router := mux.NewRouter()
	router.Use(Tracer)
	router.NewRoute().Methods(http.MethodGet).Path("/users/{id}").
		Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/users/42", http.NoBody)
		w := &tracerBenchWriter{h: make(http.Header, 4)}

		for pb.Next() {
			clear(w.h)
			router.ServeHTTP(w, req)
		}
	})
}

// BenchmarkTracerConstruct is the per-request cost gorilla/mux adds by building the chain on
// every request: one Tracer(inner) call, here from both Ps at once, under GoFr's default provider.
func BenchmarkTracerConstruct(b *testing.B) {
	tp := trace.NewTracerProvider(trace.WithSampler(trace.NeverSample()))
	prev := otel.GetTracerProvider()

	otel.SetTracerProvider(tp)
	b.Cleanup(func() {
		otel.SetTracerProvider(prev)

		_ = tp.Shutdown(context.Background())
	})

	inner := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})

	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = Tracer(inner)
		}
	})
}
