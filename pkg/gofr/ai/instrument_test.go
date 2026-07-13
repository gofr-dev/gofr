package ai

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

var errCall = errors.New("call failed")

type capturedMetric struct {
	name   string
	value  float64
	labels []string
}

type fakeMetrics struct {
	counters   []capturedMetric
	histograms []capturedMetric
}

func (f *fakeMetrics) IncrementCounter(_ context.Context, name string, labels ...string) {
	f.counters = append(f.counters, capturedMetric{name: name, labels: labels})
}

func (f *fakeMetrics) RecordHistogram(_ context.Context, name string, value float64, labels ...string) {
	f.histograms = append(f.histograms, capturedMetric{name: name, value: value, labels: labels})
}

type fakeLogger struct {
	debug []any
	errs  []any
}

func (f *fakeLogger) Debug(args ...any) { f.debug = append(f.debug, args...) }
func (f *fakeLogger) Error(args ...any) { f.errs = append(f.errs, args...) }

func testDeps() (Deps, *fakeMetrics, *fakeLogger, *tracetest.SpanRecorder) {
	m := &fakeMetrics{}
	l := &fakeLogger{}
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))

	return Deps{Metrics: m, Logger: l, Tracer: tp.Tracer("test")}, m, l, sr
}

func labelValue(labels []string, key string) string {
	for i := 0; i+1 < len(labels); i += 2 {
		if labels[i] == key {
			return labels[i+1]
		}
	}

	return ""
}

func TestInstrument_Success(t *testing.T) {
	deps, m, l, sr := testDeps()
	info := &CallInfo{Deps: deps, Provider: "groq", Model: "llama-3.3-70b", Op: "chat"}

	resp, err := Instrument(t.Context(), info, func(context.Context) (*Response, error) {
		return &Response{Content: "hi", Usage: Usage{PromptTokens: 12, CompletionTokens: 7}}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, "hi", resp.Content)

	require.Len(t, m.counters, 1)
	assert.Equal(t, metricRequestCount, m.counters[0].name)
	assert.Equal(t, statusSuccess, labelValue(m.counters[0].labels, "status"))

	require.Len(t, m.histograms, 2)
	assert.InDelta(t, 12, m.histograms[0].value, 0)
	assert.InDelta(t, 7, m.histograms[1].value, 0)

	assert.Len(t, l.debug, 1)
	assert.Empty(t, l.errs)

	spans := sr.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, "llm.chat", spans[0].Name())
}

func TestInstrument_Error(t *testing.T) {
	deps, m, l, sr := testDeps()
	info := &CallInfo{Deps: deps, Provider: "groq", Model: "llama", Op: "chat"}

	_, err := Instrument(t.Context(), info, func(context.Context) (*Response, error) {
		return nil, errCall
	})

	require.ErrorIs(t, err, errCall)

	require.Len(t, m.counters, 1)
	assert.Equal(t, statusError, labelValue(m.counters[0].labels, "status"))

	assert.Empty(t, m.histograms, "no token histogram when the call fails")
	assert.Len(t, l.errs, 1)

	spans := sr.Ended()
	require.Len(t, spans, 1)
	assert.NotEmpty(t, spans[0].Events(), "error is recorded on the span")
}

// The context handed to fn must carry the llm span, so the provider's own spans nest under it.
func TestInstrument_ContextCarriesSpan(t *testing.T) {
	deps, _, _, sr := testDeps()

	var fnSpanID trace.SpanID

	_, err := Instrument(t.Context(), &CallInfo{Deps: deps, Op: "chat"}, func(ctx context.Context) (*Response, error) {
		fnSpanID = trace.SpanContextFromContext(ctx).SpanID()
		return &Response{}, nil
	})

	require.NoError(t, err)

	spans := sr.Ended()
	require.Len(t, spans, 1)
	assert.Equal(t, spans[0].SpanContext().SpanID(), fnSpanID, "fn must run inside the llm span")
}

func TestInstrument_CancelledContextPassedToCall(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var seen error

	_, _ = Instrument(ctx, &CallInfo{Op: "chat"}, func(ctx context.Context) (*Response, error) {
		seen = ctx.Err()
		return &Response{}, nil
	})

	require.ErrorIs(t, seen, context.Canceled)
}

func TestInstrument_NilDepsNoPanic(t *testing.T) {
	assert.NotPanics(t, func() {
		_, _ = Instrument(t.Context(), &CallInfo{Op: "chat"}, func(context.Context) (*Response, error) {
			return &Response{Usage: Usage{PromptTokens: 3}}, nil
		})
	})
}

func TestStartCall_Finish(t *testing.T) {
	deps, m, _, sr := testDeps()
	rec := StartCall(t.Context(), &CallInfo{Deps: deps, Provider: "groq", Model: "llama", Op: "stream"})

	require.NotNil(t, rec.Context())
	rec.Finish(Usage{PromptTokens: 4, CompletionTokens: 9}, nil)

	require.Len(t, m.counters, 1)
	require.Len(t, m.histograms, 2)
	assert.Len(t, sr.Ended(), 1)
}

func TestStartCall_FinishError(t *testing.T) {
	deps, m, _, sr := testDeps()
	rec := StartCall(t.Context(), &CallInfo{Deps: deps, Provider: "groq", Model: "llama", Op: "stream"})

	rec.Finish(Usage{PromptTokens: 4}, errCall)

	require.Len(t, m.counters, 1)
	assert.Equal(t, statusError, labelValue(m.counters[0].labels, "status"))
	assert.Empty(t, m.histograms, "no token histogram on a failed stream")

	spans := sr.Ended()
	require.Len(t, spans, 1)
	assert.NotEmpty(t, spans[0].Events(), "error recorded on the span")
}

func TestRegisterMetrics(t *testing.T) {
	r := &fakeRegistrar{}
	RegisterMetrics(r)

	assert.Contains(t, r.counters, metricRequestCount)
	assert.Contains(t, r.histograms, metricTokensPerRequest)
}

type fakeRegistrar struct {
	counters   []string
	histograms []string
}

func (f *fakeRegistrar) NewCounter(name, _ string) { f.counters = append(f.counters, name) }
func (f *fakeRegistrar) NewHistogram(name, _ string, _ ...float64) {
	f.histograms = append(f.histograms, name)
}
