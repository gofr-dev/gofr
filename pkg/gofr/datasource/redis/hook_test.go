package redis

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	otel "github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/mock/gomock"
)

func TestQueryLog_PrettyPrint(t *testing.T) {
	testCases := []struct {
		desc   string
		ql     *QueryLog
		expOut []string
	}{
		{
			desc: "pipeline",
			ql: &QueryLog{
				Query:    "pipeline",
				Duration: 112,
				Args:     []any{"[", "set a", "get a", "ex 300: OK", "]"},
			},
			expOut: []string{"pipeline", "112", "REDIS", "set a", "get a"},
		},
		{
			desc: "single command",
			ql: &QueryLog{
				Query:    "get",
				Duration: 22,
				Args:     []any{"get", "key1"},
			},
			expOut: []string{"get", "REDIS", "22", "get key1"},
		},
	}

	for _, tc := range testCases {
		b := new(bytes.Buffer)
		tc.ql.PrettyPrint(b)

		out := b.String()

		for _, v := range tc.expOut {
			assert.Contains(t, out, v)
		}
	}
}

// queryLogRecorder is a datasource.Logger that keeps every *QueryLog passed to
// Debug, so a test can assert on the logged Args rather than on printed output.
type queryLogRecorder struct {
	logs []*QueryLog
}

func (r *queryLogRecorder) Debug(args ...any) {
	for _, a := range args {
		if l, ok := a.(*QueryLog); ok {
			r.logs = append(r.logs, l)
		}
	}
}

func (*queryLogRecorder) Debugf(string, ...any) {}
func (*queryLogRecorder) Info(...any)           {}
func (*queryLogRecorder) Infof(string, ...any)  {}
func (*queryLogRecorder) Warn(...any)           {}
func (*queryLogRecorder) Warnf(string, ...any)  {}
func (*queryLogRecorder) Error(...any)          {}
func (*queryLogRecorder) Errorf(string, ...any) {}

// lastLog returns the most recent QueryLog whose Query is query.
func (r *queryLogRecorder) lastLog(t *testing.T, query string) *QueryLog {
	t.Helper()

	for i := len(r.logs) - 1; i >= 0; i-- {
		if r.logs[i].Query == query {
			return r.logs[i]
		}
	}

	t.Fatalf("no %q query was logged", query)

	return nil
}

func TestRedisHook_Args(t *testing.T) {
	const masked = redactedArg

	testCases := []struct {
		desc     string
		hideArgs bool
		run      func(ctx context.Context, rc *redis.Client) error
		query    string
		wantArgs any
	}{
		{
			desc:     "single command shows args by default",
			run:      func(ctx context.Context, rc *redis.Client) error { return rc.Set(ctx, "key", "value", 0).Err() },
			query:    "set",
			wantArgs: []any{"set", "key", "value"},
		},
		{
			desc:     "single command masks args when hidden",
			hideArgs: true,
			run:      func(ctx context.Context, rc *redis.Client) error { return rc.Set(ctx, "key", "value", 0).Err() },
			query:    "set",
			wantArgs: []any{"set", masked, masked},
		},
		{
			desc:     "pipeline masks args when hidden",
			hideArgs: true,
			run: func(ctx context.Context, rc *redis.Client) error {
				pipe := rc.Pipeline()
				pipe.Set(ctx, "key", "value", time.Minute)
				pipe.Get(ctx, "key")
				pipe.Incr(ctx, "counter")

				_, err := pipe.Exec(ctx)

				return err
			},
			query: queryPipeline,
			wantArgs: []any{[]string{
				"set " + masked + " " + masked + " " + masked + " " + masked,
				"get " + masked,
				"incr " + masked,
			}},
		},
		{
			desc: "pipeline logs every command",
			run: func(ctx context.Context, rc *redis.Client) error {
				_, err := rc.Pipelined(ctx, setThenGet(ctx))
				return err
			},
			query:    queryPipeline,
			wantArgs: []any{[]string{"set key value: OK", "get key: value"}},
		},
		{
			desc: "transaction pipeline logs every command by default",
			run: func(ctx context.Context, rc *redis.Client) error {
				_, err := rc.TxPipelined(ctx, setThenGet(ctx))
				return err
			},
			query: queryPipeline,
			// go-redis reads each +QUEUED reply into the MULTI command, so it ends as QUEUED.
			wantArgs: []any{[]string{"multi: QUEUED", "set key value: OK", "get key: value", "exec: []"}},
		},
		{
			desc:     "transaction pipeline logs multi and exec around its commands",
			hideArgs: true,
			run: func(ctx context.Context, rc *redis.Client) error {
				_, err := rc.TxPipelined(ctx, setThenGet(ctx))
				return err
			},
			query: queryPipeline,
			wantArgs: []any{[]string{
				"multi",
				"set " + masked + " " + masked,
				"get " + masked,
				"exec",
			}},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			rc, recorder := newHookedClient(t, tc.hideArgs)

			require.NoError(t, tc.run(t.Context(), rc))

			assertLoggedArgs(t, recorder.lastLog(t, tc.query), tc.wantArgs)
		})
	}
}

// assertLoggedArgs checks that logged prints wantArgs in every log format. The
// args may be built lazily, so it compares the printed output, not the values.
func assertLoggedArgs(t *testing.T, logged *QueryLog, wantArgs any) {
	t.Helper()

	want := &QueryLog{Query: logged.Query, Duration: logged.Duration, Args: wantArgs}
	assert.Equal(t, want.String(), logged.String())

	var gotOut, wantOut bytes.Buffer

	logged.PrettyPrint(&gotOut)
	want.PrettyPrint(&wantOut)
	assert.Equal(t, wantOut.String(), gotOut.String())

	// A container logs JSON, so the encoded args must match the plain values.
	gotJSON, err := json.Marshal(logged)
	require.NoError(t, err)

	wantJSON, err := json.Marshal(wantArgs)
	require.NoError(t, err)

	assert.Contains(t, string(gotJSON), `"args":`+string(wantJSON))
}

// discardLogger is a datasource.Logger that drops every entry, as the real
// logger does with a Debug entry when LOG_LEVEL is above DEBUG.
type discardLogger struct{}

func (discardLogger) Debug(...any)          {}
func (discardLogger) Debugf(string, ...any) {}
func (discardLogger) Info(...any)           {}
func (discardLogger) Infof(string, ...any)  {}
func (discardLogger) Warn(...any)           {}
func (discardLogger) Warnf(string, ...any)  {}
func (discardLogger) Error(...any)          {}
func (discardLogger) Errorf(string, ...any) {}

type discardMetrics struct{}

func (discardMetrics) RecordHistogram(context.Context, string, float64, ...string) {}
func (discardMetrics) IncrementCounter(context.Context, string, ...string)         {}

// A pipeline's log text must be built only when the entry is written, so a
// dropped entry costs the same however many commands the pipeline holds.
func TestProcessPipelineHook_DroppedEntryBuildsNoText(t *testing.T) {
	ctx := t.Context()
	hook := (&redisHook{config: &Config{}, logger: discardLogger{}, metrics: discardMetrics{}}).
		ProcessPipelineHook(func(context.Context, []redis.Cmder) error { return nil })

	pipeline := func(n int) []redis.Cmder {
		cmds := make([]redis.Cmder, n)
		for i := range cmds {
			cmds[i] = redis.NewStringCmd(ctx, "get", "key")
		}

		return cmds
	}

	small, large := pipeline(3), pipeline(12)

	smallAllocs := testing.AllocsPerRun(100, func() { _ = hook(ctx, small) })
	largeAllocs := testing.AllocsPerRun(100, func() { _ = hook(ctx, large) })

	assert.InDelta(t, smallAllocs, largeAllocs, 0, "allocations grow with the pipeline size")
}

// newHookedClient returns a client on a fresh miniredis with the GoFr hook
// added, and the recorder that the hook logs to.
func newHookedClient(t *testing.T, hideArgs bool) (*redis.Client, *queryLogRecorder) {
	t.Helper()

	s, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(s.Close)

	mockMetrics := NewMockMetrics(gomock.NewController(t))
	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(),
		"hostname", gomock.Any(), "type", gomock.Any()).AnyTimes()

	recorder := &queryLogRecorder{}
	rc := redis.NewClient(&redis.Options{Addr: s.Addr()})
	rc.AddHook(&redisHook{config: &Config{hideArgs: hideArgs}, logger: recorder, metrics: mockMetrics})

	t.Cleanup(func() { _ = rc.Close() })

	return rc, recorder
}

// setThenGet queues a SET and a GET of the same key on a pipeline.
func setThenGet(ctx context.Context) func(redis.Pipeliner) error {
	return func(pipe redis.Pipeliner) error {
		pipe.Set(ctx, "key", "value", 0)
		pipe.Get(ctx, "key")

		return nil
	}
}

func TestTracingOptions(t *testing.T) {
	testCases := []struct {
		desc          string
		hideArgs      bool
		wantStatement bool
	}{
		{desc: "statement recorded by default", hideArgs: false, wantStatement: true},
		{desc: "statement dropped when hidden", hideArgs: true, wantStatement: false},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			s, err := miniredis.Run()
			require.NoError(t, err)

			defer s.Close()

			exporter := tracetest.NewInMemoryExporter()
			tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

			rc := redis.NewClient(&redis.Options{Addr: s.Addr()})
			defer rc.Close()

			opts := append(tracingOptions(&Config{hideArgs: tc.hideArgs}), otel.WithTracerProvider(tp))
			require.NoError(t, otel.InstrumentTracing(rc, opts...))

			require.NoError(t, rc.Set(t.Context(), "secret-key", "secret-value", 0).Err())

			spans := exporter.GetSpans()

			var setSpan *tracetest.SpanStub

			for i := range spans {
				if spans[i].Name == "set" {
					setSpan = &spans[i]
				}
			}

			require.NotNil(t, setSpan, "no span named set was exported")

			assert.Equal(t, tc.wantStatement, hasAttribute(setSpan.Attributes, "db.statement"))
		})
	}
}

func hasAttribute(attrs []attribute.KeyValue, key attribute.Key) bool {
	for _, a := range attrs {
		if a.Key == key {
			return true
		}
	}

	return false
}
