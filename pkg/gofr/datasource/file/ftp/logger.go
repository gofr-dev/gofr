package ftp

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

// Level represents different logging levels.
type Level int

const (
	DEBUG Level = iota + 1
	INFO
	ERROR
)

// FileLog handles logging with different levels.
type FileLog struct {
	Level     Level
	Operation string    `json:"operation"`
	Status    string    `json:"status,omitempty"`
	Message   string    `json:"message,omitempty"`
	Out       io.Writer `json:"out,omitempty"`
	ErrOut    io.Writer `json:"err_out,omitempty"`
}

// NewLogger creates a new FileLog instance.
func NewLogger(level Level) Logger {
	return &FileLog{
		Level:  level,
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}
}

// clean cleans up the query string.
func clean(query string) string {
	query = regexp.MustCompile(`\s+`).ReplaceAllString(query, " ")
	query = strings.TrimSpace(query)

	return query
}

// logf logs a message with the specified level and timestamp.
func (fl *FileLog) logf(level Level) {
	if level < fl.Level {
		return
	}

	out := fl.Out
	if level == ERROR {
		out = fl.ErrOut
	}

	timestamp := time.Now().Format(time.TimeOnly)

	message := clean(fl.Message)

	var levelColor string

	switch level {
	case DEBUG:
		levelColor = "\u001B[1;36m" // Cyan
	case INFO:
		levelColor = "\u001B[1;37m" // White
	case ERROR:
		levelColor = "\u001B[1;31m" // Red
	}

	fmt.Fprintf(out, "%s%-6s \u001B[1;35mFTP\u001B[0m \u001B[1;34m[%s]\u001B[1;32m %s \u001B[1;33m%s \u001B[0m%s\n",
		levelColor, levelString(level), timestamp, clean(fl.Operation), clean(fl.Status), message)
}

// levelString returns the string representation of the log level.
func levelString(level Level) string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case ERROR:
		return "ERROR"
	default:
		return ""
	}
}

// Debugf logs a debug message with a format string and arguments.
func (fl *FileLog) Debugf(operation, status, format string, args ...interface{}) {
	fl.Operation = operation
	fl.Status = status
	fl.Message = fmt.Sprintf(format, args...)
	fl.logf(DEBUG)
}

// Logf logs an info message with a format string and arguments.
func (fl *FileLog) Logf(operation, status, format string, args ...interface{}) {
	fl.Operation = operation
	fl.Status = status
	fl.Message = fmt.Sprintf(format, args...)
	fl.logf(INFO)
}

// Errorf logs an error message with a format string and arguments.
func (fl *FileLog) Errorf(operation, status, format string, args ...interface{}) {
	fl.Operation = operation
	fl.Status = status
	fl.Message = fmt.Sprintf(format, args...)
	fl.logf(ERROR)
}
