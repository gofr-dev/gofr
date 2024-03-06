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
	"golang.org/x/term"

	"gofr.dev/pkg/gofr/datasource/redis"
	"gofr.dev/pkg/gofr/datasource/sql"
	"gofr.dev/pkg/gofr/grpc"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/service"
	"gofr.dev/pkg/gofr/testutil"
)

func TestLogger_LevelInfo(t *testing.T) {
	printLog := func() {
		logger := NewLogger(INFO)
		logger.Debug("Test Debug Log")
		logger.Info("Test Info Log")
		logger.Error("Test Error Log")
	}

	infoLog := testutil.StdoutOutputForFunc(printLog)
	errLog := testutil.StderrOutputForFunc(printLog)

	assertMessageInJSONLog(t, infoLog, "Test Info Log")
	assertMessageInJSONLog(t, errLog, "Test Error Log")

	if strings.Contains(infoLog, "DEBUG") {
		t.Errorf("TestLogger_LevelInfo Failed. DEBUG log not expected ")
	}
}

func TestLogger_LevelError(t *testing.T) {
	printLog := func() {
		logger := NewLogger(ERROR)
		logger.Logf("%s", "Test Log")
		logger.Debugf("%s", "Test Debug Log")
		logger.Infof("%s", "Test Info Log")
		logger.Errorf("%s", "Test Error Log")
	}

	infoLog := testutil.StdoutOutputForFunc(printLog)
	errLog := testutil.StderrOutputForFunc(printLog)

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

	infoLog := testutil.StdoutOutputForFunc(printLog)
	errLog := testutil.StderrOutputForFunc(printLog)

	if !(strings.Contains(infoLog, "DEBUG") && strings.Contains(infoLog, "INFO")) {
		// Debug Log Level will contain all types of logs i.e. DEBUG, INFO and ERROR
		t.Errorf("TestLogger_LevelDebug Failed!")
	}

	assertMessageInJSONLog(t, errLog, "Test Error Log")
}

func TestLogger_LevelNotice(t *testing.T) {
	printLog := func() {
		logger := NewLogger(NOTICE)
		logger.Log("Test Log")
		logger.Debug("Test Debug Log")
		logger.Info("Test Info Log")
		logger.Notice("Test Notice Log")
		logger.Error("Test Error Log")
	}

	infoLog := testutil.StdoutOutputForFunc(printLog)
	errLog := testutil.StderrOutputForFunc(printLog)

	if strings.Contains(infoLog, "DEBUG") || strings.Contains(infoLog, "INFO") {
		// Notice Log Level will not contain  DEBUG and  INFO logs
		t.Errorf("TestLogger_LevelDebug Failed!")
	}

	assertMessageInJSONLog(t, errLog, "Test Error Log")
}

func TestLogger_LevelWarn(t *testing.T) {
	printLog := func() {
		logger := NewLogger(WARN)
		logger.Debug("Test Debug Log")
		logger.Info("Test Info Log")
		logger.Notice("Test Notice Log")
		logger.Warn("Test Warn Log")
		logger.Error("Test Error Log")
	}

	infoLog := testutil.StdoutOutputForFunc(printLog)
	errLog := testutil.StderrOutputForFunc(printLog)

	if strings.ContainsAny(infoLog, "NOTICE|INFO|DEBUG") && !strings.Contains(errLog, "ERROR") {
		// Warn Log Level will not contain  DEBUG,INFO, NOTICE logs
		t.Errorf("TestLogger_LevelDebug Failed!")
	}

	assertMessageInJSONLog(t, errLog, "Test Error Log")
}

func TestLogger_LevelFatal(t *testing.T) {
	printLog := func() {
		logger := NewLogger(FATAL)
		logger.Debugf("%s", "Test Debug Log")
		logger.Infof("%s", "Test Info Log")
		logger.Noticef("%s", "Test Notice Log")
		logger.Warnf("%s", "Test Warn Log")
		logger.Errorf("%s", "Test Error Log")
	}

	infoLog := testutil.StdoutOutputForFunc(printLog)
	errLog := testutil.StderrOutputForFunc(printLog)

	assert.Equal(t, "", infoLog, "TestLogger_LevelFatal Failed!")
	assert.Equal(t, "", errLog, "TestLogger_LevelFatal Failed")
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

func TestPrettyPrint_DbAndTerminalLogs(t *testing.T) {
	var testTime = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		desc           string
		entry          logEntry
		isTerminal     bool
		expectedOutput []string
	}{
		{
			desc: "RequestLog in Terminal",
			entry: logEntry{
				Level:   INFO,
				Time:    testTime,
				Message: middleware.RequestLog{Response: 200, ResponseTime: 100, Method: "GET", URI: "/path", TraceID: "123"},
			},
			isTerminal: true,
			expectedOutput: []string{
				"INFO",
				"[00:00:00]",
				"123",
				"200",
				"GET",
				"/path",
			},
		},
		{
			desc: "SQL Log",
			entry: logEntry{
				Level:   INFO,
				Time:    testTime,
				Message: sql.Log{Type: "query", Duration: 100, Query: "SELECT * FROM table"},
			},
			isTerminal: true,
			expectedOutput: []string{
				"INFO",
				"[00:00:00]",
				"SQL",
				"100",
				"SELECT * FROM table",
			},
		},
		{
			desc: "Redis Query Log",
			entry: logEntry{
				Level:   INFO,
				Time:    testTime,
				Message: redis.QueryLog{Query: "GET key", Duration: 50},
			},
			isTerminal: true,
			expectedOutput: []string{
				"INFO",
				"[00:00:00]",
				"REDIS",
				"50",
				"GET key",
			},
		},
		{
			desc: "Redis Pipeline Log",
			entry: logEntry{
				Level:   INFO,
				Time:    testTime,
				Message: redis.QueryLog{Query: "pipeline", Duration: 60, Args: []string{"get set"}},
			},
			isTerminal: true,
			expectedOutput: []string{
				"INFO",
				"[00:00:00]",
				"REDIS",
				"60",
				"pipeline",
				"get set",
			},
		},
		{
			desc: "Redis Pipeline Log",
			entry: logEntry{
				Level: INFO, Time: testTime,
				Message: grpc.RPCLog{ID: "b8810022", Method: "/test", StatusCode: 0},
			},
			isTerminal:     true,
			expectedOutput: []string{"INFO", "[00:00:00]", "0", "/test"},
		},
	}

	for _, tc := range tests {
		out := &bytes.Buffer{}
		logger := &logger{isTerminal: tc.isTerminal}

		logger.prettyPrint(tc.entry, out)

		actual := out.String()

		assert.Equal(t, uint(6), tc.entry.Level.color(), "Unexpected color code")

		for _, part := range tc.expectedOutput {
			assert.Contains(t, actual, part, "Expected format part not found")
		}
	}
}

func TestPrettyPrint_ServiceAndDefaultLogs(t *testing.T) {
	var testTime = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		desc           string
		entry          logEntry
		isTerminal     bool
		expectedOutput []string
		expectedColor  uint
	}{
		{
			desc: "Service Log",
			entry: logEntry{
				Level:   INFO,
				Time:    testTime,
				Message: service.Log{CorrelationID: "123", ResponseCode: 200, ResponseTime: 100, HTTPMethod: "GET", URI: "/path"},
			},
			isTerminal: true,
			expectedOutput: []string{
				"INFO",
				"[00:00:00]",
				"123",
				"200",
				"GET",
				"/path",
			},
			expectedColor: 6,
		},
		{
			desc: "Service Error Log",
			entry: logEntry{
				Level: ERROR,
				Time:  testTime,
				Message: service.ErrorLog{Log: service.Log{CorrelationID: "123", ResponseCode: 500, ResponseTime: 100,
					HTTPMethod: "GET", URI: "/path"}, ErrorMessage: "Error message"},
			},
			isTerminal: true,
			expectedOutput: []string{
				"ERRO",
				"[00:00:00]",
				"123",
				"500",
				"GET",
				"/path",
				"Error message",
			},
			expectedColor: 160,
		},
		{
			desc: "Default Case",
			entry: logEntry{
				Level:   INFO,
				Time:    testTime,
				Message: "Default message",
			},
			isTerminal: true,
			expectedOutput: []string{
				"INFO",
				"[00:00:00]",
				"Default message",
			},
			expectedColor: 6,
		},
	}

	for _, tc := range tests {
		out := &bytes.Buffer{}
		logger := &logger{isTerminal: tc.isTerminal}

		logger.prettyPrint(tc.entry, out)

		actual := out.String()

		assert.Equal(t, tc.expectedColor, tc.entry.Level.color(), "Unexpected color code")

		for _, part := range tc.expectedOutput {
			assert.Contains(t, actual, part, "Expected format part not found")
		}
	}
}

func Test_NewSilentLoggerSTDOutput(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		l := NewFileLogger("")

		l.Info("Info Logs")
		l.Debug("Debug Logs")
		l.Notice("Notic Logs")
		l.Warn("Warn Logs")
		l.Infof("%v Logs", "Infof")
		l.Debugf("%v Logs", "Debugf")
		l.Noticef("%v Logs", "Noticef")
		l.Warnf("%v Logs", "warnf")
	})

	assert.Equal(t, "", logs)
}
