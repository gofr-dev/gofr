package testutil

import (
	"fmt"
	"io"
	"os"

	"gofr.dev/pkg/gofr/logging"
)

const (
	DEBUGLOG = iota + 1
	INFOLOG
	NOTICELOG
	WARNLOG
	ERRORLOG
	FATALLOG
)

type MockLogger struct {
	level  int
	out    io.Writer
	errOut io.Writer
}

func (m *MockLogger) changeLevel(level logging.Level) {
	m.level = int(level)
}

func NewMockLogger(level int) *MockLogger {
	return &MockLogger{
		level:  level,
		out:    os.Stdout,
		errOut: os.Stderr,
	}
}

func (m *MockLogger) Info(args ...interface{}) {
	m.logf(INFOLOG, "%v", args...) // Add "%v" formatting directive
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.logf(INFOLOG, format, args...)
}

func (m *MockLogger) Fatal(args ...interface{}) {
	m.logf(FATALLOG, "%v", args...) // Add "%v" formatting directive
}
func (m *MockLogger) Fatalf(format string, args ...interface{}) {
	m.logf(FATALLOG, format, args...)
}

func (m *MockLogger) Notice(args ...interface{}) {
	m.logf(NOTICELOG, "%v", args...) // Add "%v" formatting directive
}
func (m *MockLogger) Noticef(format string, args ...interface{}) {
	m.logf(NOTICELOG, format, args...)
}

func (m *MockLogger) Debug(args ...interface{}) {
	m.logf(DEBUGLOG, "%v", args...) // Add "%v" formatting directive
}

func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.logf(DEBUGLOG, format, args...)
}

func (m *MockLogger) Log(args ...interface{}) {
	m.logf(INFOLOG, "%v", args...)
}

func (m *MockLogger) Logf(format string, args ...interface{}) {
	m.logf(INFOLOG, format, args...)
}

func (m *MockLogger) Warn(args ...interface{}) {
	m.logf(WARNLOG, "%v", args...)
}
func (m *MockLogger) Warnf(format string, args ...interface{}) {
	m.logf(WARNLOG, format, args...)
}

func (m *MockLogger) Error(args ...interface{}) {
	m.logf(ERRORLOG, "%v", args...)
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.logf(ERRORLOG, format, args...)
}

func (m *MockLogger) logf(level int, format string, args ...interface{}) {
	if level < m.level {
		return
	}

	out := m.out
	if level == ERRORLOG {
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
