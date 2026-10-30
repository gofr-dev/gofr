package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
)

func requestLogJSON(tb testing.TB, v any) string {
	tb.Helper()

	var buf bytes.Buffer

	require.NoError(tb, json.NewEncoder(&buf).Encode(v))

	return string(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
}

func requestLogCorpus() []*RequestLog {
	full := &RequestLog{
		TraceID: "0123456789abcdef0123456789abcdef", SpanID: "0123456789abcdef",
		StartTime: "2026-10-30T15:04:05.123456789+05:30", ResponseTime: 1234, Method: http.MethodGet,
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64)", IP: "192.0.2.10", URI: "/users/42?a=1&b=<2>", Response: 200,
	}

	return []*RequestLog{
		full, {}, {Response: 404}, {ResponseTime: -1}, {Method: http.MethodPost, URI: `/q"x\y`},
		{TraceID: full.TraceID, Response: 500}, {UserAgent: "curl/8.0", IP: "::1"},
	}
}

// TestAppendRequestLogJSONMatchesEncodingJSON pins the request-log line byte for byte against
// encoding/json, including omitted fields and HTML-escaped characters.
func TestAppendRequestLogJSONMatchesEncodingJSON(t *testing.T) {
	for _, rl := range requestLogCorpus() {
		got, ok := appendRequestLogJSON(nil, rl)
		require.True(t, ok, "%+v", rl)
		assert.Equal(t, requestLogJSON(t, rl), string(got), "%+v", rl)
	}
}

func TestAppendRequestLogJSONFallsBack(t *testing.T) {
	for _, v := range []any{(*RequestLog)(nil), RequestLog{}, "x", &RequestLog{UserAgent: "ünïcode"}, &RequestLog{URI: "/a\nb"}} {
		_, ok := appendRequestLogJSON(nil, v)
		assert.False(t, ok, "%#v must fall back to encoding/json", v)
	}
}

func FuzzAppendRequestLogJSON(f *testing.F) {
	f.Add("0123456789abcdef0123456789abcdef", "GET", "Mozilla/5.0", "192.0.2.10", "/users/42", int64(1234), 200)
	f.Add("", "", "", "", "", int64(0), 0)
	f.Add("t", "P<O>ST", "ua\"\\", "ip&", "/\u2028", int64(-5), -1)

	f.Fuzz(func(t *testing.T, traceID, method, ua, ip, uri string, rt int64, status int) {
		rl := &RequestLog{TraceID: traceID, SpanID: traceID, StartTime: method, ResponseTime: rt,
			Method: method, UserAgent: ua, IP: ip, URI: uri, Response: status}

		if got, ok := appendRequestLogJSON(nil, rl); ok {
			require.Equal(t, requestLogJSON(t, rl), string(got), "%+v", rl)
		}
	})
}

// TestLoggingRequestLogLineUnchanged pins the full line the real logger writes for a request:
// every field except the volatile time and timings must be what encoding/json writes for the
// same entry.
func TestLoggingRequestLogLineUnchanged(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)

	stdout := os.Stdout
	os.Stdout = w
	logger := logging.NewLogger(logging.INFO)
	os.Stdout = stdout

	router := mux.NewRouter()
	router.Use(Logging(LogProbes{}, logger))
	router.NewRoute().Methods(http.MethodGet).Path("/users/{id}").
		Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) }))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users/42?x=<1>&y=2", http.NoBody)
	req.RemoteAddr = "192.0.2.10:5555"
	req.Header.Set("User-Agent", "bench/1.0")

	router.ServeHTTP(httptest.NewRecorder(), req)
	w.Close()

	var line bytes.Buffer

	_, err = line.ReadFrom(r)
	require.NoError(t, err)

	var decoded struct {
		Level       string          `json:"level"`
		Time        json.RawMessage `json:"time"`
		Message     RequestLog      `json:"message"`
		GofrVersion string          `json:"gofrVersion"`
	}

	require.NoError(t, json.Unmarshal(line.Bytes(), &decoded), line.String())

	// Re-encode what was decoded with encoding/json, in the logger's own envelope shape, and
	// compare the bytes: any difference in field order, omission or escaping shows up here.
	var re bytes.Buffer

	require.NoError(t, json.NewEncoder(&re).Encode(struct {
		Level       string          `json:"level"`
		Time        json.RawMessage `json:"time"`
		Message     *RequestLog     `json:"message"`
		GofrVersion string          `json:"gofrVersion"`
	}{decoded.Level, decoded.Time, &decoded.Message, decoded.GofrVersion}))

	assert.Equal(t, re.String(), line.String())
	assert.Equal(t, "/users/42?x=<1>&y=2", decoded.Message.URI)
	assert.Equal(t, http.StatusTeapot, decoded.Message.Response)
}

// BenchmarkLogging_Emitted measures the middleware at the default level, where the request log is
// built and written for every request: the path the JSON fast path speeds up.
func BenchmarkLogging_Emitted(b *testing.B) {
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	require.NoError(b, err)

	defer devNull.Close()

	stdout := os.Stdout
	os.Stdout = devNull
	logger := logging.NewLogger(logging.INFO)
	os.Stdout = stdout

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
