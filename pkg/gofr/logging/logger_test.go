package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

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

// newLoggerAt builds a logger at a level. It exists because level is atomic --
// so that ChangeLevel is safe to call from the remote logger's polling
// goroutine -- and so cannot be given in a struct literal.
func newLoggerAt(level Level, normalOut, errorOut io.Writer) *logger {
	l := &logger{normalOut: normalOut, errorOut: errorOut, lock: make(chan struct{}, 1)}
	l.level.Store(int64(level))

	return l
}

// newBufLogger builds a JSON-mode logger writing to out (isTerminal=false).
func newBufLogger(out io.Writer) *logger {
	return newLoggerAt(DEBUG, out, out)
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
	return newLoggerAt(INFO, out, out)
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

// TestTraceMarkerMapFormStillAccepted pins the cross-package contract: pkg/gofr/ai
// emits the trace ID as a map, and that form must keep working alongside the
// cheaper marker type emitted by ContextLogger.
func TestTraceMarkerMapFormStillAccepted(t *testing.T) {
	const want = "0102030405060708090a0b0c0d0e0f10"

	traceID, filtered := extractTraceIDAndFilterArgs(
		[]any{"msg", map[string]any{traceIDMarkerKey: want}})

	require.Equal(t, want, traceID, "the map form must still be recognized")
	require.Equal(t, []any{"msg"}, filtered, "the marker must still be stripped")
}

// TestTraceMarkerTypeFormAccepted covers the cheaper form.
func TestTraceMarkerTypeFormAccepted(t *testing.T) {
	const want = "0102030405060708090a0b0c0d0e0f10"

	traceID, filtered := extractTraceIDAndFilterArgs([]any{"msg", traceIDMarker(want)})

	require.Equal(t, want, traceID)
	require.Equal(t, []any{"msg"}, filtered)
}

// TestLoggerLogEnabledMatchesLevel is the direct test of the gate the request
// middleware consults. Log writes at INFO, so the answer must be true exactly
// when an INFO entry would be emitted -- notably including the DEFAULT level,
// where the gate deliberately does not fire.
func TestLoggerLogEnabledMatchesLevel(t *testing.T) {
	tests := []struct {
		level Level
		want  bool
	}{
		{DEBUG, true},
		{INFO, true},
		{NOTICE, false},
		{WARN, false},
		{ERROR, false},
		{FATAL, false},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			l, ok := NewLogger(tt.level).(*logger)
			require.True(t, ok)

			assert.Equal(t, tt.want, l.LogEnabled())
		})
	}
}

// TestLoggerLogEnabledAgreesWithLog is the anti-drift guard: whatever the gate
// answers must match whether Log actually writes anything.
func TestLoggerLogEnabledAgreesWithLog(t *testing.T) {
	for _, level := range []Level{DEBUG, INFO, NOTICE, WARN, ERROR, FATAL} {
		t.Run(level.String(), func(t *testing.T) {
			buf := &bytes.Buffer{}
			l := newLoggerAt(level, buf, buf)

			l.Log("entry")

			assert.Equal(t, l.LogEnabled(), buf.Len() > 0,
				"LogEnabled must predict whether Log emits")
		})
	}
}

// TestLoggerLogEnabledFollowsChangeLevel pins that the gate tracks a level
// changed at runtime -- the remote logger does exactly this.
func TestLoggerLogEnabledFollowsChangeLevel(t *testing.T) {
	l, ok := NewLogger(INFO).(*logger)
	require.True(t, ok)
	require.True(t, l.LogEnabled())

	l.ChangeLevel(WARN)
	assert.False(t, l.LogEnabled(), "raising the level must close the gate")

	l.ChangeLevel(DEBUG)
	assert.True(t, l.LogEnabled(), "lowering it must reopen it")
}

// Characterization suite: the JSON log-entry envelope on the wire.
//
// These tests pin the enclosing logEntry that wraps every message written by
// this package — the outer object that a log shipper actually parses. The
// payload used throughout is shaped exactly like
// middleware.RequestLog (redeclared locally as logCharRequestLog to avoid an
// import cycle), because the per-request access log is by far the highest
// volume consumer of this envelope.
//
// Nothing here asserts what the code *should* do — only what it does today.
// All added identifiers are prefixed `logChar`.
// ---------------------------------------------------------------------------

// logCharRequestLog mirrors gofr.dev/pkg/gofr/http/middleware.RequestLog field
// for field, tag for tag. Redeclared rather than imported so the logging
// package keeps no dependency on the HTTP middleware.
type logCharRequestLog struct {
	TraceID      string `json:"trace_id,omitempty"`
	SpanID       string `json:"span_id,omitempty"`
	StartTime    string `json:"start_time,omitempty"`
	ResponseTime int64  `json:"response_time,omitempty"`
	Method       string `json:"method,omitempty"`
	UserAgent    string `json:"user_agent,omitempty"`
	IP           string `json:"ip,omitempty"`
	URI          string `json:"uri,omitempty"`
	Response     int    `json:"response,omitempty"`
}

// logCharSampleRequestLog is a fully populated access-log payload.
func logCharSampleRequestLog() *logCharRequestLog {
	return &logCharRequestLog{
		TraceID:      "e1f2d3c4b5a6978877665544332211ff",
		SpanID:       "0011223344556677",
		StartTime:    "2024-03-01T12:34:56.789-05:00",
		ResponseTime: 1234,
		Method:       "GET",
		UserAgent:    "curl/8.4.0",
		IP:           "192.0.2.10",
		URI:          "/api/v1/users?q=1",
		Response:     200,
	}
}

// logCharTimeRe and logCharVersionRe normalize the two non-deterministic parts
// of a log line so the rest can be compared byte for byte.
var (
	logCharTimeRe    = regexp.MustCompile(`"time":"[^"]*"`)
	logCharVersionRe = regexp.MustCompile(`"gofrVersion":"[^"]*"`)
)

// logCharNormalize replaces the wall-clock timestamp and the framework version
// with fixed placeholders.
func logCharNormalize(line string) string {
	line = logCharTimeRe.ReplaceAllString(line, `"time":"<TIME>"`)

	return logCharVersionRe.ReplaceAllString(line, `"gofrVersion":"<VERSION>"`)
}

// Test_LogWireFormat_RequestLogEnvelopeExact pins the ENTIRE log line for an
// access-log entry, byte for byte after timestamp/version normalization:
//   - the envelope keys and their order: level, time, message, gofrVersion;
//   - `trace_id` is ABSENT at the envelope level (it is omitempty and nothing
//     populates it for a request log — the trace ID travels inside `message`);
//   - `level` renders as the level NAME, via Level.MarshalJSON;
//   - the payload is nested as a JSON object under `message`, preserving the
//     RequestLog field order;
//   - exactly one trailing newline, courtesy of json.Encoder.Encode.
func Test_LogWireFormat_RequestLogEnvelopeExact(t *testing.T) {
	const want = `{"level":"INFO","time":"<TIME>","message":{` +
		`"trace_id":"e1f2d3c4b5a6978877665544332211ff","span_id":"0011223344556677",` +
		`"start_time":"2024-03-01T12:34:56.789-05:00","response_time":1234,"method":"GET",` +
		`"user_agent":"curl/8.4.0","ip":"192.0.2.10","uri":"/api/v1/users?q=1","response":200},` +
		`"gofrVersion":"<VERSION>"}` + "\n"

	buf := &bytes.Buffer{}
	newBufLogger(buf).Log(logCharSampleRequestLog())

	//nolint:testifylint // byte equality is the contract; JSONEq ignores key order.
	assert.Equal(t, want, logCharNormalize(buf.String()))
}

// Test_LogWireFormat_ErrorEnvelopeExact pins the same line at ERROR level and
// that it is routed to errorOut, leaving normalOut untouched.
func Test_LogWireFormat_ErrorEnvelopeExact(t *testing.T) {
	const want = `{"level":"ERROR","time":"<TIME>","message":{` +
		`"trace_id":"e1f2d3c4b5a6978877665544332211ff","span_id":"0011223344556677",` +
		`"start_time":"2024-03-01T12:34:56.789-05:00","response_time":1234,"method":"GET",` +
		`"user_agent":"curl/8.4.0","ip":"192.0.2.10","uri":"/api/v1/users?q=1","response":500},` +
		`"gofrVersion":"<VERSION>"}` + "\n"

	normal := &bytes.Buffer{}
	errs := &bytes.Buffer{}
	l := logCharLoggerAt(DEBUG, &logger{normalOut: normal, errorOut: errs})

	payload := logCharSampleRequestLog()
	payload.Response = 500

	l.Error(payload)

	assert.Empty(t, normal.String(), "ERROR must not reach normalOut")
	//nolint:testifylint // byte equality is the contract; JSONEq ignores key order.
	assert.Equal(t, want, logCharNormalize(errs.String()))
}

// Test_LogWireFormat_EnvelopeKeyOrder pins the ordered envelope key list on its
// own, so inserting an envelope field is caught even if values change.
func Test_LogWireFormat_EnvelopeKeyOrder(t *testing.T) {
	tests := []struct {
		name string
		log  func(l *logger)
		want []string
	}{
		{
			"without a trace ID",
			func(l *logger) { l.Log(logCharSampleRequestLog()) },
			[]string{"level", "time", "message", "gofrVersion"},
		},
		{
			"with a trace ID, which slots between message and gofrVersion",
			func(l *logger) {
				l.Log(map[string]any{traceIDMarkerKey: "abc123"}, logCharSampleRequestLog())
			},
			[]string{"level", "time", "message", "trace_id", "gofrVersion"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			tc.log(newBufLogger(buf))

			assert.Equal(t, tc.want, logCharTopLevelKeys(t, buf.Bytes()))
		})
	}
}

// logCharTopLevelKeys returns the top-level object keys of a JSON document in
// wire order.
func logCharTopLevelKeys(t *testing.T, b []byte) []string {
	t.Helper()

	dec := json.NewDecoder(bytes.NewReader(b))

	tok, err := dec.Token()
	require.NoError(t, err)
	require.Equal(t, json.Delim('{'), tok)

	keys := make([]string, 0, 5)

	for dec.More() {
		k, kerr := dec.Token()
		require.NoError(t, kerr)

		key, ok := k.(string)
		require.True(t, ok)

		keys = append(keys, key)

		var discard any

		require.NoError(t, dec.Decode(&discard))
	}

	return keys
}

// Test_LogWireFormat_TraceIDEnvelopeExact pins the full line when a trace-ID
// marker IS present: the marker map is stripped from the message and lifted
// into the envelope's `trace_id`.
func Test_LogWireFormat_TraceIDEnvelopeExact(t *testing.T) {
	const want = `{"level":"INFO","time":"<TIME>","message":"handled",` +
		`"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","gofrVersion":"<VERSION>"}` + "\n"

	buf := &bytes.Buffer{}
	newBufLogger(buf).Log(map[string]any{traceIDMarkerKey: "4bf92f3577b34da6a3ce929d0e0e4736"}, "handled")

	//nolint:testifylint // byte equality is the contract; JSONEq ignores key order.
	assert.Equal(t, want, logCharNormalize(buf.String()))
}

// Test_LogWireFormat_LevelRendering pins that every level serializes as its
// upper-case NAME (not the underlying integer) via Level.MarshalJSON, and that
// the unknown level renders as an empty string.
func Test_LogWireFormat_LevelRendering(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{DEBUG, `"DEBUG"`},
		{INFO, `"INFO"`},
		{NOTICE, `"NOTICE"`},
		{WARN, `"WARN"`},
		{ERROR, `"ERROR"`},
		{FATAL, `"FATAL"`},
		{Level(0), `""`},
		{Level(99), `""`},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			b, err := json.Marshal(tc.level)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(b))
		})
	}
}

// Test_LogWireFormat_MessageShapeByArity pins how `message` is typed depending
// on how many args reach logf. One arg is embedded directly (object, string,
// number...); zero or 2+ args become a JSON array — and zero args produce
// `null`, because the nil []any marshals to null rather than [].
func Test_LogWireFormat_MessageShapeByArity(t *testing.T) {
	tests := []struct {
		name string
		log  func(l *logger)
		want string
	}{
		{"no args yields null", func(l *logger) { l.Log() }, `null`},
		{"one string arg is embedded as a string", func(l *logger) { l.Log("hello") }, `"hello"`},
		{"one int arg is embedded as a number", func(l *logger) { l.Log(42) }, `42`},
		{
			"one struct arg is embedded as an object",
			func(l *logger) { l.Log(&logCharRequestLog{Method: "GET", Response: 200}) },
			`{"method":"GET","response":200}`,
		},
		{"two args become an array", func(l *logger) { l.Log("a", 1) }, `["a",1]`},
		{"three args become an array", func(l *logger) { l.Log("a", 1, true) }, `["a",1,true]`},
		{"a format string collapses to one string", func(l *logger) { l.Logf("a=%d", 7) }, `"a=7"`},
		{"a nil arg is embedded as null", func(l *logger) { l.Log(nil) }, `null`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			tc.log(newBufLogger(buf))

			assert.Equal(t, `{"level":"INFO","time":"<TIME>","message":`+tc.want+`,"gofrVersion":"<VERSION>"}`+"\n",
				logCharNormalize(buf.String()))
		})
	}
}

// Test_LogWireFormat_HTMLEscapingOnTheRealPath pins that the production writer
// (json.NewEncoder(out).Encode) HTML-escapes by default, exactly like
// json.Marshal. A URI carrying `<`, `>` or `&` is rewritten on the wire, and
// invalid UTF-8 becomes the escaped replacement character.
func Test_LogWireFormat_HTMLEscapingOnTheRealPath(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{
			"HTML significant characters",
			"/q?x=<a>&y=1",
			`{"uri":"/q?x=\u003ca\u003e\u0026y=1"}`,
		},
		{
			"quote, backslash and control characters",
			"/a\"b\\c\nd\te",
			`{"uri":"/a\"b\\c\nd\te"}`,
		},
		{
			"line and paragraph separators",
			"/\u2028\u2029",
			`{"uri":"/\u2028\u2029"}`,
		},
		{
			"invalid UTF-8",
			"/\xff/ok",
			`{"uri":"/` + replacementInJSON + `/ok"}`,
		},
		{
			"CJK and emoji stay raw",
			"/日本語/\U0001F680",
			"{\"uri\":\"/日本語/\U0001F680\"}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			newBufLogger(buf).Log(&logCharRequestLog{URI: tc.uri})

			assert.Equal(t, `{"level":"INFO","time":"<TIME>","message":`+tc.want+`,"gofrVersion":"<VERSION>"}`+"\n",
				logCharNormalize(buf.String()))
		})
	}
}

// Test_LogWireFormat_TimeIsRFC3339Nano pins the envelope timestamp format:
// time.Time marshals through its own MarshalJSON, i.e. RFC3339 with nanosecond
// precision and TRAILING ZEROS REMOVED — so a whole-second instant renders
// without a fractional part, and UTC renders as "Z" (unlike the RequestLog
// `start_time` field, which uses a numeric offset). The two timestamps in a
// single access-log line therefore use DIFFERENT formats.
func Test_LogWireFormat_TimeIsRFC3339Nano(t *testing.T) {
	buf := &bytes.Buffer{}
	newBufLogger(buf).Log("x")

	var envelope struct {
		Time string `json:"time"`
	}

	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))

	parsed, err := time.Parse(time.RFC3339Nano, envelope.Time)
	require.NoError(t, err, "envelope time %q must be RFC3339Nano", envelope.Time)
	assert.WithinDuration(t, time.Now(), parsed, time.Minute)

	// The two timestamp formats in play, pinned side by side.
	fixed := time.Date(2024, 3, 1, 12, 34, 56, 0, time.UTC)

	marshaled, err := json.Marshal(fixed)
	require.NoError(t, err)
	assert.Equal(t, `"2024-03-01T12:34:56Z"`, string(marshaled), "envelope time: RFC3339Nano, UTC renders as Z")
	assert.Equal(t, "2024-03-01T12:34:56+00:00",
		fixed.Format("2006-01-02T15:04:05.999999999-07:00"), "RequestLog start_time: numeric offset")
}

// Test_LogWireFormat_OneLinePerEntry pins that entries are newline-delimited
// with no pretty-printing: no indentation, one entry per line, and consecutive
// entries append rather than overwrite.
func Test_LogWireFormat_OneLinePerEntry(t *testing.T) {
	buf := &bytes.Buffer{}
	l := newBufLogger(buf)

	l.Log(logCharSampleRequestLog())
	l.Log(logCharSampleRequestLog())

	out := buf.String()
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")

	require.Len(t, lines, 2)
	assert.Equal(t, 2, strings.Count(out, "\n"), "one newline per entry, none inside")
	assert.Equal(t, logCharNormalize(lines[0]), logCharNormalize(lines[1]),
		"identical payloads produce identical lines once the timestamp is normalized")

	for _, line := range lines {
		assert.NotContains(t, line, "  ", "entries are never indented")
		require.True(t, json.Valid([]byte(line)))
	}
}

// Test_LogWireFormat_BelowThresholdWritesNothing pins that a filtered-out level
// produces ZERO bytes — not an empty JSON object, not a bare newline.
func Test_LogWireFormat_BelowThresholdWritesNothing(t *testing.T) {
	buf := &bytes.Buffer{}
	l := logCharLoggerAt(ERROR, &logger{normalOut: buf, errorOut: buf})

	l.Debug(logCharSampleRequestLog())
	l.Info(logCharSampleRequestLog())
	l.Log(logCharSampleRequestLog())
	l.Notice(logCharSampleRequestLog())
	l.Warn(logCharSampleRequestLog())

	assert.Empty(t, buf.String())

	l.Error(logCharSampleRequestLog())
	assert.NotEmpty(t, buf.String())
}

// Test_LogWireFormat_PayloadOmitemptyPropagates pins that the nested payload's
// omitempty tags apply inside the envelope too: a mostly-zero access log
// collapses to a tiny `message` object, and a fully zero one to `{}`.
func Test_LogWireFormat_PayloadOmitemptyPropagates(t *testing.T) {
	tests := []struct {
		name    string
		payload *logCharRequestLog
		want    string
	}{
		{"fully zero payload collapses to an empty object", &logCharRequestLog{}, `{}`},
		{
			"only a status survives",
			&logCharRequestLog{Response: 200},
			`{"response":200}`,
		},
		{
			"a zero response_time drops the key",
			&logCharRequestLog{Method: "GET", ResponseTime: 0, Response: 204},
			`{"method":"GET","response":204}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			newBufLogger(buf).Log(tc.payload)

			assert.Equal(t, `{"level":"INFO","time":"<TIME>","message":`+tc.want+`,"gofrVersion":"<VERSION>"}`+"\n",
				logCharNormalize(buf.String()))
		})
	}
}

// Test_LogWireFormat_GofrVersionIsPopulated pins that gofrVersion is always
// present and non-empty (it has no omitempty tag, so it would appear even if
// the value were empty).
func Test_LogWireFormat_GofrVersionIsPopulated(t *testing.T) {
	buf := &bytes.Buffer{}
	newBufLogger(buf).Log(logCharSampleRequestLog())

	var envelope struct {
		GofrVersion string `json:"gofrVersion"`
	}

	require.NoError(t, json.Unmarshal(buf.Bytes(), &envelope))
	assert.NotEmpty(t, envelope.GofrVersion)
	assert.Contains(t, buf.String(), `"gofrVersion":"`)
}

// Test_LogWireFormat_PrettyPrintPathIsNotJSON pins the alternative branch: when
// isTerminal is true the entry is rendered as ANSI text and is NOT valid JSON,
// which is why every wire-format assertion above depends on the sink not being
// a terminal (checkIfTerminal returns false for a *bytes.Buffer).
func Test_LogWireFormat_PrettyPrintPathIsNotJSON(t *testing.T) {
	assert.False(t, checkIfTerminal(&bytes.Buffer{}), "a bytes.Buffer is never a terminal, so JSON is emitted")

	buf := &bytes.Buffer{}
	l := logCharLoggerAt(DEBUG, &logger{normalOut: buf, errorOut: buf, isTerminal: true, lock: make(chan struct{}, 1)})

	l.Log("hello")

	assert.False(t, json.Valid(buf.Bytes()), "the terminal branch emits ANSI text, not JSON")
	assert.Contains(t, buf.String(), "hello")
	assert.Contains(t, buf.String(), "INFO")
}

// logCharLoggerAt sets the logger's level and returns it.
//
// The level moved from a plain field to an atomic.Int64 on this branch, to close
// the unsynchronized read that ChangeLevel raced with from the remote logger's
// polling goroutine. A composite literal can no longer set it, so these
// characterization tests set it through the same accessor production uses.
func logCharLoggerAt(level Level, l *logger) *logger {
	l.level.Store(int64(level))

	return l
}
