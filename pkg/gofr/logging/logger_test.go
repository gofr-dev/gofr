package logging

import (
	"bytes"
	"encoding/json"
	"golang.org/x/term"
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/testutil"
)

func TestLogger_Log(t *testing.T) {
	testLogStatement := "hello log!"

	t.Setenv("LOG_LEVEL", "INFO")

	f := func() {
		logger := NewLogger()
		logger.Log(testLogStatement)
	}

	output := testutil.StdoutOutputForFunc(f)
	assertMessageInJSONLog(t, output, testLogStatement)
}

func TestLogger_Logf(t *testing.T) {
	testLogStatement := "hello logf!"

	t.Setenv("LOG_LEVEL", "INFO")

	f := func() {
		logger := NewLogger()
		logger.Logf("%s", testLogStatement)
	}

	output := testutil.StdoutOutputForFunc(f)

	assertMessageInJSONLog(t, output, testLogStatement)
}

func TestLogger_Info(t *testing.T) {
	testLogStatement := "hello info log!"

	t.Setenv("LOG_LEVEL", "INFO")

	f := func() {
		logger := NewLogger()
		logger.Info(testLogStatement)
	}

	output := testutil.StdoutOutputForFunc(f)
	assertMessageInJSONLog(t, output, testLogStatement)
}

func TestLogger_Infof(t *testing.T) {
	testLogStatement := "hello infof log!"

	t.Setenv("LOG_LEVEL", "INFO")

	f := func() {
		logger := NewLogger()
		logger.Infof(testLogStatement)
	}

	output := testutil.StdoutOutputForFunc(f)
	assertMessageInJSONLog(t, output, testLogStatement)
}

func TestLogger_Error(t *testing.T) {
	testLogStatement := "hello error log!"

	t.Setenv("LOG_LEVEL", "ERROR")

	f := func() {
		logger := NewLogger()
		logger.Error(testLogStatement)
	}

	output := testutil.StderrOutputForFunc(f)

	assertMessageInJSONLog(t, output, testLogStatement)
}

func TestLogger_Errorf(t *testing.T) {
	testLogStatement := "hello errorf log!"

	t.Setenv("LOG_LEVEL", "ERROR")

	f := func() {
		logger := NewLogger()
		logger.Errorf("%s", testLogStatement)
	}

	output := testutil.StderrOutputForFunc(f)

	assertMessageInJSONLog(t, output, testLogStatement)
}

func TestLogger_Debug(t *testing.T) {
	testLogStatement := "hello debug log!"

	t.Setenv("LOG_LEVEL", "DEBUG")

	f := func() {
		logger := NewLogger()
		logger.Debug(testLogStatement)
	}

	output := testutil.StdoutOutputForFunc(f)

	assertMessageInJSONLog(t, output, testLogStatement)
}

func TestLogger_Debugf(t *testing.T) {
	testLogStatement := "hello debugf log!"

	t.Setenv("LOG_LEVEL", "DEBUG")

	f := func() {
		logger := NewLogger()
		logger.Debugf("%s", testLogStatement)
	}

	output := testutil.StdoutOutputForFunc(f)

	assertMessageInJSONLog(t, output, testLogStatement)
}

func assertMessageInJSONLog(t *testing.T, logLine, expectation string) {
	var l logEntry
	_ = json.Unmarshal([]byte(logLine), &l)

	if l.Message != expectation {
		t.Errorf("Log mismatch. Expected: %s Got: %s", expectation, l.Message)
	}
}

func TestGetLevel(t *testing.T) {
	tests := []struct {
		desc     string
		input    string
		expected level
	}{
		{"Valid INFO", "INFO", INFO},
		{"Valid WARN", "WARN", WARN},
		{"Valid FATAL", "FATAL", FATAL},
		{"Valid DEBUG", "DEBUG", DEBUG},
		{"Valid ERROR", "ERROR", ERROR},
		{"Invalid Level", "INVALID", INFO},
		{"Case Insensitive", "iNfO", INFO},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			result := getLevel(tc.input)
			assert.Equal(t, tc.expected, result, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestCheckIfTerminal(t *testing.T) {
	tests := []struct {
		desc       string
		writer     io.Writer
		isTerminal bool
	}{
		{"Terminal Writer", os.Stdout, term.IsTerminal(int(os.Stdout.Fd()))},
		{"Non-Terminal Writer", os.Stderr, term.IsTerminal(int(os.Stderr.Fd()))},
		{"Non-Terminal Writer (not *os.File)", &bytes.Buffer{}, false},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			result := checkIfTerminal(tc.writer)

			assert.Equal(t, tc.isTerminal, result, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func colorize(msg string, colorCode int) string {
	return "\x1b[" + strconv.Itoa(colorCode) + "m" + msg + "\x1b[0m"
}

func TestPrettyPrint(t *testing.T) {
	var testTime = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		desc       string
		entry      logEntry
		isTerminal bool
		expected   string
	}{
		{
			desc: "RequestLog in Terminal",
			entry: logEntry{
				Level:   INFO,
				Time:    testTime,
				Message: middleware.RequestLog{Response: 200, ResponseTime: 100, Method: "GET", URI: "/path"},
			},
			isTerminal: true,
			expected:   colorize("INFO", 36) + " [00:00:00] 200       100µs GET /path \n",
		},
		{
			desc: "Non-RequestLog in Terminal",
			entry: logEntry{
				Level:   INFO,
				Time:    testTime,
				Message: "Non-request log message",
			},
			isTerminal: true,
			expected:   colorize("INFO", 36) + " [00:00:00] Non-request log message\n",
		},
		{
			desc: "Non-RequestLog in Non-Terminal",
			entry: logEntry{
				Level:   INFO,
				Time:    testTime,
				Message: "Non-request log message",
			},
			isTerminal: false,
			expected:   colorize("INFO", 36) + " [00:00:00] Non-request log message\n",
		},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			out := &bytes.Buffer{}
			logger := &logger{isTerminal: tc.isTerminal}

			logger.prettyPrint(tc.entry, out)

			assert.Equal(t, tc.expected, out.String(), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
