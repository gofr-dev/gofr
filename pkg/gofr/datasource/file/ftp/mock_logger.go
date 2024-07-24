package ftp

import (
	"fmt"
	"io"
	"os"
)

// Level represents different logging levels.
type Level int

const (
	DEBUG Level = iota + 1
	INFO
	ERROR
)

type MockLogger struct {
	level  Level
	out    io.Writer
	errOut io.Writer
}

func NewMockLogger(level Level) Logger {
	return &MockLogger{
		level:  level,
		out:    os.Stdout,
		errOut: os.Stderr,
	}
}

func (m *MockLogger) Logf(pattern string, args ...interface{}) {
	m.logf(INFO, pattern, args...)
}

func (m *MockLogger) Errorf(pattern string, args ...interface{}) {
	m.logf(ERROR, pattern, args...)
}

func (m *MockLogger) logf(level Level, format string, args ...interface{}) {
	out := m.out
	if level == ERROR {
		out = m.errOut
	}

	message := fmt.Sprintf(format, args...)

	fmt.Fprintf(out, "%v\n", message)
}
