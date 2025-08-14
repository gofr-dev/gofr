package observability

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
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

const ansiRegex = "[\u001B\u009B][[\\]()#;?]*.{0,2}(?:(?:;\\d{1,3})*.[a-zA-Z\\d]|(?:\\d{1,4}/?)*[a-zA-Z])"

// Logger defines a standard interface for logging.
type Logger interface {
	Errorf(format string, args ...any)
	Warnf(format string, args ...any)
	Infof(format string, args ...any)
	Debugf(format string, args ...any)

	Hitf(message string, duration time.Duration, operation string)
	Missf(message string, duration time.Duration, operation string)

	// Generic method for creating structured logs like in the screenshot.
	// level: "INFO", "DEBUG", etc.
	// message: The initial message string (e.g., request ID, query context).
	// tag: An int (like HTTP status 200) or string (like "SQL", "REDIS").
	// duration: The operation's duration.
	// operation: The final operation string (e.g., "GET /hello", "select 2+2").
	LogRequest(level, message string, tag any, duration time.Duration, operation string)
}

type nopLogger struct{}

func NewNopLogger() Logger { return &nopLogger{} }

func (*nopLogger) Errorf(_ string, _ ...any)                                {}
func (*nopLogger) Warnf(_ string, _ ...any)                                 {}
func (*nopLogger) Infof(_ string, _ ...any)                                 {}
func (*nopLogger) Debugf(_ string, _ ...any)                                {}
func (*nopLogger) Hitf(_ string, _ time.Duration, _ string)                 {}
func (*nopLogger) Missf(_ string, _ time.Duration, _ string)                {}
func (*nopLogger) LogRequest(_, _ string, _ any, _ time.Duration, _ string) {}

type styledLogger struct {
	useColors bool
}

func NewStdLogger() Logger {
	return &styledLogger{
		useColors: isTerminal(),
	}
}

func (l *styledLogger) Errorf(format string, args ...any) {
	l.logSimple(ERROR, Red, format, args...)
}

func (l *styledLogger) Warnf(format string, args ...any) {
	l.logSimple(WARN, Yellow, format, args...)
}

func (l *styledLogger) Infof(format string, args ...any) {
	l.logSimple(INFO, Green, format, args...)
}

func (l *styledLogger) Debugf(format string, args ...any) {
	l.logSimple(DEBUG, Gray, format, args...)
}

func (l *styledLogger) Hitf(_ string, duration time.Duration, operation string) {
	l.LogRequest(INFO, "Cache hit", "HIT", duration, operation)
}

func (l *styledLogger) Missf(_ string, duration time.Duration, operation string) {
	// A miss isn't an error, but we'll color its tag yellow for attention.
	l.LogRequest(INFO, "Cache miss", "MISS", duration, operation)
}

func (l *styledLogger) LogRequest(level, message string, tag any, duration time.Duration, operation string) {
	const tagColumnStart = 45

	const durationColumnStart = 60

	levelStr, levelColor := getLevelStyle(level)
	ts := l.applyColor(Gray, "["+time.Now().Format(time.TimeOnly)+"]")
	initialPart := fmt.Sprintf("%s %s %s", l.applyColor(levelColor, levelStr), ts, message)

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

func (l *styledLogger) logSimple(level, color, format string, args ...any) {
	levelStr := l.applyColor(color, level)
	ts := l.applyColor(Gray, "["+time.Now().Format(time.TimeOnly)+"]")
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s %s\n", levelStr, ts, msg)
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
		return fmt.Sprintf("%v", tag)
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
