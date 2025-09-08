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
	reset  = "\033[0m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	cyan   = "\033[36m"
	gray   = "\033[90m"
)

const (
	infoLevel  = "INFO"
	warnLevel  = "WARN"
	errorLevel = "ERROR"
	debugLevel = "DEBUG"
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
		return " " + l.applyColor(gray, sc.TraceID().String())
	}

	return ""
}

func (l *styledLogger) Errorf(ctx context.Context, format string, args ...any) {
	l.logSimple(ctx, errorLevel, red, format, args...)
}

func (l *styledLogger) Warnf(ctx context.Context, format string, args ...any) {
	l.logSimple(ctx, warnLevel, yellow, format, args...)
}

func (l *styledLogger) Infof(ctx context.Context, format string, args ...any) {
	l.logSimple(ctx, infoLevel, green, format, args...)
}

func (l *styledLogger) Debugf(ctx context.Context, format string, args ...any) {
	l.logSimple(ctx, debugLevel, gray, format, args...)
}

func (l *styledLogger) Hitf(ctx context.Context, _ string, duration time.Duration, operation string) {
	l.LogRequest(ctx, infoLevel, "Cache hit", "HIT", duration, operation)
}

func (l *styledLogger) Missf(ctx context.Context, _ string, duration time.Duration, operation string) {
	// A miss isn't an error, but we'll color its tag yellow for attention.
	l.LogRequest(ctx, infoLevel, "Cache miss", "MISS", duration, operation)
}

func (l *styledLogger) LogRequest(ctx context.Context, level, message string, tag any, duration time.Duration, operation string) {
	const tagColumnStart = 45

	const durationColumnStart = 60

	levelStr, levelColor := getLevelStyle(level)
	ts := l.applyColor(gray, "["+time.Now().Format(time.TimeOnly)+"]")
	traceStr := l.getTraceString(ctx)
	initialPart := fmt.Sprintf("%s %s%s %s", l.applyColor(levelColor, levelStr), ts, traceStr, message)

	tagStr := l.formatTag(tag)
	durationStr := l.applyColor(gray, fmt.Sprintf("%dµs", duration.Microseconds()))

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
	ts := l.applyColor(gray, "["+time.Now().Format(time.TimeOnly)+"]")
	traceStr := l.getTraceString(ctx)
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s%s %s\n", levelStr, ts, traceStr, msg)
}

func getLevelStyle(level string) (levelStr, color string) {
	switch level {
	case errorLevel:
		return "ERROR", red
	case warnLevel:
		return "WARN", yellow
	case infoLevel:
		return "INFO", green
	case debugLevel:
		return "DEBUG", gray
	default:
		return level, reset
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
	statusOKRangeStart          = 200
	statusOKRangeEnd            = 300
	statusClientErrorRangeStart = 400
	statusClientErrorRangeEnd   = 500
	statusServerErrorRangeStart = 500
)

func (l *styledLogger) formatIntTag(t int) string {
	var color string
	if t >= statusOKRangeStart && t < statusOKRangeEnd {
		color = green
	} else if t >= statusClientErrorRangeStart && t < statusClientErrorRangeEnd {
		color = yellow
	} else if t >= statusServerErrorRangeStart {
		color = red
	} else {
		color = gray
	}

	return l.applyColor(color, fmt.Sprintf("%d", t))
}

func (l *styledLogger) formatStringTag(t string) string {
	var color string

	switch t {
	case "HIT":
		color = green
	case "MISS":
		color = yellow
	case "REDIS":
		color = blue
	case "SQL":
		color = cyan
	default:
		color = gray
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

	return color + text + reset
}

func isTerminal() bool {
	fi, _ := os.Stdout.Stat()
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// stripAnsi removes ANSI escape codes from a string.
func stripAnsi(str string) string {
	return ansiRegex.ReplaceAllString(str, "")
}
