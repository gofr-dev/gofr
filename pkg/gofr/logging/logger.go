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

type PrettyPrint interface {
	PrettyPrint(writer io.Writer)
}

// Logger represents a logging interface.
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

	switch {
	case len(args) == 1 && format == "":
		entry.Message = args[0]
	case len(args) != 1 && format == "":
		entry.Message = args
	case format != "":
		entry.Message = fmt.Sprintf(format+"", args...) // TODO - this is stupid. We should not need empty string.
	}

	if l.isTerminal {
		l.prettyPrint(entry, out)
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

	//nolint:revive // exit status is 1 as it denotes failure as signified by Fatal log
	os.Exit(1)
}

func (l *logger) Fatalf(format string, args ...any) {
	l.logf(FATAL, format, args...)

	//nolint:revive // exit status is 1 as it denotes failure as signified by Fatal log
	os.Exit(1)
}

func (l *logger) prettyPrint(e logEntry, out io.Writer) {
	// Note: we need to lock the pretty print as printing to standard output not concurrency safe
	// the logs when printed in go routines were getting misaligned since we are achieving
	// a single line of log, in 2 separate statements which caused the misalignment.
	l.lock <- struct{}{} // Acquire the channel's lock
	defer func() {
		<-l.lock // Release the channel's token
	}()

	// Pretty printing if the message interface defines a method PrettyPrint else print the log message
	// This decouples the logger implementation from its usage
	if fn, ok := e.Message.(PrettyPrint); ok {
		fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] ", e.Level.color(), e.Level.String()[0:4],
			e.Time.Format(time.TimeOnly))

		fn.PrettyPrint(out)
	} else {
		fmt.Fprintf(out, "\u001B[38;5;%dm%s\u001B[0m [%s] ", e.Level.color(), e.Level.String()[0:4],
			e.Time.Format(time.TimeOnly))

		fmt.Fprintf(out, "%v\n", e.Message)
	}
}

// NewLogger creates a new logger instance with the specified logging level.
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

// NewFileLogger creates a new logger instance with logging to a file.
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
	switch v := w.(type) {
	case *os.File:
		return term.IsTerminal(int(v.Fd()))
	default:
		return false
	}
}

func (l *logger) ChangeLevel(level Level) {
	l.level = level
}

// LogLevelResponder is an interface that provides a method to get the log level.
type LogLevelResponder interface {
	LogLevel() Level
}

// GetLogLevelForError returns the log level for the given error.
// If the error implements [logLevelResponder], its log level is returned.
// Otherwise, the default log level "error" is returned.
func GetLogLevelForError(err error) Level {
	level := ERROR

	if e, ok := err.(LogLevelResponder); ok {
		level = e.LogLevel()
	}

	return level
}
