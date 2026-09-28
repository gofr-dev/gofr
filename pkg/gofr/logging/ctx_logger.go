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
	// spanCtx is the request's SpanContext, kept as a value (it allocates
	// nothing) rather than a formatted trace ID.
	//
	// Formatting the trace ID costs a 32-character string, and wrapping it for
	// the log args costs another allocation. Both were paid when the logger was
	// built -- that is, on every request -- but they are only ever consumed by a
	// log call. A handler that logs nothing, which is the common case on a hot
	// endpoint, paid for both and used neither.
	//
	// They are now built in withTraceInfo, per log call. A handler logging once
	// pays exactly what it did before; one logging repeatedly pays per call,
	// which is the deliberate trade for making the silent path free.
	spanCtx trace.SpanContext
}

// NewContextLogger creates a new ContextLogger that wraps the provided base logger
// and automatically appends OpenTelemetry trace information (trace ID) to log output
// when available in the context.
func NewContextLogger(ctx context.Context, base Logger) *ContextLogger {
	cl := ContextLoggerFor(ctx, base)

	return &cl
}

// ContextLoggerFor returns a ContextLogger by value.
//
// It exists because the per-request construction site stores the logger in a
// struct field, so the pointer returned by NewContextLogger is dereferenced and
// copied immediately — the heap allocation backing it is then garbage. Callers
// that need a pointer keep using NewContextLogger; callers that store a value
// use this and allocate nothing for the wrapper itself.
func ContextLoggerFor(ctx context.Context, base Logger) ContextLogger {
	return ContextLogger{base: base, spanCtx: trace.SpanFromContext(ctx).SpanContext()}
}

// withTraceInfo appends the trace ID from the context (if available).
// This allows trace IDs to be extracted later during formatting or filtering.
// The marker map is precomputed once per ContextLogger, so this only pays for
// the slice append, not a fresh map allocation on every call.
func (l *ContextLogger) withTraceInfo(args ...any) []any {
	if !l.spanCtx.IsValid() {
		return args
	}

	return append(args, traceIDMarker(l.spanCtx.TraceID().String()))
}

func (l *ContextLogger) logWithTraceID(lf func(args ...any), args ...any) {
	lf(l.withTraceInfo(args...)...)
}

func (l *ContextLogger) logWithTraceIDf(lf func(f string, args ...any), f string, args ...any) {
	lf(f, l.withTraceInfo(args...)...)
}

func (l *ContextLogger) Debug(args ...any)             { l.logWithTraceID(l.base.Debug, args...) }
func (l *ContextLogger) Debugf(f string, args ...any)  { l.logWithTraceIDf(l.base.Debugf, f, args...) }
func (l *ContextLogger) Log(args ...any)               { l.logWithTraceID(l.base.Log, args...) }
func (l *ContextLogger) Logf(f string, args ...any)    { l.logWithTraceIDf(l.base.Logf, f, args...) }
func (l *ContextLogger) Info(args ...any)              { l.logWithTraceID(l.base.Info, args...) }
func (l *ContextLogger) Infof(f string, args ...any)   { l.logWithTraceIDf(l.base.Infof, f, args...) }
func (l *ContextLogger) Notice(args ...any)            { l.logWithTraceID(l.base.Notice, args...) }
func (l *ContextLogger) Noticef(f string, args ...any) { l.logWithTraceIDf(l.base.Noticef, f, args...) }
func (l *ContextLogger) Warn(args ...any)              { l.logWithTraceID(l.base.Warn, args...) }
func (l *ContextLogger) Warnf(f string, args ...any)   { l.logWithTraceIDf(l.base.Warnf, f, args...) }
func (l *ContextLogger) Error(args ...any)             { l.logWithTraceID(l.base.Error, args...) }
func (l *ContextLogger) Errorf(f string, args ...any)  { l.logWithTraceIDf(l.base.Errorf, f, args...) }
func (l *ContextLogger) Fatal(args ...any)             { l.logWithTraceID(l.base.Fatal, args...) }
func (l *ContextLogger) Fatalf(f string, args ...any)  { l.logWithTraceIDf(l.base.Fatalf, f, args...) }
func (l *ContextLogger) ChangeLevel(level Level)       { l.base.ChangeLevel(level) }
