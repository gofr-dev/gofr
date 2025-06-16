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

func (m *MockLogger) logf(level Level, format string, args ...any) {
	if level < m.level {
		return
	}

	out := m.out
	if level == ERROR {
		out = m.errOut
	}

	var message any

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

func (m *MockLogger) Debug(args ...any) {
	m.logf(DEBUG, "%v", args...) // Add "%v" formatting directive
}

func (m *MockLogger) Debugf(format string, args ...any) {
	m.logf(DEBUG, format, args...)
}

func (m *MockLogger) Info(args ...any) {
	m.logf(INFO, "%v", args...) // Add "%v" formatting directive
}

func (m *MockLogger) Infof(format string, args ...any) {
	m.logf(INFO, format, args...)
}

func (m *MockLogger) Notice(args ...any) {
	m.logf(NOTICE, "%v", args...) // Add "%v" formatting directive
}

func (m *MockLogger) Noticef(format string, args ...any) {
	m.logf(NOTICE, format, args...)
}

func (m *MockLogger) Warn(args ...any) {
	m.logf(WARN, "%v", args...)
}

func (m *MockLogger) Warnf(format string, args ...any) {
	m.logf(WARN, format, args...)
}

func (m *MockLogger) Error(args ...any) {
	m.logf(ERROR, "%v", args...)
}

func (m *MockLogger) Errorf(format string, args ...any) {
	m.logf(ERROR, format, args...)
}

func (m *MockLogger) Fatal(args ...any) {
	m.logf(FATAL, "%v", args...)
}

func (m *MockLogger) Fatalf(format string, args ...any) {
	m.logf(FATAL, format, args...)
}

func (m *MockLogger) Log(args ...any) {
	m.logf(INFO, "%v", args...)
}

func (m *MockLogger) Logf(format string, args ...any) {
	m.logf(INFO, format, args...)
}

func (m *MockLogger) ChangeLevel(level Level) {
	m.level = level
}
