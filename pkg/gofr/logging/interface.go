package logging

import (
	"io"
)

// PrettyPrint defines an interface for objects that can render
// themselves in a human-readable format to the provided writer.
type PrettyPrint interface {
	PrettyPrint(writer io.Writer)
}

// Logger interface with structured logging support
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Notice(args ...interface{})
	Noticef(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	ChangeLevel(level Level)
}

type LogLevelResponder interface {
	LogLevel() Level
}

func GetLogLevelForError(err error) Level {
	if e, ok := err.(LogLevelResponder); ok {
		return e.LogLevel()
	}
	return ERROR
}
