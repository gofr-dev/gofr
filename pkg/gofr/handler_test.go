package gofr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/http/response"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

var (
	errTest = errors.New("some error")
)

func TestHandler_ServeHTTP(t *testing.T) {
	testCases := []struct {
		desc       string
		method     string
		data       any
		err        error
		statusCode int
		body       string
	}{
		{"method is get, data is nil and error is nil", http.MethodGet, nil, nil, http.StatusOK,
			`{}`},
		{"method is get, data is mil, error is not nil", http.MethodGet, nil, errTest, http.StatusInternalServerError,
			`{"error":{"message":"some error"}}`},
		{"method is get, data is mil, error is http error", http.MethodGet, nil, gofrHTTP.ErrorEntityNotFound{}, http.StatusNotFound,
			`{"error":{"message":"No entity found with : "}}`},
		{"method is post, data is nil and error is nil", http.MethodPost, "Created", nil, http.StatusCreated,
			`{"data":"Created"}`},
		{"method is delete, data is nil and error is nil", http.MethodDelete, nil, nil, http.StatusNoContent,
			`{}`},
	}

	for i, tc := range testCases {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, "/", http.NoBody)
		c := &container.Container{
			Logger: logging.NewLogger(logging.FATAL),
		}

		handler{
			function: func(*Context) (any, error) {
				return tc.data, tc.err
			},
			container: c,
		}.ServeHTTP(w, r)

		assert.Containsf(t, w.Body.String(), tc.body, "TEST[%d], Failed.\n%s", i, tc.desc)
		assert.Equal(t, tc.statusCode, w.Code, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestHandler_ServeHTTP_Timeout(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	h := handler{requestTimeout: 100 * time.Millisecond}

	h.container = &container.Container{Logger: logging.NewLogger(logging.FATAL)}
	h.function = func(*Context) (any, error) {
		time.Sleep(200 * time.Millisecond)

		return "hey", nil
	}

	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusRequestTimeout, w.Code, "TestHandler_ServeHTTP_Timeout Failed")
	assert.Contains(t, w.Body.String(), "request timed out", "TestHandler_ServeHTTP_Timeout Failed")
}

func TestHandler_ServeHTTP_Panic(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	h := handler{}

	h.container = &container.Container{Logger: logging.NewLogger(logging.FATAL)}
	h.function = func(*Context) (any, error) {
		panic("runtime panic")
	}

	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code, "TestHandler_ServeHTTP_Panic Failed")

	assert.Contains(t, w.Body.String(), http.StatusText(http.StatusInternalServerError), "TestHandler_ServeHTTP_Panic Failed")
}

func TestHandler_ServeHTTP_WithHeaders(t *testing.T) {
	testCases := []struct {
		desc       string
		method     string
		data       any
		headers    map[string]string
		err        error
		statusCode int
		body       string
	}{
		{
			desc:   "Response with headers, method is GET, no error",
			method: http.MethodGet,
			data: response.Response{
				Headers: map[string]string{
					"X-Custom-Header": "custom-value",
					"Content-Type":    "application/json",
				},
				Data: map[string]string{
					"message": "Hello, World!",
				},
			},
			headers: map[string]string{
				"X-Custom-Header": "custom-value",
				"Content-Type":    "application/json",
			},
			statusCode: http.StatusOK,
			body:       `{"message":"Hello, World!"}`,
		},
		{
			desc:       "No headers, method is GET, data is simple string, no error",
			method:     http.MethodGet,
			data:       "simple string",
			statusCode: http.StatusOK,
			body:       `"simple string"`,
		},
	}

	for i, tc := range testCases {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(tc.method, "/", http.NoBody)
		c := &container.Container{
			Logger: logging.NewLogger(logging.FATAL),
		}

		handler{
			function: func(*Context) (any, error) {
				return tc.data, tc.err
			},
			container: c,
		}.ServeHTTP(w, r)

		assert.Containsf(t, w.Body.String(), tc.body, "TEST[%d], Failed.\n%s", i, tc.desc)

		assert.Equal(t, tc.statusCode, w.Code, "TEST[%d], Failed.\n%s", i, tc.desc)

		for key, expectedValue := range tc.headers {
			assert.Equal(t, expectedValue, w.Header().Get(key), "TEST[%d], Failed. Header mismatch: %s", i, key)
		}
	}
}

func TestHandler_faviconHandlerError(t *testing.T) {
	c := Context{
		Context: t.Context(),
	}

	d, _ := os.ReadFile("static/favicon.ico")

	// renaming the file to produce the error case and rename it back to original after completion of test.
	_, err := os.Stat("static/favicon.ico")
	if err != nil {
		t.Errorf("favicon.ico file not found in static directory")
		return
	}

	err = os.Rename("static/favicon.ico", "static/newFavicon.ico")
	if err != nil {
		t.Errorf("error in renaming favicon.ico!")
	}

	defer func() {
		err = os.Rename("static/newFavicon.ico", "static/favicon.ico")
		if err != nil {
			t.Errorf("error in renaming file back to favicon.ico")
		}
	}()

	data, err := faviconHandler(&c)

	require.NoError(t, err, "TEST Failed.\n")

	assert.Equal(t, response.File{
		Content:     d,
		ContentType: "image/x-icon",
	}, data, "TEST Failed.\n")
}

func TestHandler_faviconHandler(t *testing.T) {
	c := Context{
		Context: t.Context(),
	}

	d, _ := os.ReadFile("static/favicon.ico")
	data, err := faviconHandler(&c)

	require.NoError(t, err, "TEST Failed.\n")

	assert.Equal(t, response.File{
		Content:     d,
		ContentType: "image/x-icon",
	}, data, "TEST Failed.\n")
}

func TestHandler_catchAllHandler(t *testing.T) {
	c := Context{
		Context: t.Context(),
	}

	data, err := catchAllHandler(&c)

	assert.Nil(t, data, "TEST Failed.\n")

	assert.Equal(t, gofrHTTP.ErrorInvalidRoute{}, err, "TEST Failed.\n")
}

func TestHandler_livelinessHandler(t *testing.T) {
	resp, err := liveHandler(&Context{})

	require.NoError(t, err)
	assert.Contains(t, fmt.Sprint(resp), "UP")
}

func TestHandler_healthHandler(t *testing.T) {
	testutil.NewServerConfigs(t)

	a := New()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/.well-known/alive", r.URL.Path)

		w.WriteHeader(http.StatusOK)
	}))

	a.AddHTTPService("test-service", server.URL)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "", http.NoBody)

	r := gofrHTTP.NewRequest(req)

	ctx := newContext(nil, r, a.container)

	h, err := healthHandler(ctx)

	require.NoError(t, err)
	assert.NotNil(t, h)
}

func TestHandler_ServeHTTP_ContextCanceled(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(r.Context())
	cancel() // Cancel immediately

	r = r.WithContext(ctx)

	h := handler{
		container: &container.Container{Logger: logging.NewLogger(logging.FATAL)},
	}

	h.function = func(*Context) (any, error) {
		time.Sleep(100 * time.Millisecond)
		return "should not reach", nil
	}

	h.ServeHTTP(w, r)

	assert.Equal(t, 499, w.Code, "Should return HTTP 499 for client closed request")
	assert.Contains(t, w.Body.String(), "client closed request", "Should contain error message")
}

// TestHandler_ServeHTTP_InlinePath_NormalResponse pins the byte shape of
// a successful response on the inline (no-timeout, no-WS) path so a
// refactor of the goroutine-elimination optimization can not silently
// drop bytes from the on-the-wire JSON envelope. Goroutine path is
// exercised by TestHandler_ServeHTTP_Timeout.
func TestHandler_ServeHTTP_InlinePath_NormalResponse(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

	h := handler{
		container: &container.Container{Logger: logging.NewLogger(logging.FATAL)},
	}
	h.function = func(*Context) (any, error) {
		return map[string]string{"message": "hi"}, nil
	}

	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `{"data":{"message":"hi"}}`)
}

// TestHandler_ServeHTTP_InlinePath_HandlerError pins the wire shape when
// the handler returns an error on the inline path. The "wire shape
// preservation" claim is the whole point of this optimization — if this
// test fails, the optimization is breaking error responses for users
// without REQUEST_TIMEOUT.
func TestHandler_ServeHTTP_InlinePath_HandlerError(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

	h := handler{
		container: &container.Container{Logger: logging.NewLogger(logging.FATAL)},
	}
	h.function = func(*Context) (any, error) {
		return nil, gofrHTTP.ErrorEntityNotFound{}
	}

	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), `"error"`)
	assert.Contains(t, w.Body.String(), "No entity found")
}

func TestHandler_ServeHTTP_ContextTimeout(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)

	// Create context with 50ms timeout
	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Millisecond)
	defer cancel()

	r = r.WithContext(ctx)

	h := handler{
		container: &container.Container{Logger: logging.NewLogger(logging.FATAL)},
	}

	h.function = func(*Context) (any, error) {
		// Sleep longer than timeout to trigger deadline exceeded
		time.Sleep(10 * time.Millisecond)
		return "should timeout", nil
	}

	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusRequestTimeout, w.Code, "Should return HTTP 408 for context timeout")
	assert.Contains(t, w.Body.String(), "request timed out")
}

func TestIntegration_ConcurrentClientCancellations(t *testing.T) {
	ports := testutil.NewServerConfigs(t)
	t.Setenv("METRICS_PORT", strconv.Itoa(ports.MetricsPort))

	t.Setenv("HTTP_PORT", strconv.Itoa(ports.HTTPPort))

	var requestCount atomic.Int64

	var completedCount atomic.Int64

	app := New()

	app.GET("/concurrent", func(_ *Context) (any, error) {
		requestCount.Add(1)

		// Simulate work
		time.Sleep(10 * time.Millisecond)

		completedCount.Add(1)

		return map[string]string{"status": "completed"}, nil
	})

	go func() {
		app.Run()
	}()

	time.Sleep(5 * time.Millisecond)

	// Launch multiple concurrent requests with early cancellation
	const numRequests = 10

	var wg sync.WaitGroup

	var canceledCount atomic.Int64

	for i := 0; i < numRequests; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			ctx, cancel := context.WithCancel(t.Context())

			// Cancel after short delay
			go func() {
				time.Sleep(5 * time.Millisecond)
				cancel()
			}()

			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprint("http://localhost:", ports.HTTPPort, "/concurrent"), http.NoBody)
			client := &http.Client{}

			resp, err := client.Do(req)
			if err != nil {
				if strings.Contains(err.Error(), "canceled") {
					canceledCount.Add(1)
				}

				return
			}

			if resp != nil {
				resp.Body.Close()
			}
		}()
	}

	wg.Wait()
	time.Sleep(50 * time.Millisecond) // Let remaining requests complete

	// Verify some requests were canceled
	canceled := canceledCount.Load()
	started := requestCount.Load()
	completed := completedCount.Load()

	t.Logf("Started: %d, Completed: %d, Canceled: %d", started, completed, canceled)
	assert.Positive(t, canceled, "Some requests should have been canceled")
	assert.LessOrEqual(t, completed, started, "Completed should not exceed started")
}

func TestIntegration_ServerTimeout(t *testing.T) {
	ports := testutil.NewServerConfigs(t)

	t.Setenv("METRICS_PORT", strconv.Itoa(ports.MetricsPort))
	t.Setenv("HTTP_PORT", strconv.Itoa(ports.HTTPPort))

	// Set GoFr's built-in request timeout to 1 second
	t.Setenv("REQUEST_TIMEOUT", "1")

	app := New()

	// Handler that takes longer than server timeout
	app.GET("/timeout-test", func(*Context) (any, error) {
		// Sleep longer than REQUEST_TIMEOUT (1 second)
		time.Sleep(2 * time.Second)
		return map[string]string{"message": "should timeout"}, nil
	})

	go func() {
		app.Run()
	}()

	// Wait for server to be ready
	testURL := fmt.Sprintf("http://localhost:%d/timeout-test", ports.HTTPPort)
	client := &http.Client{Timeout: 10 * time.Second} // Client timeout longer than server

	ready := false

	for i := 0; i < 50; i++ {
		time.Sleep(10 * time.Millisecond)

		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, testURL, http.NoBody)
		require.NoError(t, err)

		resp, err := client.Do(req)
		if err == nil {
			ready = true

			resp.Body.Close()

			break
		}
	}

	require.True(t, ready, "Server should be ready")

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, testURL, http.NoBody)
	require.NoError(t, err)

	resp, err := client.Do(req)

	require.NoError(t, err, "HTTP request should complete")

	defer resp.Body.Close()

	// GoFr should return 408 Request Timeout
	assert.Equal(t, http.StatusRequestTimeout, resp.StatusCode,
		"Server should return 408 for request timeout")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var errorResponse map[string]any

	err = json.Unmarshal(body, &errorResponse)
	require.NoError(t, err)

	errorObj := errorResponse["error"].(map[string]any)
	assert.Equal(t, "request timed out", errorObj["message"])
}

// ---------------------------------------------------------------------------
// Characterization suite for handler.ServeHTTP.
//
// Pins the handler-execution contract with exact status codes and exact body
// bytes for both execution paths (serveInline and serveWithGoroutine), for
// timeout, cancellation, panic recovery and the WebSocket bypass. The two paths
// are asserted against the SAME expectations wherever they should agree, so a
// refactor cannot let them drift.
// ---------------------------------------------------------------------------

// charHandler builds a handler with a silent logger.
func charHandler(fn Handler, timeout time.Duration) handler {
	return handler{
		function:       fn,
		container:      &container.Container{Logger: logging.NewLogger(logging.FATAL)},
		requestTimeout: timeout,
	}
}

// charServe runs h against a fresh recorder for the given method.
func charServe(t *testing.T, h handler, method string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequestWithContext(t.Context(), method, "/", http.NoBody))

	return w
}

// TestHandler_Char_BothPathsAgree pins that serveInline (requestTimeout == 0)
// and serveWithGoroutine (requestTimeout > 0) produce byte-identical responses
// for every ordinary handler outcome. The timeout is generous so the deadline
// branch never fires.
func TestHandler_Char_BothPathsAgree(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		fn         Handler
		wantStatus int
		wantBody   string
	}{
		{
			"nil-nil", http.MethodGet,
			func(*Context) (any, error) { return nil, nil },
			http.StatusOK, "{}\n",
		},
		{
			"data", http.MethodGet,
			func(*Context) (any, error) { return map[string]string{"m": "hi"}, nil },
			http.StatusOK, "{\"data\":{\"m\":\"hi\"}}\n",
		},
		{
			"post-created", http.MethodPost,
			func(*Context) (any, error) { return "Created", nil },
			http.StatusCreated, "{\"data\":\"Created\"}\n",
		},
		{
			"delete-no-content", http.MethodDelete,
			func(*Context) (any, error) { return nil, nil },
			http.StatusNoContent, "{}\n",
		},
		{
			"plain-error", http.MethodGet,
			func(*Context) (any, error) { return nil, errTest },
			http.StatusInternalServerError, "{\"error\":{\"message\":\"some error\"}}\n",
		},
		{
			"status-coded-error", http.MethodGet,
			func(*Context) (any, error) { return nil, gofrHTTP.ErrorEntityNotFound{Name: "id", Value: "3"} },
			http.StatusNotFound, "{\"error\":{\"message\":\"No entity found with id: 3\"}}\n",
		},
		{
			"data-and-error-partial", http.MethodGet,
			func(*Context) (any, error) { return map[string]string{"p": "ok"}, errTest },
			http.StatusPartialContent, "{\"error\":{\"message\":\"some error\"},\"data\":{\"p\":\"ok\"}}\n",
		},
		{
			"invalid-route", http.MethodGet,
			catchAllHandler,
			http.StatusNotFound, "{\"error\":{\"message\":\"route not registered\"}}\n",
		},
		{
			"liveness", http.MethodGet,
			liveHandler,
			http.StatusOK, "{\"data\":{\"status\":\"UP\"}}\n",
		},
	}

	paths := []struct {
		name    string
		timeout time.Duration
	}{
		{"inline", 0},
		{"goroutine", time.Minute},
	}

	for _, p := range paths {
		for _, tc := range cases {
			t.Run(p.name+"/"+tc.name, func(t *testing.T) {
				w := charServe(t, charHandler(tc.fn, p.timeout), tc.method)

				assert.Equal(t, tc.wantStatus, w.Code, "status code")
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"), "Content-Type")
				assert.Equal(t, tc.wantBody, w.Body.String(), "body bytes")
			})
		}
	}
}

// TestHandler_Char_PanicRecovery pins panic recovery on BOTH paths: the client
// gets a 500 with the generic "Internal Server Error" envelope, and neither the
// panic value nor any stack frame leaks onto the wire.
func TestHandler_Char_PanicRecovery(t *testing.T) {
	const secret = "SUPER-SECRET-PANIC-VALUE"

	errSecretPanic := errors.New(secret) //nolint:err113 // panic payload under test.

	panics := []struct {
		name string
		fn   Handler
	}{
		{"string-panic", func(*Context) (any, error) { panic(secret) }},
		{"error-panic", func(*Context) (any, error) { panic(errSecretPanic) }},
		{"runtime-panic", func(*Context) (any, error) {
			s := []int{1}
			idx := len(s) + 1

			return s[idx], nil // index out of range
		}},
	}

	timeouts := []struct {
		name    string
		timeout time.Duration
	}{
		{"inline", 0},
		{"goroutine", time.Minute},
	}

	for _, to := range timeouts {
		for _, p := range panics {
			t.Run(to.name+"/"+p.name, func(t *testing.T) {
				w := charServe(t, charHandler(p.fn, to.timeout), http.MethodGet)

				assert.Equal(t, http.StatusInternalServerError, w.Code)
				assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
				//nolint:testifylint // exact bytes are the contract.
				assert.Equal(t, "{\"error\":{\"message\":\"Internal Server Error\"}}\n", w.Body.String())

				// Nothing about the panic reaches the client.
				assert.NotContains(t, w.Body.String(), secret)
				assert.NotContains(t, w.Body.String(), "goroutine")
				assert.NotContains(t, w.Body.String(), "handler_test.go")
			})
		}
	}
}

// TestHandler_Char_PanicAfterPartialResult pins that a handler which panics
// AFTER computing a value still yields the bare 500 envelope — the partial
// result is discarded, not returned as 206.
func TestHandler_Char_PanicAfterPartialResult(t *testing.T) {
	for _, timeout := range []time.Duration{0, time.Minute} {
		w := charServe(t, charHandler(func(*Context) (any, error) {
			defer panic("late")

			return map[string]string{"leaked": "value"}, nil
		}, timeout), http.MethodGet)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		//nolint:testifylint // exact bytes are the contract.
		assert.Equal(t, "{\"error\":{\"message\":\"Internal Server Error\"}}\n", w.Body.String())
	}
}

// TestHandler_Char_ServerTimeout pins the server-side request timeout: the
// deadline branch of serveWithGoroutine fires while the handler is still
// running and the client gets exactly 408 with the timeout envelope. Any result
// the handler eventually produces is dropped.
func TestHandler_Char_ServerTimeout(t *testing.T) {
	release := make(chan struct{})
	defer close(release)

	h := charHandler(func(*Context) (any, error) {
		<-release

		return "too late", nil
	}, 10*time.Millisecond)

	w := charServe(t, h, http.MethodGet)

	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	//nolint:testifylint // exact bytes are the contract.
	assert.Equal(t, "{\"error\":{\"message\":\"request timed out\"}}\n", w.Body.String())
}

// TestHandler_Char_InlineDeadlineExceeded pins the inline path's post-hoc
// deadline check: the handler runs to completion (Go cannot kill a goroutine),
// but because the inherited context expired the result is dropped and the
// client sees 408 rather than the handler's value.
func TestHandler_Char_InlineDeadlineExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
	defer cancel()

	var ran bool

	h := charHandler(func(*Context) (any, error) {
		ran = true

		time.Sleep(20 * time.Millisecond)

		return "computed anyway", nil
	}, 0)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody))

	assert.True(t, ran, "the handler still runs to completion on the inline path")
	assert.Equal(t, http.StatusRequestTimeout, w.Code)
	//nolint:testifylint // exact bytes are the contract.
	assert.Equal(t, "{\"error\":{\"message\":\"request timed out\"}}\n", w.Body.String())
}

// TestHandler_Char_ClientCanceled pins client cancellation on both paths: the
// non-standard 499 with the "client closed request" envelope.
func TestHandler_Char_ClientCanceled(t *testing.T) {
	for _, tc := range []struct {
		name    string
		timeout time.Duration
	}{
		{"inline", 0},
		{"goroutine", time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			h := charHandler(func(*Context) (any, error) { return "ignored", nil }, tc.timeout)

			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody))

			assert.Equal(t, gofrHTTP.StatusClientClosedRequest, w.Code)
			assert.Equal(t, 499, w.Code)
			//nolint:testifylint // exact bytes are the contract.
			assert.Equal(t, "{\"error\":{\"message\":\"client closed request\"}}\n", w.Body.String())
		})
	}
}

// TestHandler_Char_PanicBeatsCancellation pins the precedence on the inline
// path: a handler that panics on an already-canceled context reports the panic
// (500), because the cancellation remap is skipped when panicked is set.
func TestHandler_Char_PanicBeatsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	h := charHandler(func(*Context) (any, error) { panic("boom") }, 0)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequestWithContext(ctx, http.MethodGet, "/", http.NoBody))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	//nolint:testifylint // exact bytes are the contract.
	assert.Equal(t, "{\"error\":{\"message\":\"Internal Server Error\"}}\n", w.Body.String())
}

// TestHandler_Char_ContextDeadlinePropagation pins which context the user
// handler actually receives on each path.
func TestHandler_Char_ContextDeadlinePropagation(t *testing.T) {
	t.Run("no-timeout-has-no-deadline", func(t *testing.T) {
		var hasDeadline bool

		charServe(t, charHandler(func(c *Context) (any, error) {
			_, hasDeadline = c.Deadline()

			return nil, nil
		}, 0), http.MethodGet)

		assert.False(t, hasDeadline)
	})

	t.Run("timeout-sets-deadline", func(t *testing.T) {
		var (
			hasDeadline bool
			remaining   time.Duration
		)

		charServe(t, charHandler(func(c *Context) (any, error) {
			var dl time.Time

			dl, hasDeadline = c.Deadline()
			remaining = time.Until(dl)

			return nil, nil
		}, time.Minute), http.MethodGet)

		assert.True(t, hasDeadline)
		// Normalized: only the ballpark is asserted, never the wall clock.
		assert.Positive(t, remaining)
		assert.LessOrEqual(t, remaining, time.Minute)
	})
}

// TestHandler_Char_WebSocketBypassesTimeout pins that a WebSocket upgrade
// request ignores requestTimeout entirely: the handler's context carries no
// deadline and a handler slower than the configured timeout still returns its
// own result rather than a 408.
func TestHandler_Char_WebSocketBypassesTimeout(t *testing.T) {
	var hasDeadline bool

	h := charHandler(func(c *Context) (any, error) {
		_, hasDeadline = c.Deadline()

		time.Sleep(30 * time.Millisecond)

		return "ws-ok", nil
	}, 5*time.Millisecond)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ws", http.NoBody)
	r.Header.Set("Connection", "Upgrade")
	r.Header.Set("Upgrade", "websocket")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.False(t, hasDeadline, "a WebSocket request must not inherit the request timeout")
	assert.Equal(t, http.StatusOK, w.Code)
	//nolint:testifylint // exact bytes are the contract.
	assert.Equal(t, "{\"data\":\"ws-ok\"}\n", w.Body.String())
}

// TestHandler_Char_WebSocketStillWritesJSONEnvelope pins a sharp edge: despite
// the "do not respond with HTTP headers since this is a WebSocket request"
// comment in handleWebSocketUpgrade, the normal JSON envelope IS still written
// for an upgrade request whose handler returns without hijacking the
// connection. handleWebSocketUpgrade is a no-op in both of its branches.
func TestHandler_Char_WebSocketStillWritesJSONEnvelope(t *testing.T) {
	h := charHandler(func(*Context) (any, error) { return "ws", nil }, 0)

	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/ws", http.NoBody)
	r.Header.Set("Connection", "Upgrade")
	r.Header.Set("Upgrade", "websocket")

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	//nolint:testifylint // exact bytes are the contract.
	assert.Equal(t, "{\"data\":\"ws\"}\n", w.Body.String())
}

// TestHandler_Char_ResponseCustomHeaders pins that ServeHTTP applies
// response.Response.Headers to the writer (Respond itself never does) and that
// the headers do not appear in the JSON body.
func TestHandler_Char_ResponseCustomHeaders(t *testing.T) {
	for _, timeout := range []time.Duration{0, time.Minute} {
		w := charServe(t, charHandler(func(*Context) (any, error) {
			return response.Response{
				Data:     map[string]string{"m": "hi"},
				Metadata: map[string]any{"page": 1},
				Headers:  map[string]string{"X-One": "1", "x-two": "2"},
			}, nil
		}, timeout), http.MethodGet)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "1", w.Header().Get("X-One"))
		// Header keys are canonicalized by net/http.
		assert.Equal(t, "2", w.Header().Get("X-Two"))
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		//nolint:testifylint // exact bytes are the contract.
		assert.Equal(t, "{\"metadata\":{\"page\":1},\"data\":{\"m\":\"hi\"}}\n", w.Body.String())
	}
}

// TestHandler_Char_ResponseCustomHeadersCanOverrideContentType pins that a
// handler-supplied Content-Type wins, because ServeHTTP sets the custom headers
// before Respond decides whether to default it.
func TestHandler_Char_ResponseCustomHeadersCanOverrideContentType(t *testing.T) {
	w := charServe(t, charHandler(func(*Context) (any, error) {
		return response.Response{
			Data:    "x",
			Headers: map[string]string{"Content-Type": "application/vnd.custom+json"},
		}, nil
	}, 0), http.MethodGet)

	assert.Equal(t, "application/vnd.custom+json", w.Header().Get("Content-Type"))
	//nolint:testifylint // exact bytes are the contract.
	assert.Equal(t, "{\"data\":\"x\"}\n", w.Body.String())
}

// TestHandler_Char_ResponsePointerHeadersIgnored pins that returning a POINTER
// to response.Response neither applies the custom headers nor takes the special
// Response envelope path — the type assertion in ServeHTTP is by value.
func TestHandler_Char_ResponsePointerHeadersIgnored(t *testing.T) {
	w := charServe(t, charHandler(func(*Context) (any, error) {
		return &response.Response{Data: "x", Headers: map[string]string{"X-One": "1"}}, nil
	}, 0), http.MethodGet)

	assert.Empty(t, w.Header().Get("X-One"), "custom headers are only applied for a value Response")
	//nolint:testifylint // exact bytes are the contract.
	assert.Equal(t, "{\"data\":{\"data\":\"x\"}}\n", w.Body.String())
}

// TestHandler_Char_SpecialResponseTypes pins that the special response types
// survive the handler path unchanged.
func TestHandler_Char_SpecialResponseTypes(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		w := charServe(t, charHandler(func(*Context) (any, error) {
			return response.File{Content: []byte("abc"), ContentType: "text/csv"}, nil
		}, 0), http.MethodGet)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "text/csv", w.Header().Get("Content-Type"))
		assert.Equal(t, "abc", w.Body.String())
	})

	t.Run("redirect-get", func(t *testing.T) {
		w := charServe(t, charHandler(func(*Context) (any, error) {
			return response.Redirect{URL: "/elsewhere"}, nil
		}, 0), http.MethodGet)

		assert.Equal(t, http.StatusFound, w.Code)
		assert.Equal(t, "/elsewhere", w.Header().Get("Location"))
		assert.Empty(t, w.Body.String())
	})

	t.Run("redirect-post", func(t *testing.T) {
		w := charServe(t, charHandler(func(*Context) (any, error) {
			return response.Redirect{URL: "/elsewhere"}, nil
		}, time.Minute), http.MethodPost)

		assert.Equal(t, http.StatusSeeOther, w.Code)
		assert.Equal(t, "/elsewhere", w.Header().Get("Location"))
	})

	t.Run("raw", func(t *testing.T) {
		w := charServe(t, charHandler(func(*Context) (any, error) {
			return response.Raw{Data: map[string]string{"k": "v"}}, nil
		}, 0), http.MethodGet)

		//nolint:testifylint // exact bytes are the contract.
		assert.Equal(t, "{\"k\":\"v\"}\n", w.Body.String())
	})
}

// TestHandler_Char_ErrorLogging pins WHICH log level each error class is
// emitted at by logError — the level is part of the operational contract even
// though it never reaches the client.
func TestHandler_Char_ErrorLogging(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantLevel string
		// ERROR (and above) is written to stderr; everything else to stdout.
		onStderr bool
	}{
		{"plain-error-is-ERROR", errTest, "ERROR", true},
		{"entity-not-found-is-INFO", gofrHTTP.ErrorEntityNotFound{}, "INFO", false},
		{"already-exists-is-WARN", gofrHTTP.ErrorEntityAlreadyExist{}, "WARN", false},
		{"invalid-route-is-INFO", gofrHTTP.ErrorInvalidRoute{}, "INFO", false},
		{"panic-recovery-is-ERROR", gofrHTTP.ErrorPanicRecovery{}, "ERROR", true},
		{"too-many-requests-is-WARN", gofrHTTP.ErrorTooManyRequests{}, "WARN", false},
		{"client-closed-is-DEBUG", gofrHTTP.ErrorClientClosedRequest{}, "DEBUG", false},
		{"request-timeout-is-INFO", gofrHTTP.ErrorRequestTimeout{}, "INFO", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			serve := func() {
				h := handler{
					function:  func(*Context) (any, error) { return nil, tc.err },
					container: &container.Container{Logger: logging.NewLogger(logging.DEBUG)},
				}

				h.ServeHTTP(httptest.NewRecorder(),
					httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))
			}

			logs := testutil.StdoutOutputForFunc(serve)
			if tc.onStderr {
				logs = testutil.StderrOutputForFunc(serve)
			}

			assert.Contains(t, logs, tc.wantLevel)
			assert.Contains(t, logs, tc.err.Error())
		})
	}
}

// TestHandler_Char_NoErrorNoLog pins that a successful handler logs nothing
// from logError.
func TestHandler_Char_NoErrorNoLog(t *testing.T) {
	logs := testutil.StdoutOutputForFunc(func() {
		h := handler{
			function:  func(*Context) (any, error) { return "ok", nil },
			container: &container.Container{Logger: logging.NewLogger(logging.DEBUG)},
		}

		h.ServeHTTP(httptest.NewRecorder(),
			httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody))
	})

	assert.Empty(t, logs)
}

// TestErrorLogEntry_Char_PrettyPrint pins the exact bytes of the pretty-printed
// error log line, ANSI color escapes included.
func TestErrorLogEntry_Char_PrettyPrint(t *testing.T) {
	var buf strings.Builder

	(&ErrorLogEntry{TraceID: "abc123", Error: "went wrong"}).PrettyPrint(&buf)

	assert.Equal(t, "\u001B[38;5;8mabc123 \u001B[38;5;202mwent wrong \n", buf.String())
}
