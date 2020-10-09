package gofr

import (
	"fmt"
	"io"
	"os"
)

type Logger interface {
	Log(args ...interface{})
	Logf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

type logger struct {
	normalOut io.Writer
	errorOut  io.Writer
}

func (l *logger) Log(args ...interface{}) {
	fmt.Fprintln(l.normalOut, args...)
}

func (l *logger) Logf(format string, args ...interface{}) {
	fmt.Fprintf(l.normalOut, format, args...)
}

func (l *logger) Error(args ...interface{}) {
	fmt.Fprintln(l.errorOut, args...)
}

func (l *logger) Errorf(format string, args ...interface{}) {
	fmt.Fprintf(l.errorOut, format, args...)
}

func newLogger() Logger {
	return &logger{
		normalOut: os.Stdout,
		errorOut:  os.Stderr,
	}
}
