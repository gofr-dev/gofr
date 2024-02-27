package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

func Test_getIPAddress(t *testing.T) {
	{
		// When RemoteAddr is set
		addr := "0.0.0.0:8080"
		req, err := http.NewRequestWithContext(context.Background(), "GET", "http://dummy", http.NoBody)

		assert.Nil(t, err, "TEST Failed.\n")

		req.RemoteAddr = addr
		ip := getIPAddress(req)

		assert.Equal(t, addr, ip, "TEST Failed.\n")
	}

	{
		// When `X-Forwarded-For` header is set
		addr := "192.168.0.1:8080"
		req, err := http.NewRequestWithContext(context.Background(), "GET", "http://dummy", http.NoBody)

		assert.Nil(t, err, "TEST Failed.\n")

		req.Header.Set("X-Forwarded-For", addr)
		ip := getIPAddress(req)

		assert.Equal(t, addr, ip, "TEST Failed.\n")
	}
}

func Test_LoggingMiddleware(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://dummy", http.NoBody)

		rr := httptest.NewRecorder()

		handler := Logging(testutil.NewMockLogger(testutil.DEBUGLOG))(http.HandlerFunc(testHandler))

		handler.ServeHTTP(rr, req)
	})

	assert.Contains(t, logs, "GET    200")
}

// Test handler that uses the middleware.
func testHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Test Handler"))
}

func Test_LoggingMiddlewareStringPanicHandling(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://dummy", http.NoBody)

		rr := httptest.NewRecorder()

		handler := Logging(testutil.NewMockLogger(testutil.DEBUGLOG))(http.HandlerFunc(testStringPanicHandler))

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
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://dummy", http.NoBody)

		rr := httptest.NewRecorder()

		handler := Logging(testutil.NewMockLogger(testutil.DEBUGLOG))(http.HandlerFunc(testErrorPanicHandler))

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
		req, _ := http.NewRequestWithContext(context.Background(), "GET", "http://dummy", http.NoBody)

		rr := httptest.NewRecorder()

		handler := Logging(testutil.NewMockLogger(testutil.DEBUGLOG))(http.HandlerFunc(testUnknownPanicHandler))

		handler.ServeHTTP(rr, req)
	})

	assert.Contains(t, logs, "gofr.dev/pkg/gofr/http/middleware.testUnknownPanicHandler")
}

// Test handler that uses the middleware.
func testUnknownPanicHandler(w http.ResponseWriter, _ *http.Request) {
	panic(w)
}
