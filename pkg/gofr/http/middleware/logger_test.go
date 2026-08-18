package middleware

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/logging/remotelogger"
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
	}{
		{"no header falls back to RemoteAddr", "", false, "10.0.0.1:1234"},
		{"empty header falls back to RemoteAddr", "", true, "10.0.0.1:1234"},
		{"single ip", "203.0.113.5", true, "10.0.0.1:1234"},
		{"multiple ips takes first", "203.0.113.5, 70.41.3.18, 150.172.238.178", true, "10.0.0.1:1234"},
		{"leading spaces trimmed", "  203.0.113.5 , 70.41.3.18", true, "10.0.0.1:1234"},
		{"leading comma falls back to RemoteAddr", ", 70.41.3.18", true, "10.0.0.1:1234"},
		{"single ip with trailing comma", "203.0.113.5,", true, "10.0.0.1:1234"},
		{"only whitespace", "   ", true, "10.0.0.1:1234"},
		{"ipv6", "2001:db8::1, 70.41.3.18", true, "10.0.0.1:1234"},
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

// levelGateLogger records what it was asked and what it received, so the tests
// below can assert both that the gate is consulted and that it is obeyed.
type levelGateLogger struct {
	enabled  bool
	logCalls int
	errCalls int
}

func (l *levelGateLogger) Log(...any)       { l.logCalls++ }
func (l *levelGateLogger) Error(...any)     { l.errCalls++ }
func (l *levelGateLogger) LogEnabled() bool { return l.enabled }

// TestRequestLogEmittedWhenLevelAllows is the feature guard: when the level
// permits an informational entry, the request log is still written.
func TestRequestLogEmittedWhenLevelAllows(t *testing.T) {
	lg := &levelGateLogger{enabled: true}
	srw := &StatusResponseWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}

	handleRequestLog(srw, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody),
		time.Now(), "tid", trace.SpanContext{}, lg)

	require.Equal(t, 1, lg.logCalls, "an allowed level must still emit the request log")
}

// TestRequestLogSkippedWhenLevelDiscards pins the win.
func TestRequestLogSkippedWhenLevelDiscards(t *testing.T) {
	lg := &levelGateLogger{enabled: false}
	srw := &StatusResponseWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}

	handleRequestLog(srw, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody),
		time.Now(), "tid", trace.SpanContext{}, lg)

	require.Zero(t, lg.logCalls, "a discarded level must not be handed an entry")
}

// TestRequestLogAlwaysEmittedForServerErrors is the important boundary: a 5xx
// goes through Error, which the informational gate must never suppress.
func TestRequestLogAlwaysEmittedForServerErrors(t *testing.T) {
	lg := &levelGateLogger{enabled: false}
	srw := &StatusResponseWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusInternalServerError}

	handleRequestLog(srw, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody),
		time.Now(), "tid", trace.SpanContext{}, lg)

	require.Equal(t, 1, lg.errCalls, "a server error must be logged regardless of the informational level")
}

// TestRequestLogUngatedLoggerUnaffected pins backward compatibility: a logger
// that does not implement the optional interface behaves exactly as before.
func TestRequestLogUngatedLoggerUnaffected(t *testing.T) {
	lg := &plainLogger{}
	srw := &StatusResponseWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}

	handleRequestLog(srw, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody),
		time.Now(), "tid", trace.SpanContext{}, lg)

	require.Equal(t, 1, lg.logCalls, "a logger without the gate must still receive the entry")
}

type plainLogger struct{ logCalls int }

func (l *plainLogger) Log(...any) { l.logCalls++ }
func (*plainLogger) Error(...any) {}

// TestCorrelationIDHeaderSpellingUnchanged pins that assigning the canonical key
// directly still produces the header clients read today.
func TestCorrelationIDHeaderSpellingUnchanged(t *testing.T) {
	h := http.Header{}
	h[canonicalCorrelationID] = []string{"abc123"}

	require.Equal(t, "abc123", h.Get("X-Correlation-ID"), "lookup by the documented name must work")
	require.Equal(t, "abc123", h.Get("X-Correlation-Id"), "and by the canonical form")

	// The const is only safe to hard-code while it equals what
	// Header.Set would have canonicalized the documented name to.
	require.Equal(t, canonicalCorrelationID, textproto.CanonicalMIMEHeaderKey("X-Correlation-ID"))
}

// loggingBenchWriter keeps a header map and discards the rest, so the benchmark measures the
// middleware rather than a recorder.
type loggingBenchWriter struct{ h http.Header }

func (w *loggingBenchWriter) Header() http.Header       { return w.h }
func (*loggingBenchWriter) Write(b []byte) (int, error) { return len(b), nil }
func (*loggingBenchWriter) WriteHeader(int)             {}

// BenchmarkLogging measures the middleware at a level that DISCARDS the entry.
//
// The request log is written through Log, which logs at INFO, so the gate only discards from
// NOTICE upward -- a service at the default level still emits it and still pays to build it.
// This benchmark therefore runs at ERROR, the shape of a service that has deliberately turned
// access logging off. Anything spent building an entry that is then thrown away is pure waste,
// which is what the change targets — hence allocations, not wall clock, are the metric here.
func BenchmarkLogging(b *testing.B) {
	// A real *logging.logger, not NewMockLogger: MockLogger does not implement
	// LogEnabled, so the logEnabler assertion would fail and the benchmark would
	// measure the pre-optimization path while appearing to measure the gate.
	logger := logging.NewLogger(logging.ERROR)

	router := mux.NewRouter()
	router.Use(Logging(LogProbes{}, logger))
	router.NewRoute().Methods(http.MethodGet).Path("/users/{id}").
		Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/users/42", http.NoBody)
	req.RemoteAddr = "192.0.2.10:5555"

	w := &loggingBenchWriter{h: make(http.Header, 4)}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		clear(w.h)
		router.ServeHTTP(w, req)
	}
}

// TestRequestLogGateWithRealLoggers pins the gate against the loggers production
// actually wires, not the levelGateLogger double above. container.go builds a
// remotelogger, so that wrapper -- and the plain logger it embeds -- are what
// decide whether a request log is built.
//
// It also pins the reach honestly: at the DEFAULT level the entry is still
// emitted. The saving lands only on a service that has raised LOG_LEVEL.
func TestRequestLogGateWithRealLoggers(t *testing.T) {
	tests := []struct {
		name    string
		level   logging.Level
		emitted bool
	}{
		{"default INFO still logs", logging.INFO, true},
		{"NOTICE skips", logging.NOTICE, false},
		{"WARN skips", logging.WARN, false},
	}

	for _, tt := range tests {
		for _, build := range []struct {
			kind string
			make func(logging.Level) logging.Logger
		}{
			{"logger", logging.NewLogger},
			{"remotelogger", func(l logging.Level) logging.Logger {
				return remotelogger.New(l, "", time.Second)
			}},
		} {
			t.Run(build.kind+"/"+tt.name, func(t *testing.T) {
				out := testutil.StdoutOutputForFunc(func() {
					srw := &StatusResponseWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}
					req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody)

					handleRequestLog(srw, req, time.Now(), "tid", trace.SpanContext{}, build.make(tt.level))
				})

				assert.Equal(t, tt.emitted, strings.Contains(out, "/x"),
					"the gate must agree with what the logger would emit")
			})
		}
	}
}

// TestRequestLogGateNeverSuppresses5xx is the same guard against a real logger:
// a server error goes through Error and must survive however high the
// informational level is set.
func TestRequestLogGateNeverSuppresses5xx(t *testing.T) {
	out := testutil.StderrOutputForFunc(func() {
		srw := &StatusResponseWriter{
			ResponseWriter: httptest.NewRecorder(),
			status:         http.StatusInternalServerError,
		}
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/boom", http.NoBody)

		handleRequestLog(srw, req, time.Now(), "tid", trace.SpanContext{},
			remotelogger.New(logging.ERROR, "", time.Second))
	})

	assert.Contains(t, out, "/boom", "a 5xx must be logged even at ERROR")
}
