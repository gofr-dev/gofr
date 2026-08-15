package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
)

// mockLogger is a simple implementation of Logger interface for testing.
type mockLogger struct {
	logs []logEntry
}

func (m *mockLogger) Debug(args ...any) {
	m.logs = append(m.logs, logEntry{Level: DEBUG, Message: args})
}
func (m *mockLogger) Debugf(format string, _ ...any) {
	m.logs = append(m.logs, logEntry{Level: DEBUG, Message: format})
}
func (m *mockLogger) Log(args ...any) { m.logs = append(m.logs, logEntry{Level: INFO, Message: args}) }
func (m *mockLogger) Logf(format string, _ ...any) {
	m.logs = append(m.logs, logEntry{Level: INFO, Message: format})
}
func (m *mockLogger) Info(args ...any) { m.logs = append(m.logs, logEntry{Level: INFO, Message: args}) }
func (m *mockLogger) Infof(format string, _ ...any) {
	m.logs = append(m.logs, logEntry{Level: INFO, Message: format})
}
func (m *mockLogger) Notice(args ...any) {
	m.logs = append(m.logs, logEntry{Level: NOTICE, Message: args})
}
func (m *mockLogger) Noticef(format string, _ ...any) {
	m.logs = append(m.logs, logEntry{Level: NOTICE, Message: format})
}
func (m *mockLogger) Warn(args ...any) { m.logs = append(m.logs, logEntry{Level: WARN, Message: args}) }
func (m *mockLogger) Warnf(format string, _ ...any) {
	m.logs = append(m.logs, logEntry{Level: WARN, Message: format})
}
func (m *mockLogger) Error(args ...any) {
	m.logs = append(m.logs, logEntry{Level: ERROR, Message: args})
}
func (m *mockLogger) Errorf(format string, _ ...any) {
	m.logs = append(m.logs, logEntry{Level: ERROR, Message: format})
}
func (m *mockLogger) Fatal(args ...any) {
	m.logs = append(m.logs, logEntry{Level: FATAL, Message: args})
}
func (m *mockLogger) Fatalf(format string, _ ...any) {
	m.logs = append(m.logs, logEntry{Level: FATAL, Message: format})
}
func (*mockLogger) ChangeLevel(_ Level) {}

// mockTracerProvider creates a context with a valid trace ID for testing.
func mockTracedContext() (ctx context.Context, id string) {
	// Create a testing trace ID.
	traceID := trace.TraceID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10}
	spanID := trace.SpanID{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}

	ctx = context.Background()
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})

	// Create a new context with the SpanContext
	ctx = trace.ContextWithSpanContext(ctx, sc)

	return ctx, traceID.String()
}

func TestNewContextLogger(t *testing.T) {
	baseLogger := &mockLogger{}
	ctx := t.Context()

	ctxLogger := NewContextLogger(ctx, baseLogger)

	assert.Equal(t, baseLogger, ctxLogger.base)
}

func TestContextLogger_WithTraceInfo_NoTraceID(t *testing.T) {
	baseLogger := &mockLogger{}
	ctx := t.Context()

	ctxLogger := NewContextLogger(ctx, baseLogger)

	args := []any{"test message"}
	result := ctxLogger.withTraceInfo(args...)

	assert.Equal(t, args, result)
	assert.Len(t, result, 1)
}

func TestContextLogger_WithTraceInfo_WithTraceID(t *testing.T) {
	baseLogger := &mockLogger{}
	ctx, expectedTraceID := mockTracedContext()

	ctxLogger := NewContextLogger(ctx, baseLogger)

	args := []any{"test message"}
	result := ctxLogger.withTraceInfo(args...)

	assert.Len(t, result, 2)

	marker, ok := result[1].(traceIDMarker)
	require.True(t, ok, "Expected a traceIDMarker carrying the trace ID")
	assert.Equal(t, expectedTraceID, string(marker))
}

func TestContextLogger_LoggingMethods_NoTrace(t *testing.T) {
	baseLogger := &mockLogger{}
	ctx := t.Context()

	ctxLogger := NewContextLogger(ctx, baseLogger)

	ctxLogger.Debug("debug message")
	ctxLogger.Debugf("debug format %s", "message")
	ctxLogger.Info("info message")
	ctxLogger.Infof("info format %s", "message")
	ctxLogger.Log("log message")
	ctxLogger.Logf("log format %s", "message")
	ctxLogger.Notice("notice message")
	ctxLogger.Noticef("notice format %s", "message")
	ctxLogger.Warn("warn message")
	ctxLogger.Warnf("warn format %s", "message")
	ctxLogger.Error("error message")
	ctxLogger.Errorf("error format %s", "message")

	assert.Len(t, baseLogger.logs, 12)
}

func TestContextLogger_LoggingMethods_WithTrace(t *testing.T) {
	baseLogger := &mockLogger{}
	ctx, expectedTraceID := mockTracedContext()

	ctxLogger := NewContextLogger(ctx, baseLogger)

	ctxLogger.Info("info message")
	ctxLogger.Error("error message")

	require.Len(t, baseLogger.logs, 2)

	infoMsg, ok := baseLogger.logs[0].Message.([]any)
	require.True(t, ok, "Expected message to be []any")
	require.Len(t, infoMsg, 2)

	marker, ok := infoMsg[1].(traceIDMarker)
	require.True(t, ok, "Expected a traceIDMarker carrying the trace ID")
	assert.Equal(t, expectedTraceID, string(marker))

	errorMsg, ok := baseLogger.logs[1].Message.([]any)
	require.True(t, ok, "Expected message to be []any")
	require.Len(t, errorMsg, 2)

	marker, ok = errorMsg[1].(traceIDMarker)
	require.True(t, ok, "Expected a traceIDMarker carrying the trace ID")
	assert.Equal(t, expectedTraceID, string(marker))
}

func TestContextLogger_Integration(t *testing.T) {
	buf := &bytes.Buffer{}

	realLogger := &logger{
		level:      DEBUG,
		normalOut:  buf,
		errorOut:   buf,
		isTerminal: false,
		lock:       make(chan struct{}, 1),
	}

	ctx, expectedTraceID := mockTracedContext()

	ctxLogger := NewContextLogger(ctx, realLogger)

	ctxLogger.Info("test message")

	var logData map[string]any

	err := json.NewDecoder(bytes.NewReader(buf.Bytes())).Decode(&logData)
	require.NoError(t, err)

	traceID, ok := logData["trace_id"].(string)
	require.True(t, ok, "Expected trace_id to be a string in log output")
	assert.Equal(t, expectedTraceID, traceID)

	message, ok := logData["message"].(string)
	assert.True(t, ok, "Expected message to be a string")
	assert.Equal(t, "test message", message)

	level, ok := logData["level"].(string)
	assert.True(t, ok, "Expected level to be a string")
	assert.Equal(t, "INFO", level)
}

func TestContextLogger_ChangeLevel(t *testing.T) {
	baseLogger := &logger{
		level:      INFO,
		normalOut:  io.Discard,
		errorOut:   io.Discard,
		isTerminal: false,
		lock:       make(chan struct{}, 1),
	}

	ctx := t.Context()
	ctxLogger := NewContextLogger(ctx, baseLogger)

	ctxLogger.ChangeLevel(DEBUG)

	assert.Equal(t, DEBUG, baseLogger.level)
}

// TestContextLogger_TraceID_SurfacedAndReused verifies that a request-scoped
// ContextLogger lifts the trace ID to the top-level trace_id field on every
// call, and that reusing the precomputed marker map across calls is correct.
func TestContextLogger_TraceID_SurfacedAndReused(t *testing.T) {
	buf := &bytes.Buffer{}
	base := newBufLogger(buf)

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10},
		SpanID:     trace.SpanID{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8},
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	cl := NewContextLogger(ctx, base)
	cl.Info("first")
	cl.Info("second")

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, 2)

	want := "0102030405060708090a0b0c0d0e0f10"

	for i, line := range lines {
		var e wireLog
		require.NoError(t, json.Unmarshal([]byte(line), &e))
		assert.Equalf(t, want, e.TraceID, "line %d must carry the trace_id", i)
		// The __trace_id__ marker map must never leak into the message.
		assert.NotContains(t, line, "__trace_id__", "internal marker must not appear on the wire")
	}
}

// TestContextLogger_NoTrace_NoMarker verifies that without a valid span, no
// trace marker is attached and no trace_id is emitted.
func TestContextLogger_NoTrace_NoMarker(t *testing.T) {
	buf := &bytes.Buffer{}
	base := newBufLogger(buf)

	cl := NewContextLogger(context.Background(), base)
	assert.Nil(t, cl.traceArg, "no precomputed marker when there is no valid trace")

	cl.Info("x")

	var e wireLog
	require.NoError(t, json.Unmarshal(buf.Bytes(), &e))
	assert.Empty(t, e.TraceID)
	assert.Equal(t, "x", e.Message)
}

// benchSpanContext returns a context carrying a valid sampled SpanContext so
// the ContextLogger takes its trace-ID path.
func benchSpanContext() context.Context {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10},
		SpanID:     trace.SpanID{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8},
		TraceFlags: trace.FlagsSampled,
	})

	return trace.ContextWithSpanContext(context.Background(), sc)
}

// BenchmarkContextLoggerInfo measures a request-scoped log that carries a trace
// ID. The ContextLogger is built once (as it is per request) and used for many
// log calls — exactly the pattern the precomputed marker map optimizes.
func BenchmarkContextLoggerInfo(b *testing.B) {
	base := newBenchLogger(io.Discard)
	cl := NewContextLogger(benchSpanContext(), base)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cl.Info("request handled successfully")
	}
}
