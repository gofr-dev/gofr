package middleware

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_getIPAddress(t *testing.T) {
	{
		// When RemoteAddr is set
		addr := "0.0.0.0:8080"
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://dummy", http.NoBody)

		require.NoError(t, err, "TEST Failed.\n")

		req.RemoteAddr = addr
		ip := getIPAddress(req)

		assert.Equal(t, addr, ip, "TEST Failed.\n")
	}

	{
		// When `X-Forwarded-For` header is set
		addr := "192.168.0.1:8080"
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://dummy", http.NoBody)

		require.NoError(t, err, "TEST Failed.\n")

		req.Header.Set("X-Forwarded-For", addr)
		ip := getIPAddress(req)

		assert.Equal(t, addr, ip, "TEST Failed.\n")
	}
}

func Test_LoggingMiddleware(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://dummy", http.NoBody)

		rr := httptest.NewRecorder()

		handler := Logging(logging.NewMockLogger(logging.DEBUG))(http.HandlerFunc(testHandler))

		handler.ServeHTTP(rr, req)
	})

	assert.Contains(t, logs, "GET    200")
}

func Test_LoggingMiddlewareError(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://dummy", http.NoBody)

		rr := httptest.NewRecorder()

		handler := Logging(logging.NewMockLogger(logging.ERROR))(http.HandlerFunc(testHandlerError))

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
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://dummy", http.NoBody)

		rr := httptest.NewRecorder()

		handler := Logging(logging.NewMockLogger(logging.DEBUG))(http.HandlerFunc(testStringPanicHandler))

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
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://dummy", http.NoBody)

		rr := httptest.NewRecorder()

		handler := Logging(logging.NewMockLogger(logging.DEBUG))(http.HandlerFunc(testErrorPanicHandler))

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
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://dummy", http.NoBody)

		rr := httptest.NewRecorder()

		handler := Logging(logging.NewMockLogger(logging.DEBUG))(http.HandlerFunc(testUnknownPanicHandler))

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
