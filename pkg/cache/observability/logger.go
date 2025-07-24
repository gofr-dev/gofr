package observability

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// ANSI Color Codes
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	Cyan   = "\033[36m"
	Gray   = "\033[90m"
)

// Logger defines a standard interface for logging.
type Logger interface {
	// Methods for simple, single-line logs.
	Errorf(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Debugf(format string, args ...interface{})

	// Methods for structured operational logs (e.g., cache, HTTP, SQL).
	Hitf(message string, duration time.Duration, operation string)
	Missf(message string, duration time.Duration, operation string)

	// Generic method for creating structured logs like in the screenshot.
	// level: "INFO", "DEBU", etc.
	// message: The initial message string (e.g., request ID, query context).
	// tag: An int (like HTTP status 200) or string (like "SQL", "REDIS").
	// duration: The operation's duration.
	// operation: The final operation string (e.g., "GET /hello", "select 2+2").
	LogRequest(level, message string, tag interface{}, duration time.Duration, operation string)
}

// --- No-Op Logger ---

type nopLogger struct{}

func NewNopLogger() Logger { return &nopLogger{} }

func (n *nopLogger) Errorf(_ string, _ ...interface{})                                                 {}
func (n *nopLogger) Warnf(_ string, _ ...interface{})                                                  {}
func (n *nopLogger) Infof(_ string, _ ...interface{})                                                  {}
func (n *nopLogger) Debugf(_ string, _ ...interface{})                                                 {}
func (n *nopLogger) Hitf(_ string, _ time.Duration, _ string)                                          {}
func (n *nopLogger) Missf(_ string, _ time.Duration, _ string)                                         {}
func (n *nopLogger) LogRequest(_ string, _ string, _ interface{}, _ time.Duration, _ string) {}

// --- Standard Styled Logger ---

type styledLogger struct {
	useColors bool
}

func NewStdLogger() Logger {
	return &styledLogger{
		useColors: isTerminal(),
	}
}

// --- Public Methods ---

func (l *styledLogger) Errorf(format string, args ...interface{}) {
	l.logSimple("ERRO", Red, format, args...)
}

func (l *styledLogger) Warnf(format string, args ...interface{}) {
	l.logSimple("WARN", Yellow, format, args...)
}

func (l *styledLogger) Infof(format string, args ...interface{}) {
	l.logSimple("INFO", Green, format, args...)
}

func (l *styledLogger) Debugf(format string, args ...interface{}) {
	l.logSimple("DEBU", Gray, format, args...)
}

func (l *styledLogger) Hitf(message string, duration time.Duration, operation string) {
	l.LogRequest("DEBU", "Cache hit", "HIT", duration, operation)
}

func (l *styledLogger) Missf(message string, duration time.Duration, operation string) {
	// A miss isn't an error, but we'll color its tag yellow for attention.
	l.LogRequest("DEBU", "Cache miss", "MISS", duration, operation)
}

func (l *styledLogger) LogRequest(level, message string, tag interface{}, duration time.Duration, operation string) {
	// Column start positions for alignment. Adjust these values to your preference.
	const tagColumnStart = 45
	const durationColumnStart = 60

	// 1. Build the first part of the log: LEVEL [TIMESTAMP] MESSAGE
	levelStr, levelColor := l.getLevelStyle(level)
	ts := l.applyColor(Gray, "["+time.Now().Format("15:04:05")+"]")
	initialPart := fmt.Sprintf("%s %s %s", l.applyColor(levelColor, levelStr), ts, message)

	// 2. Format the tag (status code or string) with appropriate colors.
	tagStr := l.formatTag(tag)

	// 3. Format the duration.
	durationStr := l.applyColor(Gray, fmt.Sprintf("%dµs", duration.Microseconds()))

	// 4. Calculate padding to align the columns.
	// This requires stripping ANSI color codes to get the true visual length.
	padding1 := l.getPadding(tagColumnStart, len(stripAnsi(initialPart)))
	padding2 := l.getPadding(durationColumnStart, tagColumnStart+len(stripAnsi(tagStr)))

	// 5. Print the fully assembled and aligned log line.
	fmt.Printf("%s%s%s%s%s %s\n",
		initialPart,
		padding1,
		tagStr,
		padding2,
		durationStr,
		operation,
	)
}

// --- Private Helpers ---

// logSimple handles basic, unstructured log messages.
func (l *styledLogger) logSimple(level, color, format string, args ...interface{}) {
	levelStr := l.applyColor(color, level)
	ts := l.applyColor(Gray, "["+time.Now().Format("15:04:05")+"]")
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("%s %s %s\n", levelStr, ts, msg)
}

func (l *styledLogger) getLevelStyle(level string) (string, string) {
	switch level {
	case "ERRO":
		return "ERRO", Red
	case "WARN":
		return "WARN", Yellow
	case "INFO":
		return "INFO", Green
	case "DEBU":
		return "DEBU", Gray
	default:
		return level, Reset
	}
}

func (l *styledLogger) formatTag(tag interface{}) string {
	switch t := tag.(type) {
	case int: // Assumes HTTP status code
		var color string
		if t >= 200 && t < 300 {
			color = Green
		} else if t >= 400 && t < 500 {
			color = Yellow
		} else if t >= 500 {
			color = Red
		} else {
			color = Gray
		}
		return l.applyColor(color, fmt.Sprintf("%d", t))
	case string: // Assumes a string tag like "SQL", "REDIS", "HIT", "MISS"
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
	default:
		return fmt.Sprintf("%v", tag)
	}
}

// getPadding calculates the spaces needed to align text in columns.
func (l *styledLogger) getPadding(columnTarget, currentLength int) string {
	if padLen := columnTarget - currentLength; padLen > 0 {
		return strings.Repeat(" ", padLen)
	}
	return " " // Return at least one space if the message is too long.
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
	const ansiRegex = "[\u001B\u009B][[\\]()#;?]*.{0,2}(?:(?:;\\d{1,3})*.[a-zA-Z\\d]|(?:\\d{1,4}/?)*[a-zA-Z])"
	re := regexp.MustCompile(ansiRegex)
	return re.ReplaceAllString(str, "")
}
