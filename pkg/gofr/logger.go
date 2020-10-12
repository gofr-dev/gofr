package gofr

import (
	"fmt"
	"io"
	"os"
)

type Logger interface {
	Disable()
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

type logger struct {
	disabled  bool
	normalOut io.Writer
	errorOut  io.Writer
}

func (l *logger) Log(args ...interface{}) {
	if !l.disabled {
		fmt.Fprintln(l.normalOut, args...)
	}
}

func (l *logger) Logf(format string, args ...interface{}) {
	if !l.disabled {
		fmt.Fprintf(l.normalOut, format, args...)
	}
}

func (l *logger) Error(args ...interface{}) {
	if !l.disabled {
		fmt.Fprintln(l.errorOut, args...)
	}
}

func (l *logger) Errorf(format string, args ...interface{}) {
	if !l.disabled {
		fmt.Fprintf(l.errorOut, format, args...)
	}
}

func (l *logger) Disable() {
	l.disabled = true
}

func newLogger() Logger {
	return &logger{
		normalOut: os.Stdout,
		errorOut:  os.Stderr,
	}
}
