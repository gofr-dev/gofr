package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/http/middleware"
	"golang.org/x/term"
)

func TestLogger_LevelInfo(t *testing.T) {
	printLog := func() {
		logger := NewLogger(INFO)
		logger.Debug("Test Debug Log")
		logger.Info("Test Info Log")
		logger.Error("Test Error Log")
	}

	infoLog, errLog := captureLogOutput(printLog)

	assertMessageInJSONLog(t, infoLog, "Test Info Log")
	assertMessageInJSONLog(t, errLog, "Test Error Log")
}

func TestLogger_LevelError(t *testing.T) {
	printLog := func() {
		logger := NewLogger(ERROR)
		logger.Logf("%s", "Test Log")
		logger.Debugf("%s", "Test Debug Log")
		logger.Infof("%s", "Test Info Log")
		logger.Errorf("%s", "Test Error Log")
	}

	infoLog, errLog := captureLogOutput(printLog)

	assert.Equal(t, "", infoLog) // Since log level is ERROR we will not get any INFO logs.
	assertMessageInJSONLog(t, errLog, "Test Error Log")
}

func TestLogger_LevelDebug(t *testing.T) {
	printLog := func() {
		logger := NewLogger(DEBUG)
		logger.Logf("Test Log")
		logger.Debug("Test Debug Log")
		logger.Info("Test Info Log")
		logger.Error("Test Error Log")
	}

	infoLog, errLog := captureLogOutput(printLog)

	if !(strings.Contains(infoLog, "DEBUG") && strings.Contains(infoLog, "INFO")) {
		// Debug Log Level will contain all types of logs i.e. DEBUG, INFO and ERROR
		t.Errorf("TestLogger_LevelDebug Failed!")
	}

	assertMessageInJSONLog(t, errLog, "Test Error Log")
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

func TestPrettyPrint(t *testing.T) {
	var testTime = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		desc          string
		entry         logEntry
		isTerminal    bool
		expected      []string
		expectedColor uint
	}{
		{
			desc: "RequestLog in Terminal",
			entry: logEntry{
				Level:   INFO,
				Time:    testTime,
				Message: middleware.RequestLog{Response: 200, ResponseTime: 100, Method: "GET", URI: "/path"},
			},
			isTerminal: true,
			expected: []string{
				"INFO",
				"[00:00:00]",
				"200",
				"GET",
				"/path",
			},
			expectedColor: 6,
		},
		{
			desc: "Non-Terminal Output",
			entry: logEntry{
				Level:   ERROR,
				Time:    testTime,
				Message: "Error message",
			},
			isTerminal: false,
			expected: []string{
				"ERRO",
				"[00:00:00]",
				"Error message",
			},
			expectedColor: 160,
		},
	}

	for _, tc := range tests {
		out := &bytes.Buffer{}
		logger := &logger{isTerminal: tc.isTerminal}

		logger.prettyPrint(tc.entry, out)

		actual := out.String()

		assert.Equal(t, tc.expectedColor, tc.entry.Level.color(), "Unexpected color code")

		for _, part := range tc.expected {
			assert.Contains(t, actual, part, "Expected format part not found")
		}
	}
}

func captureLogOutput(f func()) (stdout, stderr string) {
	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	oldOut := os.Stdout
	oldErr := os.Stderr
	os.Stdout = wOut
	os.Stderr = wErr

	f()

	_ = wOut.Close()
	_ = wErr.Close()

	out, _ := io.ReadAll(rOut)
	err, _ := io.ReadAll(rErr)
	os.Stdout = oldOut
	os.Stderr = oldErr

	return string(out), string(err)
}
