package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/testutil"
	"golang.org/x/term"
)

func TestLogger_Log(t *testing.T) {
	testLogStatement := "hello log!"

	t.Setenv("LOG_LEVEL", "INFO")

	f := func() {
		logger := NewLogger(2)
		logger.Log(testLogStatement)
	}

	output := testutil.StdoutOutputForFunc(f)
	assertMessageInJSONLog(t, output, testLogStatement)
}

func TestLogger_Logf(t *testing.T) {
	testLogStatement := "hello logf!"

	t.Setenv("LOG_LEVEL", "INFO")

	f := func() {
		logger := NewLogger(2)
		logger.Logf("%s", testLogStatement)
	}

	output := testutil.StdoutOutputForFunc(f)

	assertMessageInJSONLog(t, output, testLogStatement)
}

func TestLogger_Info(t *testing.T) {
	testLogStatement := "hello info log!"

	t.Setenv("LOG_LEVEL", "INFO")

	f := func() {
		logger := NewLogger(2)
		logger.Info(testLogStatement)
	}

	output := testutil.StdoutOutputForFunc(f)
	assertMessageInJSONLog(t, output, testLogStatement)
}

func TestLogger_Infof(t *testing.T) {
	testLogStatement := "hello infof log!"

	t.Setenv("LOG_LEVEL", "INFO")

	f := func() {
		logger := NewLogger(2)
		logger.Infof(testLogStatement)
	}

	output := testutil.StdoutOutputForFunc(f)
	assertMessageInJSONLog(t, output, testLogStatement)
}

func TestLogger_Error(t *testing.T) {
	testLogStatement := "hello error log!"

	t.Setenv("LOG_LEVEL", "ERROR")

	f := func() {
		logger := NewLogger(5)
		logger.Error(testLogStatement)
	}

	output := testutil.StderrOutputForFunc(f)

	assertMessageInJSONLog(t, output, testLogStatement)
}

func TestLogger_Errorf(t *testing.T) {
	testLogStatement := "hello errorf log!"

	t.Setenv("LOG_LEVEL", "ERROR")

	f := func() {
		logger := NewLogger(5)
		logger.Errorf("%s", testLogStatement)
	}

	output := testutil.StderrOutputForFunc(f)

	assertMessageInJSONLog(t, output, testLogStatement)
}

func TestLogger_Debug(t *testing.T) {
	testLogStatement := "hello debug log!"

	t.Setenv("LOG_LEVEL", "DEBUG")

	f := func() {
		logger := NewLogger(1)
		logger.Debug(testLogStatement)
	}

	output := testutil.StdoutOutputForFunc(f)

	assertMessageInJSONLog(t, output, testLogStatement)
}

func TestLogger_Debugf(t *testing.T) {
	testLogStatement := "hello debugf log!"

	t.Setenv("LOG_LEVEL", "DEBUG")

	f := func() {
		logger := NewLogger(1)
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
		result := checkIfTerminal(tc.writer)

		assert.Equal(t, tc.isTerminal, result, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

// colorize function for consistent color output in tests.
func colorize(msg string, colorCode int) string {
	return fmt.Sprintf("\x1b[38;5;%dm%s\x1b[0m", colorCode, msg)
}

func TestPrettyPrint(t *testing.T) {
	var testTime = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		desc          string
		entry         logEntry
		isTerminal    bool
		expected      string
		expectedColor uint
	}{
		{
			desc: "RequestLog in Terminal",
			entry: logEntry{
				Level:   INFO,
				Time:    testTime,
				Message: middleware.RequestLog{Response: 200, ResponseTime: 100, Method: "GET", URI: "/path"},
			},
			isTerminal:    true,
			expected:      colorize("INFO", 6) + " [00:00:00] \x1b[38;5;8m \x1b[38;5;34m200\x1b[0m       100\x1b[38;5;8mµs\x1b[0m GET /path \n",
			expectedColor: 6,
		},
		{
			desc: "Non-Terminal Output",
			entry: logEntry{
				Level:   ERROR,
				Time:    testTime,
				Message: "Error message",
			},
			isTerminal:    false,
			expected:      colorize("ERRO", 160) + ` [00:00:00] Error message` + "\n",
			expectedColor: 160,
		},
	}

	for i, tc := range tests {
		out := &bytes.Buffer{}
		logger := &logger{isTerminal: tc.isTerminal}

		logger.prettyPrint(tc.entry, out)

		actual := out.String()

		// Assert both the formatted string and the color code
		assert.Equal(t, tc.expected, actual, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equal(t, tc.expectedColor, tc.entry.Level.color(), "Unexpected color code")
	}
}
