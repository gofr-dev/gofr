package observability

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// ANSI Color Codes.
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
	Gray   = "\033[90m"
)

const (
	INFO  = "INFO"
	WARN  = "WARN"
	ERROR = "ERROR"
	DEBUG = "DEBUG"
)

var ansiRegex = regexp.MustCompile("[\u001B\u009B][[]()#;?]*.{0,2}(?:(?:;\\d{1,3})*.[a-zA-Z\\d]|(?:\\d{1,4}/?)*[a-zA-Z])")

// Logger defines a standard interface for logging with built-in context awareness.
type Logger interface {
	// Standard logging methods with context support
	Errorf(ctx context.Context, format string, args ...any)
	Warnf(ctx context.Context, format string, args ...any)
	Infof(ctx context.Context, format string, args ...any)
	Debugf(ctx context.Context, format string, args ...any)

	// Cache-specific logging methods
	Hitf(ctx context.Context, message string, duration time.Duration, operation string)
	Missf(ctx context.Context, message string, duration time.Duration, operation string)

	// Generic structured logging method
	LogRequest(ctx context.Context, level, message string, tag any, duration time.Duration, operation string)
}

type nopLogger struct{}

func NewNopLogger() Logger { return &nopLogger{} }

func (*nopLogger) Errorf(_ context.Context, _ string, _ ...any)                                {}
func (*nopLogger) Warnf(_ context.Context, _ string, _ ...any)                                 {}
func (*nopLogger) Infof(_ context.Context, _ string, _ ...any)                                 {}
func (*nopLogger) Debugf(_ context.Context, _ string, _ ...any)                                {}
func (*nopLogger) Hitf(_ context.Context, _ string, _ time.Duration, _ string)                 {}
func (*nopLogger) Missf(_ context.Context, _ string, _ time.Duration, _ string)                {}
func (*nopLogger) LogRequest(_ context.Context, _, _ string, _ any, _ time.Duration, _ string) {}

type styledLogger struct {
	useColors bool
}

func NewStdLogger() Logger {
	return &styledLogger{
		useColors: isTerminal(),
	}
}

func (l *styledLogger) getTraceString(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	sc := trace.SpanFromContext(ctx).SpanContext()
	if sc.IsValid() {
		return " " + l.applyColor(Gray, sc.TraceID().String())
	}

	return ""
}

func (l *styledLogger) Errorf(ctx context.Context, format string, args ...any) {
	l.logSimple(ctx, ERROR, Red, format, args...)
}

func (l *styledLogger) Warnf(ctx context.Context, format string, args ...any) {
	l.logSimple(ctx, WARN, Yellow, format, args...)
}

func (l *styledLogger) Infof(ctx context.Context, format string, args ...any) {
	l.logSimple(ctx, INFO, Green, format, args...)
}

func (l *styledLogger) Debugf(ctx context.Context, format string, args ...any) {
	l.logSimple(ctx, DEBUG, Gray, format, args...)
}

func (l *styledLogger) Hitf(ctx context.Context, _ string, duration time.Duration, operation string) {
	l.LogRequest(ctx, INFO, "Cache hit", "HIT", duration, operation)
}

func (l *styledLogger) Missf(ctx context.Context, _ string, duration time.Duration, operation string) {
	// A miss isn't an error, but we'll color its tag yellow for attention.
	l.LogRequest(ctx, INFO, "Cache miss", "MISS", duration, operation)
}

func (l *styledLogger) LogRequest(ctx context.Context, level, message string, tag any, duration time.Duration, operation string) {
	const tagColumnStart = 45

	const durationColumnStart = 60

	levelStr, levelColor := getLevelStyle(level)
	ts := l.applyColor(Gray, "["+time.Now().Format(time.TimeOnly)+"]")
	traceStr := l.getTraceString(ctx)
	initialPart := fmt.Sprintf("%s %s%s %s", l.applyColor(levelColor, levelStr), ts, traceStr, message)

	tagStr := l.formatTag(tag)
	durationStr := l.applyColor(Gray, fmt.Sprintf("%dµs", duration.Microseconds()))

	padding1 := getPadding(tagColumnStart, len(stripAnsi(initialPart)))
	padding2 := getPadding(durationColumnStart, tagColumnStart+len(stripAnsi(tagStr)))

	fmt.Printf("%s%s%s%s%s %s\n",
		initialPart,
		padding1,
		tagStr,
		padding2,
		durationStr,
		operation,
	)
}

func (l *styledLogger) logSimple(ctx context.Context, level, color, format string, args ...any) {
	levelStr := l.applyColor(color, level)
	ts := l.applyColor(Gray, "["+time.Now().Format(time.TimeOnly)+"]")
	traceStr := l.getTraceString(ctx)
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s%s %s\n", levelStr, ts, traceStr, msg)
}

func getLevelStyle(level string) (levelStr, color string) {
	switch level {
	case ERROR:
		return "ERROR", Red
	case WARN:
		return "WARN", Yellow
	case INFO:
		return "INFO", Green
	case DEBUG:
		return "DEBUG", Gray
	default:
		return level, Reset
	}
}

func (l *styledLogger) formatTag(tag any) string {
	switch t := tag.(type) {
	case int:
		return l.formatIntTag(t)
	case string:
		return l.formatStringTag(t)
	default:
		return fmt.Sprint(tag)
	}
}

const (
	StatusOKRangeStart          = 200
	StatusOKRangeEnd            = 300
	StatusClientErrorRangeStart = 400
	StatusClientErrorRangeEnd   = 500
	StatusServerErrorRangeStart = 500
)

func (l *styledLogger) formatIntTag(t int) string {
	var color string
	if t >= StatusOKRangeStart && t < StatusOKRangeEnd {
		color = Green
	} else if t >= StatusClientErrorRangeStart && t < StatusClientErrorRangeEnd {
		color = Yellow
	} else if t >= StatusServerErrorRangeStart {
		color = Red
	} else {
		color = Gray
	}

	return l.applyColor(color, fmt.Sprintf("%d", t))
}

func (l *styledLogger) formatStringTag(t string) string {
	var color string

	switch t {
	case "HIT":
		color = Green
	case "MISS":
		color = Yellow
	case "REDIS":
		color = Blue
	case "SQL":
		color = Cyan
	default:
		color = Gray
	}

	return l.applyColor(color, t)
}

func getPadding(columnTarget, currentLength int) string {
	if padLen := columnTarget - currentLength; padLen > 0 {
		return strings.Repeat(" ", padLen)
	}

	return " "
}

func (l *styledLogger) applyColor(color, text string) string {
	if !l.useColors {
		return text
	}

	return color + text + Reset
}

func isTerminal() bool {
	fi, _ := os.Stdout.Stat()
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// stripAnsi removes ANSI escape codes from a string.
func stripAnsi(str string) string {
	return regexp.MustCompile(ansiRegex).ReplaceAllString(str, "")
}
