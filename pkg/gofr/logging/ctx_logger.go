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
	base    Logger
	traceID string
	// traceArg is the precomputed {"__trace_id__": traceID} marker map, built
	// once when the ContextLogger is constructed (i.e. once per request) rather
	// than on every log call. It is read-only downstream (the base logger only
	// reads the key out and drops the map), so sharing the single instance
	// across all log calls of this request is safe. nil when no valid trace ID
	// is in scope.
	traceArg map[string]any
}

// NewContextLogger creates a new ContextLogger that wraps the provided base logger
// and automatically appends OpenTelemetry trace information (trace ID) to log output
// when available in the context.
func NewContextLogger(ctx context.Context, base Logger) *ContextLogger {
	var traceID string

	sc := trace.SpanFromContext(ctx).SpanContext()

	if sc.IsValid() {
		traceID = sc.TraceID().String()
	}

	cl := &ContextLogger{base: base, traceID: traceID}

	if traceID != "" {
		cl.traceArg = map[string]any{traceIDMarkerKey: traceID}
	}

	return cl
}

// withTraceInfo appends the trace ID from the context (if available).
// This allows trace IDs to be extracted later during formatting or filtering.
// The marker map is precomputed once per ContextLogger, so this only pays for
// the slice append, not a fresh map allocation on every call.
func (l *ContextLogger) withTraceInfo(args ...any) []any {
	if l.traceArg != nil {
		return append(args, l.traceArg)
	}

	return args
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
