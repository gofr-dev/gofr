package datasource

import (
	"fmt"
	"io"
	"os"
)

const (
	debugLevel = iota
	logLevel
	errorLevel
)

type MockLogger struct {
	level  int
	out    io.Writer
	errOut io.Writer
}

func NewMockLogger(level int) *MockLogger {
	return &MockLogger{
		level:  level,
		out:    os.Stdout,
		errOut: os.Stderr,
	}
}

func (m *MockLogger) Debug(args ...interface{}) {
	m.logf(debugLevel, "%v", args...) // Add "%v" formatting directive
}

func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.logf(debugLevel, format, args...)
}

func (m *MockLogger) Log(args ...interface{}) {
	m.logf(logLevel, "%v", args...)
}

func (m *MockLogger) Logf(format string, args ...interface{}) {
	m.logf(logLevel, format, args...)
}

func (m *MockLogger) Error(args ...interface{}) {
	m.logf(errorLevel, "%v", args...)
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.logf(errorLevel, format, args...)
}

func (m *MockLogger) logf(level int, format string, args ...interface{}) {
	if level < m.level {
		return
	}

	out := m.out
	if level == errorLevel {
		out = m.errOut
	}

	var message interface{}

	switch {
	case len(args) == 1 && format == "":
		message = args[0]
	case len(args) != 1 && format == "":
		message = args
	case format != "":
		message = fmt.Sprintf(format, args...)
	}

	fmt.Fprintf(out, "%v\n", message)
}
