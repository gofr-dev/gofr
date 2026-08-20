package gofr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/gofr/config"
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
	require.NotNil(t, h)

	// The public endpoint must expose only {name, status} and never per-dependency details
	// (hosts, ports, credentials, connection stats). See issue #3802.
	resp, ok := h.(healthResponse)
	require.True(t, ok, "expected redacted healthResponse, got %T", h)
	assert.NotEmpty(t, resp.Status)

	body, err := json.Marshal(resp)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(body, &got))

	// Pin the exact key set rather than blocklisting known-bad keys, so any future field added to
	// the public body — an ES username, a port, a version — fails this test instead of leaking.
	assert.Equal(t, []string{"name", "status"}, slices.Sorted(maps.Keys(got)))
}

func TestHandler_aggregateStatus(t *testing.T) {
	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer live.Close()

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()

	tests := []struct {
		desc string
		url  string
		want string
	}{
		{"all dependencies healthy", live.URL, "UP"},
		{"a dependency down", dead.URL, "DEGRADED"},
	}

	for i, tc := range tests {
		testutil.NewServerConfigs(t)

		a := New()
		a.AddHTTPService("test-service", tc.url)

		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "", http.NoBody)
		ctx := newContext(nil, gofrHTTP.NewRequest(req), a.container)

		// Only the aggregate string survives — never a per-dependency details map.
		assert.Equal(t, tc.want, aggregateStatus(ctx), "TEST[%d], Failed.\n%s", i, tc.desc)
	}
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

// TestLogErrorStillCarriesTraceID is the feature guard for resolving the trace
// ID lazily: an erroring handler must still log it. The value is now formatted
// inside logError rather than on every request, so this pins that the error path
// did not lose it.
func TestLogErrorStillCarriesTraceID(t *testing.T) {
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10},
		SpanID:     trace.SpanID{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8},
		TraceFlags: trace.FlagsSampled,
	})

	logs := testutil.StderrOutputForFunc(func() {
		c := container.NewContainer(config.NewMockConfig(map[string]string{"LOG_LEVEL": "ERROR"}))
		h := handler{
			function:  func(*Context) (any, error) { return nil, errTraceIDGuard },
			container: c,
		}

		// Chain from t.Context(), not context.Background(): the chained WithContext replaces the
		// one NewRequestWithContext installed, so backgrounding it here would silently drop the
		// test's cancellation and make the NewRequestWithContext call pointless.
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody).
			WithContext(trace.ContextWithSpanContext(t.Context(), sc))

		h.ServeHTTP(httptest.NewRecorder(), req)
	})

	assert.Contains(t, logs, "0102030405060708090a0b0c0d0e0f10",
		"the error log must still carry the trace ID")
}

var errTraceIDGuard = errors.New("trace id guard")

// buildRealPathRouter mirrors what a GoFr application actually serves: the handler is wrapped in
// gofr's own handler{}, so newHTTPContext builds the Context/Request/Responder and the result goes
// back through responder.Respond as a JSON envelope.
//
// The existing BenchmarkRequest_FullChain family registers raw http.HandlerFunc values straight on
// the router, so none of that code runs there — it measures the middleware chain only.
//
// The chain itself comes from useGoFrMiddleware rather than being rebuilt here, so there is one
// definition to keep in sync with newHTTPServer.
func buildRealPathRouter(c *container.Container) http.Handler {
	r := gofrHTTP.NewRouter()

	useGoFrMiddleware(r, c)

	r.Add(http.MethodGet, "/real/{id}", handler{
		function:  func(ctx *Context) (any, error) { return map[string]string{"id": ctx.PathParam("id")}, nil },
		container: c,
	})

	return r
}

// BenchmarkRequest_RealHandlerPath measures the per-request allocations of the wrapped-handler path.
//
// Note what it does NOT exercise: the handler always returns a nil error, and the mock config
// installs no real TracerProvider, so the span context is invalid. After this PR TraceID().String()
// runs only inside logError, on the error path — so no trace ID is formatted here. The alloc delta
// this reports is the Context/Request/Responder bundling alone, which is what the PR claims.
func BenchmarkRequest_RealHandlerPath(b *testing.B) {
	c := container.NewContainer(config.NewMockConfig(map[string]string{"LOG_LEVEL": "ERROR"}))
	h := buildRealPathRouter(c)
	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/real/42", http.NoBody)
	w := &benchDiscardResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		h.ServeHTTP(w, req)
	}
}
