package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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

	traceMap, ok := result[1].(map[string]any)
	require.True(t, ok, "Expected a map with trace ID")

	traceID, ok := traceMap["__trace_id__"].(string)
	require.True(t, ok, "Expected a string trace ID")
	assert.Equal(t, expectedTraceID, traceID)
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

	traceMap, ok := infoMsg[1].(map[string]any)
	require.True(t, ok, "Expected a map with trace ID")

	traceID, ok := traceMap["__trace_id__"].(string)
	require.True(t, ok, "Expected a string trace ID")
	assert.Equal(t, expectedTraceID, traceID)

	errorMsg, ok := baseLogger.logs[1].Message.([]any)
	require.True(t, ok, "Expected message to be []any")
	require.Len(t, errorMsg, 2)

	traceMap, ok = errorMsg[1].(map[string]any)
	require.True(t, ok, "Expected a map with trace ID")

	traceID, ok = traceMap["__trace_id__"].(string)
	require.True(t, ok, "Expected a string trace ID")
	assert.Equal(t, expectedTraceID, traceID)
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
