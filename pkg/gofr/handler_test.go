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

	h, err := a.healthHandler(ctx)

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

// downstream returns a dependency URL that makes GoFr's own dependency checks report DEGRADED
// (500) or UP (200), so readiness can be exercised against both framework verdicts.
func downstream(t *testing.T, healthy bool) string {
	t.Helper()

	code := http.StatusInternalServerError
	if healthy {
		code = http.StatusOK
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	}))
	t.Cleanup(srv.Close)

	return srv.URL
}

// assertReadiness drives healthHandler and asserts the outcome. wantStatus is the status expected in
// the redacted {name, status} body of a 200; an empty wantStatus expects a not-ready result instead —
// no body and a 503 whose message is only "DOWN", the reason staying in the logs.
func assertReadiness(t *testing.T, a *App, ctx *Context, wantStatus, msg string) {
	t.Helper()

	h, err := a.healthHandler(ctx)

	if wantStatus != "" {
		require.NoError(t, err, msg)

		resp, ok := h.(healthResponse)
		require.True(t, ok, "%s: expected healthResponse, got %T", msg, h)
		assert.Equal(t, wantStatus, resp.Status, msg)

		return
	}

	require.Nil(t, h, msg)

	var sc interface{ StatusCode() int }

	require.ErrorAs(t, err, &sc, msg)
	assert.Equal(t, http.StatusServiceUnavailable, sc.StatusCode(), msg)
	assert.Equal(t, statusDown, err.Error(), msg)
}

// errCheck is the failure a registered readiness check reports in these tests.
var errCheck = errors.New("dependency unavailable")

func TestApp_SetReadinessCheck(t *testing.T) {
	// propagate requires GoFr's checks in addition to the app's own; override ignores them.
	propagate := func(appErr error) ReadinessCheck {
		return func(_ *Context, framework error) error {
			if framework != nil {
				return framework
			}

			return appErr
		}
	}

	override := func(appErr error) ReadinessCheck {
		return func(*Context, error) error { return appErr }
	}

	tests := []struct {
		desc       string
		depHealthy bool
		check      ReadinessCheck
		// wantStatus is the status reported with a 200; empty means a not-ready 503 is expected.
		wantStatus string
	}{
		{"no check registered still answers 200 with the aggregate", false, nil, "DEGRADED"},
		{"propagating check is ready when both sides pass", true, propagate(nil), statusUp},
		{"propagating check fails on the framework verdict", false, propagate(nil), ""},
		{"propagating check fails on its own verdict", true, propagate(errCheck), ""},
		{"overriding check ignores a degraded framework", false, override(nil), statusUp},
		{"overriding check still gates readiness", true, override(errCheck), ""},
	}

	for i, tc := range tests {
		testutil.NewServerConfigs(t)

		a := New()
		a.AddHTTPService("test-service", downstream(t, tc.depHealthy))

		if tc.check != nil {
			a.SetReadinessCheck(tc.check)
		}

		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "", http.NoBody)
		ctx := newContext(nil, gofrHTTP.NewRequest(req), a.container)

		assertReadiness(t, a, ctx, tc.wantStatus, fmt.Sprintf("TEST[%d], Failed.\n%s", i, tc.desc))
	}
}

func TestApp_frameworkReadiness(t *testing.T) {
	tests := []struct {
		desc       string
		depHealthy bool
		wantErr    bool
	}{
		{"all dependencies healthy", true, false},
		{"a dependency down", false, true},
	}

	for i, tc := range tests {
		testutil.NewServerConfigs(t)

		a := New()
		a.AddHTTPService("test-service", downstream(t, tc.depHealthy))

		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "", http.NoBody)
		ctx := newContext(nil, gofrHTTP.NewRequest(req), a.container)

		err := frameworkReadiness(ctx)

		if !tc.wantErr {
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			continue
		}

		require.ErrorIs(t, err, errFrameworkChecks, "TEST[%d], Failed.\n%s", i, tc.desc)
		// The aggregate status may be reported, but never the per-dependency detail behind it.
		assert.Contains(t, err.Error(), "DEGRADED", "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func TestApp_logReadiness(t *testing.T) {
	tests := []struct {
		desc  string
		check ReadinessCheck
		want  string
	}{
		{"nothing logged without an app check", nil, ""},
		{
			"a registered check is announced at startup",
			func(*Context, error) error { return nil },
			"readiness: app check registered",
		},
	}

	for i, tc := range tests {
		logs := testutil.StdoutOutputForFunc(func() {
			testutil.NewServerConfigs(t)

			a := New()
			if tc.check != nil {
				a.SetReadinessCheck(tc.check)
			}

			a.logReadiness()
		})

		if tc.want == "" {
			assert.NotContains(t, logs, "readiness:", "TEST[%d], Failed.\n%s", i, tc.desc)
			continue
		}

		assert.Contains(t, logs, tc.want, "TEST[%d], Failed.\n%s", i, tc.desc)
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
