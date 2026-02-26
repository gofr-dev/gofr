package remotelogger

import (
	"fmt"
	"strings"

	"gofr.dev/pkg/gofr/logging"
)

// testBufferLogger is a simple logger that writes to a buffer.
// It's primarily used in dynamic_level_logger_test.go for testing HTTPLogFilter and other log-related functionality,
// where direct capture of stdout would be insufficient.
// Unlike testutil.StdoutOutputForFunc, this logger provides isolation for component-specific testing and respects the logging levels.
type testBufferLogger struct {
	buf   *strings.Builder
	level logging.Level
}

func (l *testBufferLogger) Debug(args ...any) {
	if l.level <= logging.DEBUG {
		fmt.Fprintln(l.buf, args...)
	}
}

func (l *testBufferLogger) Logf(format string, args ...any) {
	fmt.Fprintf(l.buf, format+"\n", args...)
}

func (l *testBufferLogger) Info(args ...any) {
	if l.level <= logging.INFO {
		fmt.Fprintln(l.buf, args...)
	}
}

func (l *testBufferLogger) Notice(args ...any) {
	if l.level <= logging.NOTICE {
		fmt.Fprintln(l.buf, args...)
	}
}

func (l *testBufferLogger) Warn(args ...any) {
	if l.level <= logging.WARN {
		fmt.Fprintln(l.buf, args...)
	}
}

func (l *testBufferLogger) Error(args ...any) {
	if l.level <= logging.ERROR {
		fmt.Fprintln(l.buf, args...)
	}
}

func (l *testBufferLogger) Fatal(args ...any) {
	if l.level <= logging.FATAL {
		fmt.Fprintln(l.buf, args...)
	}
}

func (l *testBufferLogger) Log(args ...any) {
	fmt.Fprintln(l.buf, args...)
}

func (l *testBufferLogger) Infof(format string, args ...any) {
	fmt.Fprintf(l.buf, format+"\n", args...)
}

func (l *testBufferLogger) Debugf(format string, args ...any) {
	fmt.Fprintf(l.buf, format+"\n", args...)
}

func (l *testBufferLogger) Warnf(format string, args ...any) {
	fmt.Fprintf(l.buf, format+"\n", args...)
}

func (l *testBufferLogger) Errorf(format string, args ...any) {
	fmt.Fprintf(l.buf, format+"\n", args...)
}

func (l *testBufferLogger) Fatalf(format string, args ...any) {
	fmt.Fprintf(l.buf, format+"\n", args...)
}

func (l *testBufferLogger) Noticef(format string, args ...any) {
	fmt.Fprintf(l.buf, format+"\n", args...)
}

func (l *testBufferLogger) ChangeLevel(level logging.Level) {
	l.level = level
}
