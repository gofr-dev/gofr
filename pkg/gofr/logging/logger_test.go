package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/term"

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

	assert.Empty(t, infoLog) // Since log level is ERROR we will not get any INFO logs.
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

	levels := []Level{DEBUG, INFO, NOTICE}

	for i, l := range levels {
		assert.NotContainsf(t, infoLog, l.String(), "TEST[%d], Failed.\nunexpected %s log", i, l)
	}

	assertMessageInJSONLog(t, errLog, "Test Error Log")
}

func TestLogger_LevelFatal(t *testing.T) {
	// running the failing part only when a specific env variable is set
	if os.Getenv("GOFR_EXITER") == "1" {
		logger := NewLogger(FATAL)

		logger.Debugf("%s", "Test Debug Log")
		logger.Infof("%s", "Test Info Log")
		logger.Logf("%s", "Test Log")
		logger.Noticef("%s", "Test Notice Log")
		logger.Warnf("%s", "Test Warn Log")
		logger.Errorf("%s", "Test Error Log")
		logger.Fatalf("%s", "Test Fatal Log")

		return
	}

	//nolint:gosec // starting the actual test in a different subprocess
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=TestLogger_LevelFatal")
	cmd.Env = append(os.Environ(), "GOFR_EXITER=1")

	stderr, err := cmd.StderrPipe()
	require.NoError(t, err)

	require.NoError(t, cmd.Start())

	stderrBytes, err := io.ReadAll(stderr)
	require.NoError(t, err)

	// Use stderr as the log output (the JSON log is written to stderr)
	// Extract only the JSON log line from stderr
	stderrOutput := string(stderrBytes)

	lines := strings.Split(stderrOutput, "\n")

	var log string

	for _, line := range lines {
		if strings.HasPrefix(line, "{") && strings.Contains(line, "level") {
			log = line
			break
		}
	}

	levels := []Level{DEBUG, INFO, NOTICE, WARN, ERROR} // levels which should not be present in case of FATAL log_level

	for i, l := range levels {
		assert.NotContainsf(t, log, l.String(), "TEST[%d], Failed.\nunexpected %s log", i, l)
	}

	assertMessageInJSONLog(t, log, "Test Fatal Log")

	err = cmd.Wait()

	var e *exec.ExitError

	require.ErrorAs(t, err, &e)
	assert.False(t, e.Success())
}

func assertMessageInJSONLog(t *testing.T, logLine, expectation string) {
	t.Helper()

	// Try to unmarshal the entire log line as JSON first
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

	assert.Empty(t, logs)
}

type mockLog struct {
	msg string
}

func (m *mockLog) PrettyPrint(writer io.Writer) {
	fmt.Fprintf(writer, "TEST %s", m.msg)
}

func TestPrettyPrint(t *testing.T) {
	m := &mockLog{msg: "mock test log"}
	out := &bytes.Buffer{}
	l := &logger{isTerminal: true, lock: make(chan struct{}, 1)}

	// case PrettyPrint is implemented
	l.prettyPrint(&logEntry{
		Level:   INFO,
		Message: m,
	}, out)

	outputLog := out.String()
	expOut := []string{"INFO", "[00:00:00]", "TEST mock test log"}

	for _, v := range expOut {
		assert.Contains(t, outputLog, v)
	}

	// case pretty print is not implemented
	out.Reset()

	l.prettyPrint(&logEntry{
		Level:   DEBUG,
		Message: "test log for normal log",
	}, out)

	outputLog = out.String()
	expOut = []string{"DEBU", "[00:00:00]", "test log for normal log"}

	for _, v := range expOut {
		assert.Contains(t, outputLog, v)
	}
}

func TestNewFileLogger_UnwritablePath(t *testing.T) {
	l := NewFileLogger("/root/invalid.log")
	logger, ok := l.(*logger)
	require.True(t, ok)

	assert.Equal(t, io.Discard, logger.normalOut)
	assert.Equal(t, io.Discard, logger.errorOut)
}

func TestNewFileLogger_NilPath(t *testing.T) {
	l := NewFileLogger("")
	logger, ok := l.(*logger)
	require.True(t, ok)

	assert.Equal(t, io.Discard, logger.normalOut)
	assert.Equal(t, io.Discard, logger.errorOut)
}

func TestNewFileLogger_Close(t *testing.T) {
	tempFile, err := os.CreateTemp(t.TempDir(), "gofr_test_log_*.log")
	require.NoError(t, err)

	tempFile.Close() // Close it since NewFileLogger will open it.

	l := NewFileLogger(tempFile.Name())

	closer, ok := l.(io.Closer)
	require.True(t, ok, "logger should implement io.Closer")

	err = closer.Close()
	require.NoError(t, err)

	// verify that subsequent Close calls do not panic
	err = closer.Close()
	assert.ErrorIs(t, err, os.ErrClosed)
}

// newBufLogger builds a JSON-mode logger writing to out (isTerminal=false).
func newBufLogger(out io.Writer) *logger {
	return &logger{level: DEBUG, normalOut: out, errorOut: out}
}

// wireLog decodes a JSON log line for assertions. It mirrors the production
// logEntry but types level as a string, because Level.MarshalJSON emits the
// level name ("INFO") and there is no matching UnmarshalJSON to read it back
// into a Level. Decoding here is exactly what an external log consumer does.
type wireLog struct {
	Level       string `json:"level"`
	Message     any    `json:"message"`
	TraceID     string `json:"trace_id"`
	GofrVersion string `json:"gofrVersion"`
}

// TestLogger_JSONWireFormat_Unchanged asserts the pooled-buffer write path
// produces the same JSON shape as before: a single object per line, trailing
// newline, with level/message/gofrVersion fields intact.
func TestLogger_JSONWireFormat_Unchanged(t *testing.T) {
	buf := &bytes.Buffer{}
	l := newBufLogger(buf)

	l.Info("request handled successfully")

	out := buf.String()

	require.True(t, strings.HasPrefix(out, "{"), "line must be a JSON object")
	require.True(t, strings.HasSuffix(out, "}\n"), "line must end with a single newline")
	require.Equal(t, 1, strings.Count(out, "\n"), "exactly one trailing newline, no double newline")

	var e wireLog
	require.NoError(t, json.Unmarshal([]byte(out), &e))
	assert.Equal(t, "INFO", e.Level)
	assert.Equal(t, "request handled successfully", e.Message)
	assert.NotEmpty(t, e.GofrVersion, "gofrVersion field must still be populated")
}

// TestLogger_Infof_Unchanged verifies the formatted path is unaffected.
func TestLogger_Infof_Unchanged(t *testing.T) {
	buf := &bytes.Buffer{}
	l := newBufLogger(buf)

	l.Infof("handled %s in %d us", "GET /users", 1234)

	var e wireLog
	require.NoError(t, json.Unmarshal(buf.Bytes(), &e))
	assert.Equal(t, "handled GET /users in 1234 us", e.Message)
}

// syncWriter is a concurrency-safe io.Writer, matching production where the
// logger's sink is an *os.File (whose Write is safe for concurrent use).
type syncWriter struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (w *syncWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.Write(p)
}

func (w *syncWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.buf.String()
}

// TestLogger_ConcurrentWrites_NoInterleave stresses concurrent logging: many
// goroutines share one logger writing to one concurrency-safe sink. Every
// emitted line must remain a well-formed, complete JSON object — proof that a
// per-call encoder does not corrupt or interleave output when the sink is safe.
func TestLogger_ConcurrentWrites_NoInterleave(t *testing.T) {
	buf := &syncWriter{}
	l := newBufLogger(buf)

	const goroutines, perG = 50, 200

	var wg sync.WaitGroup

	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()

			for i := 0; i < perG; i++ {
				l.Info("msg")
			}
		}()
	}

	wg.Wait()

	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	require.Len(t, lines, goroutines*perG, "no lines lost or merged")

	for i, line := range lines {
		var e wireLog
		require.NoErrorf(t, json.Unmarshal([]byte(line), &e), "line %d is not valid JSON: %q", i, line)
		assert.Equal(t, "msg", e.Message)
	}
}

// TestExtractTraceID_FastPath_NoMarker confirms the zero-alloc fast path
// returns the args slice untouched when no trace marker is present.
func TestExtractTraceID_FastPath_NoMarker(t *testing.T) {
	args := []any{"a", 1, map[string]any{"k": "v"}}

	tid, filtered := extractTraceIDAndFilterArgs(args)

	assert.Empty(t, tid)
	assert.Equal(t, args, filtered)
}

// TestExtractTraceID_SlowPath_Marker confirms the marker is extracted and
// stripped exactly as before, preserving the __trace_id__ contract.
func TestExtractTraceID_SlowPath_Marker(t *testing.T) {
	args := []any{"a", map[string]any{"__trace_id__": "abc123"}, "b"}

	tid, filtered := extractTraceIDAndFilterArgs(args)

	assert.Equal(t, "abc123", tid)
	assert.Equal(t, []any{"a", "b"}, filtered, "marker map must be filtered out of the message args")
}

// newBenchLogger builds a logger writing to out with the JSON (non-terminal)
// path, matching production behavior where stdout is not a TTY.
func newBenchLogger(out io.Writer) *logger {
	return &logger{
		level:     INFO,
		normalOut: out,
		errorOut:  out,
	}
}

// BenchmarkLoggerInfoJSON measures the single-string JSON log hot path.
func BenchmarkLoggerInfoJSON(b *testing.B) {
	l := newBenchLogger(io.Discard)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		l.Info("request handled successfully")
	}
}

// BenchmarkLoggerInfofJSON measures the formatted JSON log hot path.
func BenchmarkLoggerInfofJSON(b *testing.B) {
	l := newBenchLogger(io.Discard)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		l.Infof("handled %s in %d us", "GET /users", 1234)
	}
}
