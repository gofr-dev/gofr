package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/term"

	"gofr.dev/pkg/gofr/version"
)

const fileMode = 0644

// PrettyPrint defines an interface for objects that can render
// themselves in a human-readable format to the provided writer.
type PrettyPrint interface {
	PrettyPrint(writer io.Writer)
}

// Logger defines the interface for structured logging across
// different log levels such as Debug, Info, Warn, Error, and Fatal.
// It allows formatted logs and log level control.
type Logger interface {
	Debug(args ...any)
	Debugf(format string, args ...any)
	Log(args ...any)
	Logf(format string, args ...any)
	Info(args ...any)
	Infof(format string, args ...any)
	Notice(args ...any)
	Noticef(format string, args ...any)
	Warn(args ...any)
	Warnf(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
	Fatal(args ...any)
	Fatalf(format string, args ...any)
	ChangeLevel(level Level)
}

type logger struct {
	level      Level
	normalOut  io.Writer
	errorOut   io.Writer
	isTerminal bool
	lock       chan struct{}
}

type logEntry struct {
	Level       Level     `json:"level"`
	Time        time.Time `json:"time"`
	Message     any       `json:"message"`
	TraceID     string    `json:"trace_id,omitempty"`
	GofrVersion string    `json:"gofrVersion"`
}

func (l *logger) logf(level Level, format string, args ...any) {
	if level < l.level {
		return
	}

	out := l.normalOut
	if level >= ERROR {
		out = l.errorOut
	}

	entry := logEntry{
		Level:       level,
		Time:        time.Now(),
		GofrVersion: version.Framework,
	}

	traceID, filteredArgs := extractTraceIDAndFilterArgs(args)
	entry.TraceID = traceID

	switch {
	case len(filteredArgs) == 1 && format == "":
		entry.Message = filteredArgs[0]
	case len(filteredArgs) != 1 && format == "":
		entry.Message = filteredArgs
	case format != "":
		entry.Message = fmt.Sprintf(format, filteredArgs...)
	}

	if l.isTerminal {
		l.prettyPrint(&entry, out)
	} else {
		_ = json.NewEncoder(out).Encode(entry)
	}
}

func (l *logger) Debug(args ...any) {
	l.logf(DEBUG, "", args...)
}

func (l *logger) Debugf(format string, args ...any) {
	l.logf(DEBUG, format, args...)
}

func (l *logger) Info(args ...any) {
	l.logf(INFO, "", args...)
}

func (l *logger) Infof(format string, args ...any) {
	l.logf(INFO, format, args...)
}

func (l *logger) Notice(args ...any) {
	l.logf(NOTICE, "", args...)
}

func (l *logger) Noticef(format string, args ...any) {
	l.logf(NOTICE, format, args...)
}

func (l *logger) Warn(args ...any) {
	l.logf(WARN, "", args...)
}

func (l *logger) Warnf(format string, args ...any) {
	l.logf(WARN, format, args...)
}

func (l *logger) Log(args ...any) {
	l.logf(INFO, "", args...)
}

func (l *logger) Logf(format string, args ...any) {
	l.logf(INFO, format, args...)
}

func (l *logger) Error(args ...any) {
	l.logf(ERROR, "", args...)
}

func (l *logger) Errorf(format string, args ...any) {
	l.logf(ERROR, format, args...)
}

func (l *logger) Fatal(args ...any) {
	l.logf(FATAL, "", args...)

	// Flush output before exiting
	if f, ok := l.errorOut.(*os.File); ok {
		_ = f.Sync() // Ignore sync error as we're about to exit
	}

	//nolint:revive // exit status is 1 as it denotes failure as signified by Fatal log
	os.Exit(1)
}

func (l *logger) Fatalf(format string, args ...any) {
	l.logf(FATAL, format, args...)

	// Flush output before exiting
	if f, ok := l.errorOut.(*os.File); ok {
		_ = f.Sync() // Ignore sync error as we're about to exit
	}

	//nolint:revive // exit status is 1 as it denotes failure as signified by Fatal log
	os.Exit(1)
}

func (l *logger) prettyPrint(e *logEntry, out io.Writer) {
	// Note: we need to lock the pretty print as printing to standard output not concurrency safe
	// the logs when printed in go routines were getting misaligned since we are achieving
	// a single line of log, in 2 separate statements which caused the misalignment.
	l.lock <- struct{}{} // Acquire the channel's lock

	defer func() {
		<-l.lock // Release the channel's token
	}()

	// Pretty printing if the message interface defines a method PrettyPrint else print the log message
	// This decouples the logger implementation from its usage
	fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s]", e.Level.color(), e.Level.String()[0:4], e.Time.Format(time.TimeOnly))

	if e.TraceID != "" {
		fmt.Fprintf(out, " \u001B[38;5;8m%s\u001B[0m", e.TraceID)
	}

	fmt.Fprint(out, " ")

	// Print the message
	if fn, ok := e.Message.(PrettyPrint); ok {
		fn.PrettyPrint(out)
	} else {
		fmt.Fprintf(out, "%v\n", e.Message)
	}
}

// NewLogger creates a new Logger instance configured with the given log level.
// Logs will be printed to stdout and stderr depending on the level.
func NewLogger(level Level) Logger {
	l := &logger{
		normalOut: os.Stdout,
		errorOut:  os.Stderr,
		lock:      make(chan struct{}, 1),
	}

	l.level = level

	l.isTerminal = checkIfTerminal(l.normalOut)

	return l
}

// NewFileLogger creates a new Logger instance that writes logs to the specified file.
// If the file cannot be opened or created, logs are discarded.
func NewFileLogger(path string) Logger {
	l := &logger{
		normalOut: io.Discard,
		errorOut:  io.Discard,
	}

	if path == "" {
		return l
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode)
	if err != nil {
		return l
	}

	l.normalOut = f
	l.errorOut = f

	return l
}

func checkIfTerminal(w io.Writer) bool {
	// Force JSON output in test environments
	if os.Getenv("GOFR_EXITER") == "1" {
		return false
	}

	switch v := w.(type) {
	case *os.File:
		return term.IsTerminal(int(v.Fd()))
	default:
		return false
	}
}

func (l *logger) IsTerminal() bool {
	return l.isTerminal
}

// ChangeLevel changes the log level of the logger.
// This allows dynamic adjustment of the logging verbosity.
func (l *logger) ChangeLevel(level Level) {
	l.level = level
}

// LogLevelResponder provides a method to get the log level.
type LogLevelResponder interface {
	LogLevel() Level
}

// GetLogLevelForError extracts the log level from an error if it implements LogLevelResponder.
// If the error does not implement this interface, it defaults to ERROR level.
// This is useful for determining the appropriate log level when handling errors.
func GetLogLevelForError(err error) Level {
	level := ERROR

	if e, ok := err.(LogLevelResponder); ok {
		level = e.LogLevel()
	}

	return level
}

// extractTraceIDAndFilterArgs checks if any of the arguments contain a trace ID
// under the key "__trace_id__" and returns the extracted trace ID along with
// the remaining arguments excluding the trace metadata.
func extractTraceIDAndFilterArgs(args []any) (traceID string, filtered []any) {
	filtered = make([]any, 0, len(args))

	for _, arg := range args {
		if m, ok := arg.(map[string]any); ok {
			if tid, exists := m["__trace_id__"].(string); exists && traceID == "" {
				traceID = tid

				continue
			}
		}

		filtered = append(filtered, arg)
	}

	return traceID, filtered
}
