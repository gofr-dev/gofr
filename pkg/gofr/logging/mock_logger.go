package logging

import (
	"fmt"
	"io"
	"os"
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

func (m *MockLogger) logf(level Level, format string, args ...interface{}) {
	if level < m.level {
		return
	}

	out := m.out
	if level == ERROR {
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

func (m *MockLogger) Debug(args ...interface{}) {
	m.logf(DEBUG, "%v", args...) // Add "%v" formatting directive
}

func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.logf(DEBUG, format, args...)
}

func (m *MockLogger) Info(args ...interface{}) {
	m.logf(INFO, "%v", args...) // Add "%v" formatting directive
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.logf(INFO, format, args...)
}

func (m *MockLogger) Notice(args ...interface{}) {
	m.logf(NOTICE, "%v", args...) // Add "%v" formatting directive
}

func (m *MockLogger) Noticef(format string, args ...interface{}) {
	m.logf(NOTICE, format, args...)
}

func (m *MockLogger) Warn(args ...interface{}) {
	m.logf(WARN, "%v", args...)
}

func (m *MockLogger) Warnf(format string, args ...interface{}) {
	m.logf(WARN, format, args...)
}

func (m *MockLogger) Error(args ...interface{}) {
	m.logf(ERROR, "%v", args...)
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.logf(ERROR, format, args...)
}

func (m *MockLogger) Fatal(args ...interface{}) {
	m.logf(FATAL, "%v", args...)
}

func (m *MockLogger) Fatalf(format string, args ...interface{}) {
	m.logf(FATAL, format, args...)
}

func (m *MockLogger) Log(args ...interface{}) {
	m.logf(INFO, "%v", args...)
}

func (m *MockLogger) Logf(format string, args ...interface{}) {
	m.logf(INFO, format, args...)
}

func (m *MockLogger) ChangeLevel(level Level) {
	m.level = level
}
