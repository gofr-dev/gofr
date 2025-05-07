package logging

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// ContextLogger is a wrapper around a base Logger that injects the current
// trace ID (if present in the context) into log messages automatically.
//
// It is intended for use within request-scoped contexts where OpenTelemetry
// trace information is available.
type ContextLogger struct {
	base Logger
	ctx  context.Context
}

// NewContextLogger creates a new ContextLogger that wraps the provided base logger
// and automatically appends OpenTelemetry trace information (trace ID) to log output
// when available in the context.
func NewContextLogger(ctx context.Context, base Logger) *ContextLogger {
	return &ContextLogger{base: base, ctx: ctx}
}

// withTraceInfo appends the trace ID from the context (if available).
// This allows trace IDs to be extracted later during formatting or filtering.
func (l *ContextLogger) withTraceInfo(args ...any) []any {
	sc := trace.SpanFromContext(l.ctx).SpanContext()
	if sc.IsValid() {
		return append(args, map[string]any{"__trace_id__": sc.TraceID().String()})
	}

	return args
}

func (l *ContextLogger) Debug(args ...any)            { l.base.Debug(l.withTraceInfo(args...)...) }
func (l *ContextLogger) Debugf(f string, args ...any) { l.base.Debugf(f, l.withTraceInfo(args...)...) }
func (l *ContextLogger) Log(args ...any)              { l.base.Log(l.withTraceInfo(args...)...) }
func (l *ContextLogger) Logf(f string, args ...any)   { l.base.Logf(f, l.withTraceInfo(args...)...) }
func (l *ContextLogger) Info(args ...any)             { l.base.Info(l.withTraceInfo(args...)...) }
func (l *ContextLogger) Infof(f string, args ...any)  { l.base.Infof(f, l.withTraceInfo(args...)...) }
func (l *ContextLogger) Notice(args ...any)           { l.base.Notice(l.withTraceInfo(args...)...) }
func (l *ContextLogger) Noticef(f string, args ...any) {
	l.base.Noticef(f, l.withTraceInfo(args...)...)
}
func (l *ContextLogger) Warn(args ...any)             { l.base.Warn(l.withTraceInfo(args...)...) }
func (l *ContextLogger) Warnf(f string, args ...any)  { l.base.Warnf(f, l.withTraceInfo(args...)...) }
func (l *ContextLogger) Error(args ...any)            { l.base.Error(l.withTraceInfo(args...)...) }
func (l *ContextLogger) Errorf(f string, args ...any) { l.base.Errorf(f, l.withTraceInfo(args...)...) }
func (l *ContextLogger) Fatal(args ...any)            { l.base.Fatal(l.withTraceInfo(args...)...) }
func (l *ContextLogger) Fatalf(f string, args ...any) { l.base.Fatalf(f, l.withTraceInfo(args...)...) }
func (l *ContextLogger) ChangeLevel(level Level)      { l.base.ChangeLevel(level) }
