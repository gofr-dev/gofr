package logging

import (
	"context"
	"go.opentelemetry.io/otel/trace"
)

type ContextLogger struct {
	base    Logger
	ctx     context.Context
	Dynamic bool
}

func NewContextLogger(ctx context.Context, base Logger) *ContextLogger {
	return &ContextLogger{base: base, ctx: ctx, Dynamic: true}
}

func (l *ContextLogger) withTraceInfo(args ...any) []any {
	var traceID string

	if l.Dynamic {
		if span := trace.SpanFromContext(l.ctx); span.IsRecording() {
			traceID = span.SpanContext().TraceID().String()
		}
	} else if l.ctx != nil {
		traceID = trace.SpanFromContext(l.ctx).SpanContext().TraceID().String()
	}

	if traceID != "" {
		return append(args, map[string]any{"__trace_id__": traceID})
	}
	return args
}

func (l *ContextLogger) SetContext(ctx context.Context) {
	if l.Dynamic {
		l.ctx = ctx
	}
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
