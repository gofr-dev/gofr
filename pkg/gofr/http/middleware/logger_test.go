package middleware

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/service"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_getIPAddress(t *testing.T) {
	{
		// When RemoteAddr is set
		addr := "0.0.0.0:8080"
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://dummy", http.NoBody)

		require.NoError(t, err, "TEST Failed.\n")

		req.RemoteAddr = addr
		ip := getIPAddress(req)

		assert.Equal(t, addr, ip, "TEST Failed.\n")
	}

	{
		// When `X-Forwarded-For` header is set
		addr := "192.168.0.1:8080"
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://dummy", http.NoBody)

		require.NoError(t, err, "TEST Failed.\n")

		req.Header.Set("X-Forwarded-For", addr)
		ip := getIPAddress(req)

		assert.Equal(t, addr, ip, "TEST Failed.\n")
	}
}

func Test_LoggingMiddleware(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://dummy", http.NoBody)

		rr := httptest.NewRecorder()
		probes := LogProbes{
			Disabled: false,
		}

		handler := Logging(probes, logging.NewMockLogger(logging.DEBUG))(http.HandlerFunc(testHandler))

		handler.ServeHTTP(rr, req)
	})

	assert.Contains(t, logs, "GET    200")
}

func Test_LoggingMiddlewareProbesEnable(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://dummy/.well-known/alive", http.NoBody)

		rr := httptest.NewRecorder()
		probes := LogProbes{
			Disabled: false,
			Paths:    []string{"/.well-known/alive", "/.well-known/health"},
		}

		handler := Logging(probes, logging.NewMockLogger(logging.DEBUG))(http.HandlerFunc(testHandler))

		handler.ServeHTTP(rr, req)
	})

	assert.Contains(t, logs, "GET    200")
}

func Test_LoggingMiddlewareProbesDisable(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://dummy/.well-known/alive", http.NoBody)

		rr := httptest.NewRecorder()
		probes := LogProbes{
			Disabled: true,
			Paths:    []string{"/.well-known/alive", "/.well-known/health"},
		}

		handler := Logging(probes, logging.NewMockLogger(logging.DEBUG))(http.HandlerFunc(testHandler))

		handler.ServeHTTP(rr, req)
	})

	assert.Empty(t, logs, "TEST Failed.\n")
}

func Test_LoggingMiddlewareError(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://dummy", http.NoBody)

		rr := httptest.NewRecorder()
		probes := LogProbes{
			Disabled: false,
		}

		handler := Logging(probes, logging.NewMockLogger(logging.ERROR))(http.HandlerFunc(testHandlerError))

		handler.ServeHTTP(rr, req)
	})

	assert.Contains(t, logs, "GET    500")
}

// Test handler that uses the middleware.
func testHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Test Handler"))
}

// Test handler for internalServerErrors that uses the middleware.
func testHandlerError(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte("error"))
}

func Test_LoggingMiddlewareStringPanicHandling(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://dummy", http.NoBody)

		rr := httptest.NewRecorder()
		probes := LogProbes{
			Disabled: false,
		}

		handler := Logging(probes, logging.NewMockLogger(logging.DEBUG))(http.HandlerFunc(testStringPanicHandler))

		handler.ServeHTTP(rr, req)
	})

	assert.Contains(t, logs, "gofr.dev/pkg/gofr/http/middleware.testStringPanicHandler")
}

// Test handler that uses the middleware.
func testStringPanicHandler(_ http.ResponseWriter, r *http.Request) {
	panic(r.URL.Path)
}

func Test_LoggingMiddlewareErrorPanicHandling(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://dummy", http.NoBody)

		rr := httptest.NewRecorder()
		probes := LogProbes{
			Disabled: false,
		}

		handler := Logging(probes, logging.NewMockLogger(logging.DEBUG))(http.HandlerFunc(testErrorPanicHandler))

		handler.ServeHTTP(rr, req)
	})

	assert.Contains(t, logs, "gofr.dev/pkg/gofr/http/middleware.testErrorPanicHandler")
}

// Test handler that uses the middleware.
func testErrorPanicHandler(http.ResponseWriter, *http.Request) {
	panic(testutil.CustomError{ErrorMessage: "panic"})
}

func Test_LoggingMiddlewareUnknownPanicHandling(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://dummy", http.NoBody)

		rr := httptest.NewRecorder()
		probes := LogProbes{
			Disabled: false,
		}

		handler := Logging(probes, logging.NewMockLogger(logging.DEBUG))(http.HandlerFunc(testUnknownPanicHandler))

		handler.ServeHTTP(rr, req)
	})

	assert.Contains(t, logs, "gofr.dev/pkg/gofr/http/middleware.testUnknownPanicHandler")
}

// Test handler that uses the middleware.
func testUnknownPanicHandler(w http.ResponseWriter, _ *http.Request) {
	panic(w)
}

func TestRequestLog_PrettyPrint(t *testing.T) {
	rl := &RequestLog{
		TraceID:      "7e5c0e9a58839071d4d006dd1d0f4f3a",
		SpanID:       "b19d9aa6323b29bb",
		StartTime:    "2024-04-16T13:34:35.761893+05:30",
		ResponseTime: 1432,
		Method:       "GET",
		UserAgent:    "",
		IP:           "[::1]:59614",
		URI:          "/test",
		Response:     200,
	}
	w := new(bytes.Buffer)
	rl.PrettyPrint(w)

	assert.Equal(t, "\u001B[38;5;8m7e5c0e9a58839071d4d006dd1d0f4f3a \u001B[38;5;34m200   \u001B[0m"+
		"     1432\u001B[38;5;8mµs\u001B[0m GET /test \n", w.String())
}

func Test_ColorForStatusCode(t *testing.T) {
	testCases := []struct {
		desc   string
		code   int
		expOut int
	}{
		{desc: "200 OK", code: 200, expOut: 34},
		{desc: "201 Created", code: 201, expOut: 34},
		{desc: "400 Bad Request", code: 400, expOut: 220},
		{desc: "409 Conflict", code: 409, expOut: 220},
		{desc: "500 Internal Srv Error", code: 500, expOut: 202},
		{desc: "unknown status code", code: 0, expOut: 0},
	}

	for _, tc := range testCases {
		out := colorForStatusCode(tc.code)

		assert.Equal(t, tc.expOut, out)
	}
}

func Test_StatusResponseWriter_WriteHeader(t *testing.T) {
	tests := []struct {
		name           string
		status         int
		expectedStatus int
	}{
		{"WriteHeader 200", 200, 200},
		{"WriteHeader 404", 404, 404},
		{"WriteHeader 500", 500, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			srw := &StatusResponseWriter{ResponseWriter: rr}

			srw.WriteHeader(tt.status)

			require.Equal(t, tt.expectedStatus, srw.status, "status mismatch")
			require.True(t, srw.wroteHeader, "expected wroteHeader to be true")
			require.Equal(t, tt.expectedStatus, rr.Code, "recorder status mismatch")
		})
	}
}

func Test_StatusResponseWriter_WriteHeader_DuplicateCalls(t *testing.T) {
	rr := httptest.NewRecorder()
	srw := &StatusResponseWriter{ResponseWriter: rr}

	srw.WriteHeader(http.StatusOK)
	srw.WriteHeader(http.StatusNotFound) // This should be ignored

	require.Equal(t, http.StatusOK, srw.status, "expected status 200")
	require.Equal(t, http.StatusOK, rr.Code, "expected recorder status 200")
}

// Test_StatusResponseWriter_ImplicitOK asserts that calling Write
// without first calling WriteHeader records status=200 — matching
// net/http's implicit-200 wire behavior. The common idiomatic handler
// pattern is `w.Write([]byte("..."))` without an explicit WriteHeader;
// before the Write() method on StatusResponseWriter was added, logs and
// metrics for those handlers recorded status=0.
func Test_StatusResponseWriter_ImplicitOK(t *testing.T) {
	rr := httptest.NewRecorder()
	srw := &StatusResponseWriter{ResponseWriter: rr}

	_, err := srw.Write([]byte("ok"))
	require.NoError(t, err)

	require.Equal(t, http.StatusOK, srw.status,
		"Write before WriteHeader must record status=200 (matches net/http implicit-200)")
	require.True(t, srw.wroteHeader, "wroteHeader must be set after first Write")

	// A subsequent explicit WriteHeader must be ignored (matches existing
	// duplicate-call guard) so we cannot upgrade an already-implicit 200
	// into something else after bytes have been written.
	srw.WriteHeader(http.StatusInternalServerError)
	require.Equal(t, http.StatusOK, srw.status, "explicit WriteHeader after Write must not overwrite")
}

func Test_StatusResponseWriter_Hijack_Supported(t *testing.T) {
	rr := httptest.NewRecorder()
	srw := &StatusResponseWriter{ResponseWriter: rr}

	// Wrap the recorder in a type that supports Hijack
	hijacker := &hijackableResponseRecorder{rr}
	srw.ResponseWriter = hijacker

	conn, rw, err := srw.Hijack()
	require.NoError(t, err, "expected no error during Hijack")
	require.NotNil(t, conn, "expected conn to be non-nil")
	require.NotNil(t, rw, "expected rw to be non-nil")
}

func Test_StatusResponseWriter_Hijack_NotSupported(t *testing.T) {
	rr := httptest.NewRecorder()
	srw := &StatusResponseWriter{ResponseWriter: rr}

	_, _, err := srw.Hijack()
	require.Error(t, err, "expected an error during Hijack")
	require.ErrorIs(t, err, errHijackNotSupported, "expected error to be errHijackNotSupported")
}

// hijackableResponseRecorder is a custom ResponseRecorder that supports the Hijack method.
type hijackableResponseRecorder struct {
	*httptest.ResponseRecorder
}

func (*hijackableResponseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn := &mockConn{}
	rw := bufio.NewReadWriter(bufio.NewReader(bytes.NewReader(nil)), bufio.NewWriter(bytes.NewBuffer(nil)))

	return conn, rw, nil
}

// mockConn is a mock implementation of net.Conn for testing purposes.
type mockConn struct{}

func (*mockConn) Read([]byte) (n int, err error)   { return 0, nil }
func (*mockConn) Write([]byte) (n int, err error)  { return 0, nil }
func (*mockConn) Close() error                     { return nil }
func (*mockConn) LocalAddr() net.Addr              { return &mockAddr{} }
func (*mockConn) RemoteAddr() net.Addr             { return &mockAddr{} }
func (*mockConn) SetDeadline(time.Time) error      { return nil }
func (*mockConn) SetReadDeadline(time.Time) error  { return nil }
func (*mockConn) SetWriteDeadline(time.Time) error { return nil }

// mockAddr is a mock implementation of net.Addr for testing purposes.
type mockAddr struct{}

func (*mockAddr) Network() string { return "tcp" }
func (*mockAddr) String() string  { return "127.0.0.1:8080" }

// TestRequestLogSchemaSnapshot pins the JSON field set that the request
// log line emits per HTTP request. Log aggregators and dashboards built
// on top of these field names will break if we rename or drop one.
//
// We do not pin field VALUES (timestamps, IDs, durations vary by run) —
// we pin the field NAMES. Both the tracing-on and tracing-off paths
// emit the same field set; trace_id / span_id default to the zero
// strings ("00...0") when no SpanContext is in scope so log
// aggregators always see the keys.
func TestRequestLogSchemaSnapshot(t *testing.T) {
	out := testutil.StdoutOutputForFunc(func() {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://dummy/users/42?x=1", http.NoBody)
		req.Header.Set("User-Agent", "snapshot-test")
		// NewRequestWithContext leaves these fields empty (the real HTTP
		// server sets them on incoming requests). Set them explicitly so
		// all RequestLog fields populate.
		req.RequestURI = "/users/42?x=1"
		req.RemoteAddr = "1.2.3.4:5678"

		rr := httptest.NewRecorder()

		handler := Logging(LogProbes{}, logging.NewLogger(logging.INFO))(http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				// Sleep so response_time is non-zero — the field has
				// json:omitempty and is dropped on zero-duration handlers.
				time.Sleep(time.Microsecond)
				w.WriteHeader(http.StatusOK)
			},
		))

		handler.ServeHTTP(rr, req)
	})

	// One log line per request; locate the JSON object.
	idx := strings.Index(out, "{")
	require.Greater(t, idx, -1, "expected JSON log line, got: %q", out)
	end := strings.LastIndex(out, "}")
	require.Greater(t, end, idx, "malformed JSON log line: %q", out)

	var entry map[string]any

	require.NoError(t, json.Unmarshal([]byte(out[idx:end+1]), &entry))

	// The logger framing fields (level, time, gofrVersion, message).
	for _, k := range []string{"level", "time", "gofrVersion", "message"} {
		assert.Contains(t, entry, k, "log entry missing framing field %q", k)
	}

	// The request-log fields are nested under "message" as an object.
	msg, ok := entry["message"].(map[string]any)
	require.True(t, ok, "message is not an object: %T", entry["message"])

	wantFields := []string{
		"trace_id", "span_id", "start_time", "response_time",
		"method", "user_agent", "ip", "uri", "response",
	}
	for _, k := range wantFields {
		assert.Contains(t, msg, k, "request log missing field %q", k)
	}

	// Spot-check that two of the fields hold the values we set, to
	// guard against accidental rename + same field count.
	assert.Equal(t, "GET", msg["method"])
	assert.Equal(t, "/users/42?x=1", msg["uri"])
	assert.Equal(t, "snapshot-test", msg["user_agent"])
}

// A streaming response reaches the connection's flusher only if StatusResponseWriter forwards
// through Unwrap; without it http.NewResponseController returns ErrNotSupported.
func TestStatusResponseWriter_UnwrapEnablesFlush(t *testing.T) {
	rec := httptest.NewRecorder()
	srw := &StatusResponseWriter{ResponseWriter: rec}

	assert.Equal(t, rec, srw.Unwrap())

	rc := http.NewResponseController(srw)
	require.NoError(t, rc.Flush(), "Unwrap must let ResponseController reach the underlying Flusher")
}

// getIPAddressOld is the pre-optimization implementation, kept here purely as a
// reference oracle so the optimized getIPAddress can be proven byte-for-byte
// equivalent across a range of inputs.
func getIPAddressOld(r *http.Request) string {
	ips := strings.Split(r.Header.Get("X-Forwarded-For"), ",")

	ipAddress := ips[0]
	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	}

	return strings.TrimSpace(ipAddress)
}

func TestGetIPAddress_BackwardCompatible(t *testing.T) {
	cases := []struct {
		name       string
		xff        string
		setXFF     bool
		remoteAddr string
		// divergesFromOld marks inputs where the current implementation
		// intentionally differs from the pre-optimization behavior.
		divergesFromOld bool
	}{
		{"no header falls back to RemoteAddr", "", false, "10.0.0.1:1234", false},
		{"empty header falls back to RemoteAddr", "", true, "10.0.0.1:1234", false},
		{"single ip", "203.0.113.5", true, "10.0.0.1:1234", false},
		{"multiple ips takes first", "203.0.113.5, 70.41.3.18, 150.172.238.178", true, "10.0.0.1:1234", false},
		{"leading spaces trimmed", "  203.0.113.5 , 70.41.3.18", true, "10.0.0.1:1234", false},
		{"leading comma falls back to RemoteAddr", ", 70.41.3.18", true, "10.0.0.1:1234", false},
		{"single ip with trailing comma", "203.0.113.5,", true, "10.0.0.1:1234", false},
		{"only whitespace", "   ", true, "10.0.0.1:1234", true},
		{"ipv6", "2001:db8::1, 70.41.3.18", true, "10.0.0.1:1234", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
			req.RemoteAddr = tc.remoteAddr

			if tc.setXFF {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}

			got := getIPAddress(req)
			want := getIPAddressOld(req)

			if tc.divergesFromOld {
				// Deliberate divergence: the original tested for emptiness
				// BEFORE trimming, so a whitespace-only first entry produced ""
				// instead of falling back to RemoteAddr — and omitempty then
				// dropped the client address from the log line entirely.
				assert.NotEqualf(t, want, got, "expected a deliberate divergence for input %q", tc.xff)
				assert.Equalf(t, strings.TrimSpace(req.RemoteAddr), got,
					"whitespace-only XFF must fall back to RemoteAddr for input %q", tc.xff)

				return
			}

			assert.Equalf(t, want, got, "optimized getIPAddress diverged from the original for input %q", tc.xff)
		})
	}
}

// BenchmarkGetIPAddress measures the per-request X-Forwarded-For parse, which
// the IndexByte change targets (it avoids the []string strings.Split allocates
// on every request just to read the first entry).
func BenchmarkGetIPAddress(b *testing.B) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
	req.Header.Set("X-Forwarded-For", "203.0.113.7, 198.51.100.4, 192.0.2.1")
	req.RemoteAddr = "10.0.0.1:12345"

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = getIPAddress(req)
	}
}

// ---------------------------------------------------------------------------
// Characterization suite: HTTP logging middleware + RequestLog wire format.
//
// Everything below is a *characterization* test: it pins the behavior of the
// code exactly as it is today (bugs included) so that any future refactor —
// including replacing encoding/json with a hand-rolled encoder — has to
// reproduce the current bytes verbatim. Nothing here asserts what the code
// *should* do; deviations found while writing these tests are documented in
// comments rather than fixed.
//
// All identifiers added here are prefixed `logChar` to stay collision-free with
// the rest of the package's tests.
// ---------------------------------------------------------------------------

// logCharStartTimeLayout mirrors the layout string handleRequestLog passes to
// time.Format. Duplicated (not referenced) on purpose: if production changes
// the layout, this test must fail rather than silently follow.
const logCharStartTimeLayout = "2006-01-02T15:04:05.999999999-07:00"

// logCharPanicEnvelope is the exact body panicRecovery writes. The map is a
// map[string]any, so encoding/json sorts the keys alphabetically
// (code, message, status) — NOT declaration order — and json.Encoder.Encode
// appends a trailing newline.
const logCharPanicEnvelope = `{"code":500,"message":"Some unexpected error has occurred","status":"ERROR"}` + "\n"

// errLogCharPanic is the sentinel used by the panic(error) case. Named errFoo
// to satisfy revive's error-naming rule.
var errLogCharPanic = errors.New("boom from an error value")

// logCharRecord is one captured call into the middleware's `logger` interface.
type logCharRecord struct {
	level string // "LOG" or "ERROR"
	arg   any    // the single argument the middleware passes
}

// logCharRecorder implements the package-private `logger` interface and records
// every call, preserving order and which method (Log vs Error) was used.
type logCharRecorder struct {
	mu      sync.Mutex
	records []logCharRecord
}

func (r *logCharRecorder) Log(args ...any) { r.capture("LOG", args) }

func (r *logCharRecorder) Error(args ...any) { r.capture("ERROR", args) }

func (r *logCharRecorder) capture(level string, args []any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var a any
	if len(args) == 1 {
		a = args[0]
	} else {
		a = args
	}

	r.records = append(r.records, logCharRecord{level: level, arg: a})
}

func (r *logCharRecorder) all() []logCharRecord {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([]logCharRecord(nil), r.records...)
}

// requestLog returns the i-th record asserted to be a *RequestLog.
func (r *logCharRecorder) requestLog(t *testing.T, i int) *RequestLog {
	t.Helper()

	recs := r.all()
	require.Greater(t, len(recs), i, "expected at least %d log record(s)", i+1)

	rl, ok := recs[i].arg.(*RequestLog)
	require.True(t, ok, "record %d is %T, want *RequestLog", i, recs[i].arg)

	return rl
}

// logCharFullRequestLog is a RequestLog with every field non-zero, used as the
// baseline for wire-format assertions.
func logCharFullRequestLog() RequestLog {
	return RequestLog{
		TraceID:      "e1f2d3c4b5a6978877665544332211ff",
		SpanID:       "0011223344556677",
		StartTime:    "2024-03-01T12:34:56.789-05:00",
		ResponseTime: 1234,
		Method:       http.MethodGet,
		UserAgent:    "curl/8.4.0",
		IP:           "192.0.2.10",
		URI:          "/api/v1/users?q=1",
		Response:     200,
	}
}

// logCharNewRequest builds a request bound to the test context.
func logCharNewRequest(t *testing.T, method, target string) *http.Request {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), method, target, http.NoBody)
	require.NoError(t, err)

	return req
}

// logCharServe runs one request through a freshly built Logging middleware.
func logCharServe(t *testing.T, probes LogProbes, l logger, h http.HandlerFunc,
	req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	rr := httptest.NewRecorder()
	Logging(probes, l)(h).ServeHTTP(rr, req)

	return rr
}

// logCharStatusHandler returns a handler that writes the given status.
func logCharStatusHandler(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}
}

// ---------------------------------------------------------------------------
// A. RequestLog JSON wire format
// ---------------------------------------------------------------------------

// Test_LoggingContract_RequestLogJSONShape pins the complete serialized form of
// a fully populated RequestLog: every field name, the exact field ORDER
// (encoding/json emits struct fields in declaration order, which is
// deterministic), and the JSON types — response_time and response are JSON
// numbers, never strings.
func Test_LoggingContract_RequestLogJSONShape(t *testing.T) {
	const want = `{"trace_id":"e1f2d3c4b5a6978877665544332211ff","span_id":"0011223344556677",` +
		`"start_time":"2024-03-01T12:34:56.789-05:00","response_time":1234,"method":"GET",` +
		`"user_agent":"curl/8.4.0","ip":"192.0.2.10","uri":"/api/v1/users?q=1","response":200}`

	rl := logCharFullRequestLog()

	byValue, err := json.Marshal(rl)
	require.NoError(t, err)
	assert.JSONEq(t, want, string(byValue))
	//nolint:testifylint // byte equality is the contract; JSONEq ignores key order.
	assert.Equal(t, []byte(want), byValue, "field order must stay in struct declaration order")

	// The middleware always logs a *RequestLog; a pointer must marshal
	// byte-identically to the value.
	byPointer, err := json.Marshal(&rl)
	require.NoError(t, err)
	//nolint:testifylint // byte equality is the contract; JSONEq ignores key order.
	assert.Equal(t, []byte(want), byPointer, "a pointer marshals byte-identically to the value")

	// Types, asserted structurally so a future encoder cannot quote the numbers.
	var generic map[string]any
	require.NoError(t, json.Unmarshal(byValue, &generic))
	assert.IsType(t, float64(0), generic["response_time"], "response_time must be a JSON number")
	assert.IsType(t, float64(0), generic["response"], "response must be a JSON number")
	assert.IsType(t, "", generic["trace_id"])
	assert.Len(t, generic, 9, "RequestLog has exactly 9 wire fields")
}

// Test_LoggingContract_RequestLogFieldOrderKeys pins the ordered key list on its
// own, independent of the values, so a field insertion is caught immediately.
func Test_LoggingContract_RequestLogFieldOrderKeys(t *testing.T) {
	want := []string{
		"trace_id", "span_id", "start_time", "response_time",
		"method", "user_agent", "ip", "uri", "response",
	}

	b, err := json.Marshal(logCharFullRequestLog())
	require.NoError(t, err)

	dec := json.NewDecoder(strings.NewReader(string(b)))

	tok, err := dec.Token()
	require.NoError(t, err)
	require.Equal(t, json.Delim('{'), tok)

	got := make([]string, 0, len(want))

	for dec.More() {
		k, kerr := dec.Token()
		require.NoError(t, kerr)

		got = append(got, k.(string))

		var discard any

		require.NoError(t, dec.Decode(&discard))
	}

	assert.Equal(t, want, got)
}

// Test_LoggingContract_RequestLogOmitempty pins exactly which fields DISAPPEAR
// when they hold their zero value. Every field carries `omitempty`, so a 0
// response_time, a 0 response status, and empty strings all vanish from the
// wire — consumers cannot rely on the keys being present.
func Test_LoggingContract_RequestLogOmitempty(t *testing.T) {
	const base = `{"trace_id":"e1f2d3c4b5a6978877665544332211ff","span_id":"0011223344556677",` +
		`"start_time":"2024-03-01T12:34:56.789-05:00","response_time":1234,"method":"GET",` +
		`"user_agent":"curl/8.4.0","ip":"192.0.2.10","uri":"/api/v1/users?q=1","response":200}`

	tests := []struct {
		name   string
		mutate func(*RequestLog)
		want   string
	}{
		{"all fields set", func(*RequestLog) {}, base},
		{
			"zero response_time drops the key",
			func(rl *RequestLog) { rl.ResponseTime = 0 },
			`{"trace_id":"e1f2d3c4b5a6978877665544332211ff","span_id":"0011223344556677",` +
				`"start_time":"2024-03-01T12:34:56.789-05:00","method":"GET",` +
				`"user_agent":"curl/8.4.0","ip":"192.0.2.10","uri":"/api/v1/users?q=1","response":200}`,
		},
		{
			"zero response drops the key",
			func(rl *RequestLog) { rl.Response = 0 },
			`{"trace_id":"e1f2d3c4b5a6978877665544332211ff","span_id":"0011223344556677",` +
				`"start_time":"2024-03-01T12:34:56.789-05:00","response_time":1234,"method":"GET",` +
				`"user_agent":"curl/8.4.0","ip":"192.0.2.10","uri":"/api/v1/users?q=1"}`,
		},
		{
			"empty user_agent, ip and uri drop their keys",
			func(rl *RequestLog) { rl.UserAgent, rl.IP, rl.URI = "", "", "" },
			`{"trace_id":"e1f2d3c4b5a6978877665544332211ff","span_id":"0011223344556677",` +
				`"start_time":"2024-03-01T12:34:56.789-05:00","response_time":1234,` +
				`"method":"GET","response":200}`,
		},
		{
			"empty method drops the key",
			func(rl *RequestLog) { rl.Method = "" },
			`{"trace_id":"e1f2d3c4b5a6978877665544332211ff","span_id":"0011223344556677",` +
				`"start_time":"2024-03-01T12:34:56.789-05:00","response_time":1234,` +
				`"user_agent":"curl/8.4.0","ip":"192.0.2.10","uri":"/api/v1/users?q=1","response":200}`,
		},
		{
			"empty trace_id and span_id drop their keys",
			func(rl *RequestLog) { rl.TraceID, rl.SpanID = "", "" },
			`{"start_time":"2024-03-01T12:34:56.789-05:00","response_time":1234,"method":"GET",` +
				`"user_agent":"curl/8.4.0","ip":"192.0.2.10","uri":"/api/v1/users?q=1","response":200}`,
		},
		{
			"empty start_time drops the key",
			func(rl *RequestLog) { rl.StartTime = "" },
			`{"trace_id":"e1f2d3c4b5a6978877665544332211ff","span_id":"0011223344556677",` +
				`"response_time":1234,"method":"GET",` +
				`"user_agent":"curl/8.4.0","ip":"192.0.2.10","uri":"/api/v1/users?q=1","response":200}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rl := logCharFullRequestLog()
			tc.mutate(&rl)

			b, err := json.Marshal(&rl)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(b))
		})
	}
}

// Test_LoggingContract_RequestLogZeroValueIsEmptyObject is the extreme
// omitempty case: a zero RequestLog serializes to "{}" — no keys at all.
func Test_LoggingContract_RequestLogZeroValueIsEmptyObject(t *testing.T) {
	b, err := json.Marshal(&RequestLog{})
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(b))
	assert.Equal(t, `{}`, string(b))
}

// Test_LoggingContract_RequestLogEscaping pins encoding/json's exact escaping
// for hostile field values. These byte strings are the contract: a hand-rolled
// encoder must reproduce them character for character.
//
// Notable behaviors pinned here:
//   - `<`, `>` and `&` become \u003c, \u003e, \u0026 (encoding/json HTML-escapes
//     by default, for both Marshal and Encoder).
//   - `'` is NOT escaped.
//   - Control chars use the short forms \n \r \t \b \f where they exist and
//     \u00XX otherwise; DEL (U+007F) is NOT escaped and passes through raw.
//   - U+2028 / U+2029 ARE escaped (JS line separators).
//   - Non-ASCII (CJK, emoji, accented) passes through as raw UTF-8, unescaped.
//   - Invalid UTF-8 bytes are replaced by the escaped form of U+FFFD.
func Test_LoggingContract_RequestLogEscaping(t *testing.T) {
	tests := []struct {
		name string
		in   RequestLog
		want string
	}{
		{
			"double quote and backslash in uri",
			RequestLog{URI: "/a\"b\\c"},
			`{"uri":"/a\"b\\c"}`,
		},
		{
			"control characters use short forms where they exist, \\u00XX otherwise",
			RequestLog{URI: "/a\nb\tc\x00d\x1fe\rf\bg\fx"},
			`{"uri":"/a\nb\tc\u0000d\u001fe\rf\bg\fx"}`,
		},
		{
			"DEL 0x7f is NOT escaped",
			RequestLog{URI: "/\x7f/end"},
			"{\"uri\":\"/\x7f/end\"}",
		},
		{
			"HTML significant characters in uri are escaped",
			RequestLog{URI: "/q?x=<a>&y='z'"},
			`{"uri":"/q?x=\u003ca\u003e\u0026y='z'"}`,
		},
		{
			"HTML significant characters in user_agent are escaped",
			RequestLog{UserAgent: "Mozilla/5.0 <script>alert(\"x\")&</script>"},
			`{"user_agent":"Mozilla/5.0 \u003cscript\u003ealert(\"x\")\u0026\u003c/script\u003e"}`,
		},
		{
			"tab and HTML characters in ip",
			RequestLog{IP: "10.0.0.1\t<b>"},
			`{"ip":"10.0.0.1\t\u003cb\u003e"}`,
		},
		{
			"quotes, backslash and HTML characters in method",
			RequestLog{Method: "GE\"T\\<>&"},
			`{"method":"GE\"T\\\u003c\u003e\u0026"}`,
		},
		{
			"CJK, emoji and combining marks pass through as raw UTF-8",
			RequestLog{URI: "/\u65e5\u672c\u8a9e/\U0001F680/e\u0301"},
			"{\"uri\":\"/\u65e5\u672c\u8a9e/\U0001F680/e\u0301\"}",
		},
		{
			"line and paragraph separators are escaped",
			RequestLog{URI: "/\u2028\u2029"},
			`{"uri":"/\u2028\u2029"}`,
		},
		{
			"invalid UTF-8 bytes become the escaped replacement character",
			RequestLog{URI: "/\xff\xfe/ok"},
			`{"uri":"/\ufffd\ufffd/ok"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(&tc.in)
			require.NoError(t, err, "encoding/json never fails on a RequestLog, even with invalid UTF-8")
			assert.Equal(t, tc.want, string(b))
		})
	}
}

// Test_LoggingContract_MarshalVersusEncoder pins the difference between the two
// encoding entry points. The production path is json.NewEncoder(out).Encode in
// pkg/gofr/logging — so the real log line ends with exactly one "\n" and HTML
// escaping is ON (Encoder defaults to SetEscapeHTML(true), same as Marshal).
func Test_LoggingContract_MarshalVersusEncoder(t *testing.T) {
	const body = `{"uri":"/q?a=\u003cb\u003e\u0026c=1","response":200}`

	rl := &RequestLog{URI: "/q?a=<b>&c=1", Response: 200}

	marshaled, err := json.Marshal(rl)
	require.NoError(t, err)
	//nolint:testifylint // byte equality is the contract; JSONEq ignores key order.
	assert.Equal(t, []byte(body), marshaled, "json.Marshal adds no trailing newline")

	var buf bytes.Buffer
	require.NoError(t, json.NewEncoder(&buf).Encode(rl))
	//nolint:testifylint // byte equality is the contract; JSONEq ignores key order.
	assert.Equal(t, []byte(body+"\n"), buf.Bytes(), "json.Encoder.Encode appends exactly one newline")
	assert.Equal(t, 1, strings.Count(buf.String(), "\n"))
}

// Test_LoggingContract_StartTimeLayout pins the rendered shape of the
// "2006-01-02T15:04:05.999999999-07:00" layout used for start_time. The
// `.999999999` verb TRIMS trailing zeros, so a whole-second instant renders
// with NO fractional part at all — a consumer parsing a fixed-width timestamp
// will break. The `-07:00` verb always renders a numeric offset, so UTC becomes
// "+00:00" rather than "Z".
func Test_LoggingContract_StartTimeLayout(t *testing.T) {
	plus0530 := time.FixedZone("+0530", 5*3600+1800)
	minus0500 := time.FixedZone("-0500", -5*3600)

	tests := []struct {
		name string
		in   time.Time
		want string
	}{
		{"whole second in UTC has no fractional part", time.Date(2024, 3, 1, 12, 34, 56, 0, time.UTC), "2024-03-01T12:34:56+00:00"},
		{"whole second keeps the numeric offset", time.Date(2024, 3, 1, 12, 34, 56, 0, plus0530), "2024-03-01T12:34:56+05:30"},
		{"milliseconds", time.Date(2024, 3, 1, 12, 34, 56, 123000000, time.UTC), "2024-03-01T12:34:56.123+00:00"},
		{"microseconds", time.Date(2024, 3, 1, 12, 34, 56, 123456000, time.UTC), "2024-03-01T12:34:56.123456+00:00"},
		{"nanoseconds", time.Date(2024, 3, 1, 12, 34, 56, 123456789, time.UTC), "2024-03-01T12:34:56.123456789+00:00"},
		{"trailing zeros are trimmed", time.Date(2024, 3, 1, 12, 34, 56, 100000000, time.UTC), "2024-03-01T12:34:56.1+00:00"},
		{"leading zeros are kept", time.Date(2024, 3, 1, 12, 34, 56, 1, time.UTC), "2024-03-01T12:34:56.000000001+00:00"},
		{"negative offset", time.Date(2024, 3, 1, 12, 34, 56, 500000000, minus0500), "2024-03-01T12:34:56.5-05:00"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.in.Format(logCharStartTimeLayout))
		})
	}
}

// Test_LoggingContract_StartTimeIsParseableFromMiddleware confirms the layout
// pinned above is the one the middleware really uses, by round-tripping the
// start_time emitted for a real request.
func Test_LoggingContract_StartTimeIsParseableFromMiddleware(t *testing.T) {
	rec := &logCharRecorder{}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy/x")

	logCharServe(t, LogProbes{}, rec, logCharStatusHandler(http.StatusOK), req)

	rl := rec.requestLog(t, 0)

	parsed, err := time.Parse(logCharStartTimeLayout, rl.StartTime)
	require.NoError(t, err, "start_time %q must parse with the production layout", rl.StartTime)
	assert.WithinDuration(t, time.Now(), parsed, time.Minute)

	// Parsing alone is too weak a pin: time.Parse is lenient about the number
	// of fractional digits, so a production layout of ".000000000" (fixed
	// 9 digits, no trailing-zero trimming) would still parse here. Re-render
	// the parsed instant with the layout pinned above and require byte
	// equality — that fails the moment the production layout verb changes.
	assert.Equal(t, rl.StartTime, parsed.Format(logCharStartTimeLayout),
		"start_time must round-trip byte-for-byte through the pinned layout")
}

// ---------------------------------------------------------------------------
// B. Middleware behavior
// ---------------------------------------------------------------------------

// Test_LoggingContract_StatusRoutesLogVsError pins the severity routing
// threshold at exactly 500: anything below goes to logger.Log, 500 and above to
// logger.Error. 499 vs 500 is the boundary pair.
func Test_LoggingContract_StatusRoutesLogVsError(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   string
	}{
		{"200 is logged at Log level", http.StatusOK, "LOG"},
		{"204 is logged at Log level", http.StatusNoContent, "LOG"},
		{"301 is logged at Log level", http.StatusMovedPermanently, "LOG"},
		{"400 is logged at Log level", http.StatusBadRequest, "LOG"},
		{"404 is logged at Log level", http.StatusNotFound, "LOG"},
		{"499 is logged at Log level", 499, "LOG"},
		{"500 crosses to Error level", http.StatusInternalServerError, "ERROR"},
		{"503 is logged at Error level", http.StatusServiceUnavailable, "ERROR"},
		{"599 is logged at Error level", 599, "ERROR"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := &logCharRecorder{}
			req := logCharNewRequest(t, http.MethodGet, "http://dummy/status")

			rr := logCharServe(t, LogProbes{}, rec, logCharStatusHandler(tc.status), req)

			records := rec.all()
			require.Len(t, records, 1, "exactly one log line per request")
			assert.Equal(t, tc.want, records[0].level)
			assert.Equal(t, tc.status, rec.requestLog(t, 0).Response)
			assert.Equal(t, tc.status, rr.Code)
		})
	}
}

// Test_LoggingContract_HandlerWritesNothingLogs200 pins the implicit-200
// normalization: a handler that never calls WriteHeader or Write still logs
// response 200 (Status() maps the internal 0 to http.StatusOK) — so the
// omitempty on `response` never actually elides the key in practice.
func Test_LoggingContract_HandlerWritesNothingLogs200(t *testing.T) {
	rec := &logCharRecorder{}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy/empty")

	logCharServe(t, LogProbes{}, rec, func(http.ResponseWriter, *http.Request) {}, req)

	records := rec.all()
	require.Len(t, records, 1)
	assert.Equal(t, "LOG", records[0].level)
	assert.Equal(t, http.StatusOK, rec.requestLog(t, 0).Response)

	b, err := json.Marshal(rec.requestLog(t, 0))
	require.NoError(t, err)
	assert.Contains(t, string(b), `"response":200`)
}

// Test_LoggingContract_RequestLogPopulatedFields pins the value shapes the
// middleware fills in for a realistic request.
func Test_LoggingContract_RequestLogPopulatedFields(t *testing.T) {
	rec := &logCharRecorder{}
	req := logCharNewRequest(t, http.MethodPost, "http://dummy/api/v1/users?q=1")
	req.RequestURI = "/api/v1/users?q=1"
	req.Header.Set("User-Agent", "gofr-test/1.0")
	req.RemoteAddr = "198.51.100.7:44321"

	logCharServe(t, LogProbes{}, rec, logCharStatusHandler(http.StatusCreated), req)

	rl := rec.requestLog(t, 0)
	assert.Equal(t, http.MethodPost, rl.Method)
	assert.Equal(t, "gofr-test/1.0", rl.UserAgent)
	assert.Equal(t, "198.51.100.7:44321", rl.IP, "RemoteAddr is used verbatim, port included")
	assert.Equal(t, "/api/v1/users?q=1", rl.URI, "uri comes from r.RequestURI, not r.URL")
	assert.Equal(t, http.StatusCreated, rl.Response)
	assert.Equal(t, zeroTraceID, rl.TraceID)
	assert.Equal(t, zeroSpanID, rl.SpanID)
	assert.GreaterOrEqual(t, rl.ResponseTime, int64(0), "response_time is microseconds, derived from time.Since")
}

// Test_LoggingContract_ResponseTimeIsMicroseconds pins the UNIT of the
// response_time field. The exact number is wall-clock dependent, so instead of
// a value we pin an order-of-magnitude window around a handler that sleeps a
// known duration: 20ms is 20_000µs, which falls inside the window below but
// would land far outside it if the field were ever switched to nanoseconds
// (20_000_000) or milliseconds (20).
func Test_LoggingContract_ResponseTimeIsMicroseconds(t *testing.T) {
	const sleep = 20 * time.Millisecond

	rec := &logCharRecorder{}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy/slow")

	logCharServe(t, LogProbes{}, rec, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(sleep)
		w.WriteHeader(http.StatusOK)
	}), req)

	rl := rec.requestLog(t, 0)

	// Lower bound is half the sleep (coarse timers / CI jitter can under-report
	// slightly); upper bound is 100x the sleep, which a scheduler hiccup can
	// plausibly reach but a unit change cannot fit inside.
	assert.Greater(t, rl.ResponseTime, int64(sleep/time.Microsecond)/2,
		"response_time %d is too small to be microseconds for a %v handler", rl.ResponseTime, sleep)
	assert.Less(t, rl.ResponseTime, int64(sleep/time.Microsecond)*100,
		"response_time %d is too large to be microseconds for a %v handler", rl.ResponseTime, sleep)
}

// Test_LoggingContract_PanicRecoveryShapes pins panic handling for all three
// branches of panicRecovery's type switch, the 500 status, and the EXACT bytes
// of the JSON envelope written to the client.
func Test_LoggingContract_PanicRecoveryShapes(t *testing.T) {
	tests := []struct {
		name      string
		panicWith any
		wantError string
	}{
		{"panic with a string uses the string verbatim", "boom from a string", "boom from a string"},
		{"panic with an error uses Error()", errLogCharPanic, "boom from an error value"},
		{"panic with any other type is labeled", struct{ A int }{1}, "Unknown panic type"},
		{"panic with an int is labeled", 42, "Unknown panic type"},
		{"panic with nil-ish non-error value is labeled", []string{"x"}, "Unknown panic type"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := &logCharRecorder{}
			req := logCharNewRequest(t, http.MethodGet, "http://dummy/panic")

			rr := logCharServe(t, LogProbes{}, rec, func(http.ResponseWriter, *http.Request) {
				panic(tc.panicWith)
			}, req)

			logCharAssertPanicRecord(t, rec, tc.wantError)

			assert.Equal(t, http.StatusInternalServerError, rr.Code)
			//nolint:testifylint // byte equality is the contract; JSONEq ignores key order.
			assert.Equal(t, logCharPanicEnvelope, rr.Body.String(),
				"panic envelope keys are map-sorted (code, message, status) with a trailing newline")
		})
	}
}

// logCharAssertPanicRecord pins the panicLog record: it is always the LAST
// record, always at Error level, and always a panicLog value (not a pointer).
func logCharAssertPanicRecord(t *testing.T, rec *logCharRecorder, wantErr string) {
	t.Helper()

	records := rec.all()
	require.NotEmpty(t, records)

	// The panic record is emitted BEFORE the access log (recovery runs first),
	// so locate it by type rather than by position.
	var (
		pl    panicLog
		ok    bool
		level string
	)

	for _, rec := range records {
		if p, isPanic := rec.arg.(panicLog); isPanic {
			pl, ok, level = p, true, rec.level
			break
		}
	}

	require.True(t, ok, "no panicLog record was emitted; got %d records", len(records))
	assert.Equal(t, "ERROR", level)
	assert.Equal(t, wantErr, pl.Error)
	assert.NotEmpty(t, pl.StackTrace)

	b, err := json.Marshal(pl)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(b), `{"error":"`), "panicLog field order is error then stack_trace")
	assert.Contains(t, string(b), `","stack_trace":"`)
}

// Test_LoggingContract_PanicLogJSONShape pins the panicLog wire format,
// including its omitempty behavior.
func Test_LoggingContract_PanicLogJSONShape(t *testing.T) {
	b, err := json.Marshal(panicLog{Error: "boom", StackTrace: "goroutine 1 [running]:"})
	require.NoError(t, err)
	//nolint:testifylint // byte equality is the contract; JSONEq ignores key order.
	assert.Equal(t, `{"error":"boom","stack_trace":"goroutine 1 [running]:"}`, string(b))

	empty, err := json.Marshal(panicLog{})
	require.NoError(t, err)
	assert.Equal(t, `{}`, string(empty), "both panicLog fields are omitempty")

	only, err := json.Marshal(panicLog{Error: "boom"})
	require.NoError(t, err)
	//nolint:testifylint // byte equality is the contract; JSONEq ignores key order.
	assert.Equal(t, `{"error":"boom"}`, string(only))
}

// Test_LoggingContract_PanicAlsoEmitsRequestLogFirst pins a subtle and
// surprising ordering consequence of the defer registration order in Logging:
//
//	defer pool.Put            (registered 1st -> runs 3rd)
//	defer panicRecovery       (registered 2nd -> runs 2nd)
//	defer handleRequestLog    (registered 3rd -> runs 1st)
//
// So on a panic the REQUEST log is emitted BEFORE the panic log, and because
// panicRecovery has not yet written the 500 at that point, the request log
// records response 200 for a request the client sees as a 500.
func Test_LoggingContract_PanicLogPrecedesRequestLog(t *testing.T) {
	rec := &logCharRecorder{}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy/panic")

	rr := logCharServe(t, LogProbes{}, rec, func(http.ResponseWriter, *http.Request) {
		panic("kaboom")
	}, req)

	records := rec.all()
	require.Len(t, records, 2, "a panic emits both a panic log and a request log")

	_, ok := records[0].arg.(panicLog)
	require.True(t, ok, "the panic log comes FIRST, so recovery has written the 500")
	assert.Equal(t, "ERROR", records[0].level)

	rl, ok := records[1].arg.(*RequestLog)
	require.True(t, ok, "the request log comes SECOND")
	assert.Equal(t, http.StatusInternalServerError, rl.Response,
		"the access log must report the 500 the client actually received")
	assert.Equal(t, "ERROR", records[1].level,
		"a 5xx access log is routed to Error, not Log")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// Test_LoggingContract_PanicAfterWriteKeepsFirstStatus pins that WriteHeader is
// idempotent: when the handler has already committed a status, panicRecovery's
// WriteHeader(500) is a no-op and the envelope is APPENDED to whatever the
// handler already wrote.
func Test_LoggingContract_PanicAfterWriteKeepsFirstStatus(t *testing.T) {
	rec := &logCharRecorder{}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy/panic-after-write")

	rr := logCharServe(t, LogProbes{}, rec, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("partial"))

		panic("late boom")
	}, req)

	assert.Equal(t, http.StatusTeapot, rr.Code, "the first WriteHeader wins; no superfluous-header panic")
	assert.Equal(t, "partial"+logCharPanicEnvelope, rr.Body.String())
	// Records are [panicLog, RequestLog]: recovery runs before the access log.
	assert.Equal(t, http.StatusTeapot, rec.requestLog(t, 1).Response,
		"the status already written by the handler is preserved in the access log")
}

// Test_LoggingContract_PanicOnProbePathSkipsRequestLog pins that the probe-skip
// early return happens AFTER the panicRecovery defer is registered, so panics
// on probe paths are still recovered and logged — but no request log is emitted.
func Test_LoggingContract_PanicOnProbePathSkipsRequestLog(t *testing.T) {
	rec := &logCharRecorder{}
	probes := LogProbes{Disabled: true, Paths: []string{service.HealthPath, service.AlivePath}}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy"+service.HealthPath)

	rr := logCharServe(t, probes, rec, func(http.ResponseWriter, *http.Request) {
		panic("probe boom")
	}, req)

	records := rec.all()
	require.Len(t, records, 1, "only the panic log; no request log on a skipped path")

	pl, ok := records[0].arg.(panicLog)
	require.True(t, ok)
	assert.Equal(t, "probe boom", pl.Error)
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	//nolint:testifylint // byte equality is the contract; JSONEq ignores key order.
	assert.Equal(t, logCharPanicEnvelope, rr.Body.String())
}

// Test_LoggingContract_ProbeFiltering pins which (Disabled, Paths, urlPath)
// combinations suppress the request log entirely.
func Test_LoggingContract_ProbeFiltering(t *testing.T) {
	defaults := []string{service.HealthPath, service.AlivePath}

	tests := []struct {
		name     string
		probes   LogProbes
		path     string
		wantLogs int
	}{
		{"disabled + health path is silent", LogProbes{Disabled: true, Paths: defaults}, service.HealthPath, 0},
		{"disabled + alive path is silent", LogProbes{Disabled: true, Paths: defaults}, service.AlivePath, 0},
		{"disabled + unrelated path still logs", LogProbes{Disabled: true, Paths: defaults}, "/api/users", 1},
		{"disabled + prefix of a probe path still logs", LogProbes{Disabled: true, Paths: defaults}, "/.well-known", 1},
		{"disabled + probe path with trailing slash still logs",
			LogProbes{Disabled: true, Paths: defaults}, service.HealthPath + "/", 1},
		{"not disabled + health path still logs", LogProbes{Disabled: false, Paths: defaults}, service.HealthPath, 1},
		{"not disabled + alive path still logs", LogProbes{Disabled: false, Paths: defaults}, service.AlivePath, 1},
		{"disabled with empty path list logs everything", LogProbes{Disabled: true}, service.HealthPath, 1},
		{"disabled with a custom path is silent", LogProbes{Disabled: true, Paths: []string{"/ping"}}, "/ping", 0},
		{"zero-value LogProbes logs everything", LogProbes{}, service.HealthPath, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := &logCharRecorder{}
			req := logCharNewRequest(t, http.MethodGet, "http://dummy"+tc.path)

			logCharServe(t, tc.probes, rec, logCharStatusHandler(http.StatusOK), req)

			assert.Len(t, rec.all(), tc.wantLogs)
		})
	}
}

// Test_LoggingContract_ProbePathPassesRawWriterToHandler pins a real asymmetry
// in Logging: on the skipped (probe) path it calls inner.ServeHTTP(w, r) with
// the RAW ResponseWriter, while on the normal path it calls
// inner.ServeHTTP(srw, r) with the pooled *StatusResponseWriter.
//
// Consequence: on probe paths the downstream chain (and the handler) do NOT see
// a *StatusResponseWriter. Any downstream middleware that type-asserts on it —
// pkg/gofr/http/middleware/tracer.go does exactly that — silently falls back to
// its own wrapper. Reported as a latent inconsistency, pinned here as-is.
func Test_LoggingContract_ProbePathPassesRawWriterToHandler(t *testing.T) {
	probes := LogProbes{Disabled: true, Paths: []string{service.HealthPath, service.AlivePath}}

	var (
		skippedIsWrapped bool
		normalIsWrapped  bool
	)

	handler := func(target *bool) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			_, ok := w.(*StatusResponseWriter)
			*target = ok
		}
	}

	logCharServe(t, probes, &logCharRecorder{}, handler(&skippedIsWrapped),
		logCharNewRequest(t, http.MethodGet, "http://dummy"+service.HealthPath))

	logCharServe(t, probes, &logCharRecorder{}, handler(&normalIsWrapped),
		logCharNewRequest(t, http.MethodGet, "http://dummy/api/users"))

	assert.True(t, skippedIsWrapped,
		"probe path hands the handler the same wrapped writer as any other path")
	assert.True(t, normalIsWrapped, "normal path hands the handler the pooled *StatusResponseWriter")
}

// Test_LoggingContract_ProbePathStillSetsCorrelationID pins that the
// X-Correlation-ID header is set BEFORE the probe short-circuit, so probe
// responses still carry it even though nothing is logged.
func Test_LoggingContract_ProbePathStillSetsCorrelationID(t *testing.T) {
	rec := &logCharRecorder{}
	probes := LogProbes{Disabled: true, Paths: []string{service.HealthPath, service.AlivePath}}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy"+service.AlivePath)

	rr := logCharServe(t, probes, rec, logCharStatusHandler(http.StatusOK), req)

	assert.Empty(t, rec.all())
	assert.Equal(t, zeroTraceID, rr.Header().Get("X-Correlation-ID"))
}

// Test_LoggingContract_IsLogProbeDisabled pins the predicate directly.
func Test_LoggingContract_IsLogProbeDisabled(t *testing.T) {
	paths := []string{service.HealthPath, service.AlivePath}

	tests := []struct {
		name   string
		probes LogProbes
		path   string
		want   bool
	}{
		{"health path with probes disabled", LogProbes{Disabled: true, Paths: paths}, service.HealthPath, true},
		{"alive path with probes disabled", LogProbes{Disabled: true, Paths: paths}, service.AlivePath, true},
		{"health path with probes enabled", LogProbes{Disabled: false, Paths: paths}, service.HealthPath, false},
		{"unknown path with probes disabled", LogProbes{Disabled: true, Paths: paths}, "/other", false},
		{"empty path list", LogProbes{Disabled: true, Paths: nil}, service.HealthPath, false},
		{"empty string path matches an empty entry", LogProbes{Disabled: true, Paths: []string{""}}, "", true},
		{"match is exact, not prefix", LogProbes{Disabled: true, Paths: paths}, service.HealthPath + "?x=1", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isLogProbeDisabled(tc.probes, tc.path))
		})
	}
}

// Test_LoggingContract_CorrelationIDWithoutSpan pins the no-trace default: the
// header and both log ID fields carry the all-zero W3C strings, not "".
func Test_LoggingContract_CorrelationIDWithoutSpan(t *testing.T) {
	rec := &logCharRecorder{}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy/no-span")

	rr := logCharServe(t, LogProbes{}, rec, logCharStatusHandler(http.StatusOK), req)

	assert.Equal(t, "00000000000000000000000000000000", rr.Header().Get("X-Correlation-ID"))
	assert.Len(t, rr.Header().Get("X-Correlation-ID"), 32)

	rl := rec.requestLog(t, 0)
	assert.Equal(t, "00000000000000000000000000000000", rl.TraceID)
	assert.Equal(t, "0000000000000000", rl.SpanID)
	assert.Len(t, rl.SpanID, 16)
}

// Test_LoggingContract_CorrelationIDWithRecordingSpan pins that a real sampled
// span's IDs flow into both the header and the log line. Uses a locally
// constructed TracerProvider so no OTel process globals are touched.
func Test_LoggingContract_CorrelationIDWithRecordingSpan(t *testing.T) {
	tp := sdktrace.NewTracerProvider()

	t.Cleanup(func() { _ = tp.Shutdown(t.Context()) })

	ctx, span := tp.Tracer("logchar").Start(t.Context(), "logchar-op")
	defer span.End()

	require.True(t, span.IsRecording())
	require.True(t, span.SpanContext().IsValid())

	rec := &logCharRecorder{}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy/traced").WithContext(ctx)

	rr := logCharServe(t, LogProbes{}, rec, logCharStatusHandler(http.StatusOK), req)

	wantTrace := span.SpanContext().TraceID().String()
	wantSpan := span.SpanContext().SpanID().String()

	assert.Equal(t, wantTrace, rr.Header().Get("X-Correlation-ID"))
	assert.Equal(t, wantTrace, rec.requestLog(t, 0).TraceID)
	assert.Equal(t, wantSpan, rec.requestLog(t, 0).SpanID)
	assert.NotEqual(t, zeroTraceID, wantTrace)
}

// Test_LoggingContract_CorrelationIDWithNeverSampledSpan pins the default
// no-exporter deployment shape described in pkg/gofr/otel.go: a NeverSample
// provider still mints VALID trace/span IDs, so correlation IDs stay unique
// per request even though the span is not recording.
func Test_LoggingContract_CorrelationIDWithNeverSampledSpan(t *testing.T) {
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.NeverSample()))

	t.Cleanup(func() { _ = tp.Shutdown(t.Context()) })

	ctx, span := tp.Tracer("logchar").Start(t.Context(), "logchar-op")
	defer span.End()

	require.False(t, span.IsRecording(), "NeverSample spans do not record")
	require.True(t, span.SpanContext().IsValid(), "but the SpanContext is still valid")

	rec := &logCharRecorder{}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy/never-sampled").WithContext(ctx)

	rr := logCharServe(t, LogProbes{}, rec, logCharStatusHandler(http.StatusOK), req)

	wantTrace := span.SpanContext().TraceID().String()

	assert.Equal(t, wantTrace, rr.Header().Get("X-Correlation-ID"))
	assert.NotEqual(t, zeroTraceID, wantTrace)
	assert.Equal(t, wantTrace, rec.requestLog(t, 0).TraceID)
	assert.Equal(t, span.SpanContext().SpanID().String(), rec.requestLog(t, 0).SpanID)
}

// Test_LoggingContract_CorrelationIDWithRemoteSpanContext pins that a
// SpanContext injected directly (as the W3C propagator would after extracting
// traceparent) is honored without any TracerProvider involved.
func Test_LoggingContract_CorrelationIDWithRemoteSpanContext(t *testing.T) {
	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	require.NoError(t, err)

	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7") // spellchecker:disable-line
	require.NoError(t, err)

	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID, SpanID: spanID, TraceFlags: trace.FlagsSampled, Remote: true,
	})
	ctx := trace.ContextWithSpanContext(t.Context(), sc)

	rec := &logCharRecorder{}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy/remote").WithContext(ctx)

	rr := logCharServe(t, LogProbes{}, rec, logCharStatusHandler(http.StatusOK), req)

	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", rr.Header().Get("X-Correlation-ID"))
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", rec.requestLog(t, 0).TraceID)
	assert.Equal(t, "00f067aa0ba902b7", rec.requestLog(t, 0).SpanID) // spellchecker:disable-line
}

// Test_LoggingContract_GetIPAddress pins X-Forwarded-For parsing, including two
// sharp edges:
//   - RemoteAddr is used VERBATIM, port included — it is never split.
//   - A whitespace-only XFF header yields the EMPTY string rather than falling
//     back to RemoteAddr, because the empty check runs BEFORE TrimSpace. The
//     resulting empty `ip` is then dropped from the log line by omitempty.
func Test_LoggingContract_GetIPAddress(t *testing.T) {
	const remote = "10.0.0.9:54321"

	tests := []struct {
		name string
		xff  string
		want string
	}{
		{"no XFF falls back to RemoteAddr with its port", "", remote},
		{"single XFF entry", "203.0.113.5", "203.0.113.5"},
		{"comma list takes the first entry", "203.0.113.5, 70.41.3.18, 150.172.238.178", "203.0.113.5"},
		{"first entry is trimmed", "  203.0.113.5  , 70.41.3.18", "203.0.113.5"},
		{"surrounding whitespace on a single entry is trimmed", "\t203.0.113.5 ", "203.0.113.5"},
		{"XFF with a port keeps the port", "192.168.0.1:8080", "192.168.0.1:8080"},
		{"lone comma falls back to RemoteAddr", ",", remote},
		{"leading comma falls back to RemoteAddr", ",203.0.113.5", remote},
		{"whitespace-only XFF falls back to RemoteAddr", " ", remote},
		{"whitespace before a comma falls back to RemoteAddr", " , 203.0.113.5", remote},
		{"IPv6 entry", "2001:db8::1, 203.0.113.5", "2001:db8::1"},
		{"trailing comma keeps the first entry", "203.0.113.5,", "203.0.113.5"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := logCharNewRequest(t, http.MethodGet, "http://dummy/ip")
			req.RemoteAddr = remote

			if tc.xff != "" {
				req.Header.Set("X-Forwarded-For", tc.xff)
			}

			assert.Equal(t, tc.want, getIPAddress(req))
		})
	}
}

// Test_LoggingContract_EmptyIPIsOmittedFromWire pins the omitempty consequence
// of the whitespace-only XFF edge above: the `ip` key disappears entirely.
func Test_LoggingContract_EmptyIPIsOmittedFromWire(t *testing.T) {
	rec := &logCharRecorder{}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy/ip")
	req.RequestURI = "/ip"
	req.RemoteAddr = ""
	req.Header.Set("X-Forwarded-For", " ")

	logCharServe(t, LogProbes{}, rec, logCharStatusHandler(http.StatusOK), req)

	// A whitespace-only XFF now falls back to RemoteAddr, so the only way to
	// reach an empty ip is for RemoteAddr to be empty too. omitempty then drops
	// the field from the wire.
	rl := rec.requestLog(t, 0)
	assert.Empty(t, rl.IP)

	b, err := json.Marshal(rl)
	require.NoError(t, err)
	assert.NotContains(t, string(b), `"ip"`, "an empty ip is dropped from the wire by omitempty")
}

// Test_LoggingContract_EmptyUserAgentIsOmittedFromWire pins the same for
// user_agent, which is trivially reachable (most non-browser clients send none).
func Test_LoggingContract_EmptyUserAgentIsOmittedFromWire(t *testing.T) {
	rec := &logCharRecorder{}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy/ua")
	req.RequestURI = "/ua"

	logCharServe(t, LogProbes{}, rec, logCharStatusHandler(http.StatusOK), req)

	rl := rec.requestLog(t, 0)
	assert.Empty(t, rl.UserAgent)

	b, err := json.Marshal(rl)
	require.NoError(t, err)
	assert.NotContains(t, string(b), `"user_agent"`)
}

// Test_LoggingContract_PoolResetsStateAcrossRequests pins that the pooled
// StatusResponseWriter is fully reset per request: a 500 followed by a
// write-nothing request through the SAME middleware instance must log 500 then
// 200, never a leaked 500.
func Test_LoggingContract_PoolResetsStateAcrossRequests(t *testing.T) {
	rec := &logCharRecorder{}
	mw := Logging(LogProbes{}, rec)

	first := mw(logCharStatusHandler(http.StatusInternalServerError))
	first.ServeHTTP(httptest.NewRecorder(), logCharNewRequest(t, http.MethodGet, "http://dummy/one"))

	second := mw(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rr := httptest.NewRecorder()
	second.ServeHTTP(rr, logCharNewRequest(t, http.MethodGet, "http://dummy/two"))

	records := rec.all()
	require.Len(t, records, 2)
	assert.Equal(t, "ERROR", records[0].level)
	assert.Equal(t, http.StatusInternalServerError, rec.requestLog(t, 0).Response)
	assert.Equal(t, "LOG", records[1].level)
	assert.Equal(t, http.StatusOK, rec.requestLog(t, 1).Response, "wroteHeader/status must be reset on pool Get")
	assert.Equal(t, http.StatusOK, rr.Code)
}

// Test_LoggingContract_PoolNilsResponseWriterAfterRequest pins that the pooled
// wrapper's ResponseWriter pointer is cleared once the request completes, so a
// stale writer cannot leak across requests through the pool.
func Test_LoggingContract_PoolNilsResponseWriterAfterRequest(t *testing.T) {
	var captured *StatusResponseWriter

	rec := &logCharRecorder{}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy/pool")

	logCharServe(t, LogProbes{}, rec, func(w http.ResponseWriter, _ *http.Request) {
		captured, _ = w.(*StatusResponseWriter)

		w.WriteHeader(http.StatusOK)
	}, req)

	require.NotNil(t, captured)
	assert.Nil(t, captured.ResponseWriter, "ResponseWriter is nil'd before the wrapper returns to the pool")
	assert.Equal(t, http.StatusOK, captured.Status(), "status/wroteHeader are NOT cleared on Put, only on the next Get")
}

// Test_LoggingContract_DoubleWrapPropagatesStatus pins the production chain
// shape: Tracer wraps the writer, then Logging wraps it AGAIN from its pool.
// The status a handler writes must surface through both layers.
func Test_LoggingContract_DoubleWrapPropagatesStatus(t *testing.T) {
	rec := &logCharRecorder{}
	rr := httptest.NewRecorder()
	outer := &StatusResponseWriter{ResponseWriter: rr}

	Logging(LogProbes{}, rec)(logCharStatusHandler(http.StatusServiceUnavailable)).
		ServeHTTP(outer, logCharNewRequest(t, http.MethodGet, "http://dummy/double"))

	assert.Equal(t, http.StatusServiceUnavailable, outer.Status(), "outer wrapper sees the status")
	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, "the real writer sees the status")
	assert.Equal(t, http.StatusServiceUnavailable, rec.requestLog(t, 0).Response)
	assert.Equal(t, zeroTraceID, rr.Header().Get("X-Correlation-ID"), "header set through both wrappers")
}

// ---------------------------------------------------------------------------
// C. StatusResponseWriter
// ---------------------------------------------------------------------------

// Test_LoggingContract_SRWDuplicateWriteHeader pins that the second (and any
// later) WriteHeader call is dropped without a "superfluous response.WriteHeader
// call" from net/http.
func Test_LoggingContract_SRWDuplicateWriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	srw := &StatusResponseWriter{ResponseWriter: rr}

	srw.WriteHeader(http.StatusTeapot)
	srw.WriteHeader(http.StatusOK)
	srw.WriteHeader(http.StatusBadGateway)

	assert.Equal(t, http.StatusTeapot, srw.Status())
	assert.Equal(t, http.StatusTeapot, rr.Code)
	assert.True(t, srw.wroteHeader)
}

// Test_LoggingContract_SRWWriteThenWriteHeader pins that a bare Write commits an
// implicit 200 and a subsequent WriteHeader cannot change it.
func Test_LoggingContract_SRWWriteThenWriteHeader(t *testing.T) {
	rr := httptest.NewRecorder()
	srw := &StatusResponseWriter{ResponseWriter: rr}

	n, err := srw.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, http.StatusOK, srw.Status())

	srw.WriteHeader(http.StatusInternalServerError)

	assert.Equal(t, http.StatusOK, srw.Status(), "the implicit 200 from Write wins")
	assert.Equal(t, "hello", rr.Body.String())
}

// Test_LoggingContract_SRWStatusDefaultsTo200 pins Status()'s zero-normalization
// on an untouched writer.
func Test_LoggingContract_SRWStatusDefaultsTo200(t *testing.T) {
	srw := &StatusResponseWriter{ResponseWriter: httptest.NewRecorder()}

	assert.Equal(t, 0, srw.status, "the raw field is still zero")
	assert.Equal(t, http.StatusOK, srw.Status(), "Status() normalizes zero to 200")
	assert.False(t, srw.wroteHeader)
}

// Test_LoggingContract_SRWUnwrap pins that Unwrap returns the exact wrapped
// writer and that http.NewResponseController reaches Flush through it.
func Test_LoggingContract_SRWUnwrap(t *testing.T) {
	rr := httptest.NewRecorder()
	srw := &StatusResponseWriter{ResponseWriter: rr}

	assert.Same(t, rr, srw.Unwrap())

	_, err := srw.Write([]byte("chunk"))
	require.NoError(t, err)

	require.NoError(t, http.NewResponseController(srw).Flush())
	assert.True(t, rr.Flushed, "Flush reached the recorder via Unwrap")
	assert.Equal(t, "chunk", rr.Body.String())
}

// Test_LoggingContract_SRWResponseControllerUnsupported pins that capabilities
// the wrapped writer does not have surface as http.ErrNotSupported rather than
// being masked by the wrapper.
func Test_LoggingContract_SRWResponseControllerUnsupported(t *testing.T) {
	srw := &StatusResponseWriter{ResponseWriter: httptest.NewRecorder()}

	err := http.NewResponseController(srw).SetWriteDeadline(time.Now().Add(time.Second))
	require.Error(t, err)
	assert.ErrorIs(t, err, http.ErrNotSupported)
}

// Test_LoggingContract_SRWHijackNotSupported pins the exact error produced when
// the wrapped writer is not an http.Hijacker, including the sentinel wrapping.
func Test_LoggingContract_SRWHijackNotSupported(t *testing.T) {
	srw := &StatusResponseWriter{ResponseWriter: httptest.NewRecorder()}

	conn, rw, err := srw.Hijack()

	require.Error(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, rw)
	require.ErrorIs(t, err, errHijackNotSupported)
	assert.Equal(t, "response writer does not support hijacking: cannot hijack connection", err.Error())
	assert.Equal(t, "response writer does not support hijacking", errHijackNotSupported.Error())
}

// ---------------------------------------------------------------------------
// D. PrettyPrint (terminal path)
// ---------------------------------------------------------------------------

// Test_LoggingContract_PrettyPrintExactBytes pins the exact ANSI byte sequence
// PrettyPrint emits, including the %-6d status padding, the %8d duration
// padding, and the trailing " \n" (space then newline).
func Test_LoggingContract_PrettyPrintExactBytes(t *testing.T) {
	const (
		grey  = "\u001B[38;5;8m"
		reset = "\u001B[0m"
	)

	tests := []struct {
		name   string
		status int
		want   string
	}{
		{"2xx uses color 34", 200, grey + "abc \u001B[38;5;34m200   " + reset + "       42" + grey + "µs" + reset + " GET /x \n"},
		{"4xx uses color 220", 404, grey + "abc \u001B[38;5;220m404   " + reset + "       42" + grey + "µs" + reset + " GET /x \n"},
		{"5xx uses color 202", 500, grey + "abc \u001B[38;5;202m500   " + reset + "       42" + grey + "µs" + reset + " GET /x \n"},
		{"1xx falls through to color 0", 100, grey + "abc \u001B[38;5;0m100   " + reset + "       42" + grey + "µs" + reset + " GET /x \n"},
		{"3xx falls through to color 0", 302, grey + "abc \u001B[38;5;0m302   " + reset + "       42" + grey + "µs" + reset + " GET /x \n"},
		{"6xx falls through to color 0", 600, grey + "abc \u001B[38;5;0m600   " + reset + "       42" + grey + "µs" + reset + " GET /x \n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rl := &RequestLog{TraceID: "abc", Response: tc.status, ResponseTime: 42, Method: http.MethodGet, URI: "/x"}

			var buf bytes.Buffer
			rl.PrettyPrint(&buf)

			assert.Equal(t, tc.want, buf.String())
		})
	}
}

// Test_LoggingContract_PrettyPrintPaddingOverflow pins that the %-6d and %8d
// widths are MINIMUMS: oversized values push the line wider rather than being
// truncated, and there is no separator besides the padding.
func Test_LoggingContract_PrettyPrintPaddingOverflow(t *testing.T) {
	rl := &RequestLog{TraceID: "t", Response: 1234567, ResponseTime: 1234567890, Method: "PATCH", URI: "/long/path"}

	var buf bytes.Buffer
	rl.PrettyPrint(&buf)

	want := "\u001B[38;5;8mt \u001B[38;5;0m1234567\u001B[0m 1234567890\u001B[38;5;8mµs\u001B[0m PATCH /long/path \n"
	assert.Equal(t, want, buf.String())
}

// Test_LoggingContract_PrettyPrintEmptyRequestLog pins the zero-value render —
// PrettyPrint applies no omitempty logic, so empty fields become empty columns.
func Test_LoggingContract_PrettyPrintEmptyRequestLog(t *testing.T) {
	var buf bytes.Buffer
	(&RequestLog{}).PrettyPrint(&buf)

	want := "\u001B[38;5;8m \u001B[38;5;0m0     \u001B[0m        0\u001B[38;5;8mµs\u001B[0m   \n"
	assert.Equal(t, want, buf.String())
}

// Test_LoggingContract_ColorForStatusCodeBoundaries pins every boundary of the
// status -> color mapping used by PrettyPrint.
func Test_LoggingContract_ColorForStatusCodeBoundaries(t *testing.T) {
	tests := []struct {
		status int
		want   int
	}{
		{0, 0}, {100, 0}, {199, 0},
		{200, 34}, {299, 34},
		{300, 0}, {399, 0},
		{400, 220}, {499, 220},
		{500, 202}, {599, 202},
		{600, 0}, {-1, 0},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("status_%d", tc.status), func(t *testing.T) {
			assert.Equal(t, tc.want, colorForStatusCode(tc.status))
		})
	}
}

// ---------------------------------------------------------------------------
// E. Misc
// ---------------------------------------------------------------------------

// Test_LoggingContract_HandleRequestLogNilLogger pins that a nil logger is a
// silent no-op in handleRequestLog rather than a nil-pointer panic. (Note:
// panicRecovery has no such guard — it calls logger.Error unconditionally.)
func Test_LoggingContract_HandleRequestLogNilLogger(t *testing.T) {
	srw := &StatusResponseWriter{ResponseWriter: httptest.NewRecorder()}
	req := logCharNewRequest(t, http.MethodGet, "http://dummy/nil-logger")

	assert.NotPanics(t, func() {
		handleRequestLog(srw, req, time.Now(), zeroTraceID, zeroSpanID, nil)
	})
}

// Test_LoggingContract_ZeroIDConstants pins the literal zero-ID constants used
// when no valid SpanContext is in scope.
func Test_LoggingContract_ZeroIDConstants(t *testing.T) {
	assert.Equal(t, "00000000000000000000000000000000", zeroTraceID)
	assert.Equal(t, "0000000000000000", zeroSpanID)
	assert.Equal(t, zeroTraceID, trace.TraceID{}.String(), "matches the W3C invalid TraceID rendering")
	assert.Equal(t, zeroSpanID, trace.SpanID{}.String(), "matches the W3C invalid SpanID rendering")
}

// Test_LoggingContract_MethodIsNotNormalized pins that the log records
// r.Method verbatim — no upper-casing (unlike the Tracer middleware, which does
// strings.ToUpper for the span name).
func Test_LoggingContract_MethodIsNotNormalized(t *testing.T) {
	rec := &logCharRecorder{}

	req := logCharNewRequest(t, http.MethodGet, "http://dummy/case")
	req.Method = "get"

	logCharServe(t, LogProbes{}, rec, logCharStatusHandler(http.StatusOK), req)

	assert.Equal(t, "get", rec.requestLog(t, 0).Method)
}

// Test_LoggingContract_OneLogPerRequest pins that a normal request produces
// exactly one record and that repeated requests do not accumulate duplicates.
func Test_LoggingContract_OneLogPerRequest(t *testing.T) {
	rec := &logCharRecorder{}
	mw := Logging(LogProbes{}, rec)(logCharStatusHandler(http.StatusOK))

	const n = 5

	for i := range n {
		mw.ServeHTTP(httptest.NewRecorder(), logCharNewRequest(t, http.MethodGet, fmt.Sprintf("http://dummy/%d", i)))
	}

	assert.Len(t, rec.all(), n)
}

// Test_LoggingContract_ConcurrentRequestsShareThePool exercises the sync.Pool
// under -race with concurrent requests and pins that each request still gets an
// independent, correctly-scoped status.
func Test_LoggingContract_ConcurrentRequestsShareThePool(t *testing.T) {
	rec := &logCharRecorder{}
	mw := Logging(LogProbes{}, rec)

	const n = 32

	var wg sync.WaitGroup

	for i := range n {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			status := http.StatusOK
			if i%2 == 0 {
				status = http.StatusInternalServerError
			}

			h := mw(logCharStatusHandler(status))
			rr := httptest.NewRecorder()
			req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://dummy/c", http.NoBody)
			h.ServeHTTP(rr, req)

			assert.Equal(t, status, rr.Code)
		}(i)
	}

	wg.Wait()

	records := rec.all()
	require.Len(t, records, n)

	var errs int

	for _, r := range records {
		if r.level == "ERROR" {
			errs++
		}
	}

	assert.Equal(t, n/2, errs, "half the requests were 500s; no status leaked across pooled writers")
}
