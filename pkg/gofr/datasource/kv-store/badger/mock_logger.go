package badger

import (
	"bytes"
	"fmt"
	"io"
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

func NewMockLogger(level Level, b *bytes.Buffer) Logger {
	return &MockLogger{
		level:  level,
		out:    b,
		errOut: b,
	}
}

func (m *MockLogger) Debugf(pattern string, args ...interface{}) {
	m.logf(DEBUG, pattern, args...)
}

func (m *MockLogger) Debug(args ...interface{}) {
	m.log(DEBUG, args...)
}

func (m *MockLogger) Infof(pattern string, args ...interface{}) {
	m.logf(INFO, pattern, args...)
}

func (m *MockLogger) Info(args ...interface{}) {
	m.log(INFO, args...)
}

func (m *MockLogger) Errorf(patter string, args ...interface{}) {
	m.logf(ERROR, patter, args...)
}

func (m *MockLogger) Error(args ...interface{}) {
	m.log(ERROR, args...)
}

func (m *MockLogger) logf(level Level, format string, args ...interface{}) {
	out := m.out
	if level == ERROR {
		out = m.errOut
	}

	fmt.Fprintf(out, format+"\n", args...)
}

func (m *MockLogger) log(level Level, args ...interface{}) {
	out := m.out
	if level == ERROR {
		out = m.errOut
	}

	message := fmt.Sprint(args...)

	fmt.Fprintf(out, "%v\n", message)
}
