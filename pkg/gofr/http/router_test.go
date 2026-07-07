package http

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestRouter(t *testing.T) {
	port := testutil.GetFreePort(t)

	cfg := map[string]string{"HTTP_PORT": fmt.Sprint(port), "LOG_LEVEL": "INFO"}
	c := container.NewContainer(config.NewMockConfig(cfg))

	c.Metrics().NewCounter("test-counter", "test")

	// Create a new router instance using the mock container
	router := NewRouter()

	// Add a test handler to the router
	router.Add(http.MethodGet, "/test", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Send a request to the test handler
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRouterWithMiddleware(t *testing.T) {
	port := testutil.GetFreePort(t)

	cfg := map[string]string{"HTTP_PORT": fmt.Sprint(port), "LOG_LEVEL": "INFO"}
	c := container.NewContainer(config.NewMockConfig(cfg))

	c.Metrics().NewCounter("test-counter", "test")

	// Create a new router instance using the mock container
	router := NewRouter()

	router.UseMiddleware(func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Test-Middleware", "applied")
			inner.ServeHTTP(w, r)
		})
	})

	// Add a test handler to the router
	router.Add(http.MethodGet, "/test", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Send a request to the test handler
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, rec.Code)
	// checking if the testMiddleware has added the required header in the response properly.
	testHeaderValue := rec.Header().Get("X-Test-Middleware")
	assert.Equal(t, "applied", testHeaderValue, "Test_UseMiddleware Failed! header value mismatch.")
}

// TestRouter_DoubleSlashPath_GET verifies that GET requests with double slashes
// are normalized and routed correctly to the GET handler.
func TestRouter_DoubleSlashPath_GET(t *testing.T) {
	router := NewRouter()

	getHandlerCalled := false
	postHandlerCalled := false

	// Register both GET and POST handlers for /hello
	router.Add(http.MethodGet, "/hello", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		getHandlerCalled = true

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("GET handler"))
	}))

	router.Add(http.MethodPost, "/hello", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		postHandlerCalled = true

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("POST handler"))
	}))

	tests := []struct {
		desc string
		path string
	}{
		{desc: "GET request to /hello", path: "/hello"},
		{desc: "GET request to //hello", path: "//hello"},
		{desc: "GET request to ///hello", path: "///hello"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			getHandlerCalled = false
			postHandlerCalled = false

			req := httptest.NewRequest(http.MethodGet, tc.path, http.NoBody)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code, "Status code mismatch")
			assert.True(t, getHandlerCalled, "GET handler should be called")
			assert.False(t, postHandlerCalled, "POST handler should NOT be called")
			assert.Equal(t, "GET handler", rec.Body.String(), "Response body mismatch")
			assert.Empty(t, rec.Header().Get("Location"), "No redirect should be issued")
		})
	}
}

// TestIsCleanPath pins the fast-path predicate that ServeHTTP uses to skip
// path.Clean for already-canonical URLs. A wrong negative would re-run the
// normalizer pointlessly (performance regression); a wrong positive would
// route a non-canonical URL without cleaning it (correctness regression).
func TestIsCleanPath(t *testing.T) {
	canonical := []string{
		"/",
		"/users",
		"/api/v1/things",
		"/users/42",
		"/a/b/c/d",
		"/path-with-dashes",
		"/path.with.dots",
		"/users/42.json",
	}
	dirty := []string{
		"",             // empty
		"users",        // no leading slash
		"//",           // double slash root
		"//users",      // leading double slash
		"/users//42",   // mid double slash
		"/.",           // trailing /.
		"/..",          // trailing /..
		"/./users",     // /./
		"/../users",    // /../
		"/users/.",     // /.
		"/users/..",    // /..
		"/users/./42",  // /./ mid
		"/users/../42", // /../ mid
		"/users/",      // trailing slash on non-root
	}

	for _, p := range canonical {
		if !isCleanPath(p) {
			t.Errorf("isCleanPath(%q) = false, want true", p)
		}
	}

	for _, p := range dirty {
		if isCleanPath(p) {
			t.Errorf("isCleanPath(%q) = true, want false", p)
		}
	}
}

// TestRouter_PathNormalization tests the path normalization function directly.
func TestRouter_PathNormalization(t *testing.T) {
	router := NewRouter()

	// Register handlers for testing
	router.Add(http.MethodGet, "/hello", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))

	router.Add(http.MethodGet, "/api/v1/users", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("users"))
	}))

	router.Add(http.MethodGet, "/bar", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("bar"))
	}))

	tests := []struct {
		name         string
		input        string
		expectedCode int
		expectedBody string
	}{
		{name: "simple path", input: "/hello", expectedCode: http.StatusOK, expectedBody: "hello"},
		{name: "double slash", input: "//hello", expectedCode: http.StatusOK, expectedBody: "hello"},
		{name: "triple slash", input: "///hello", expectedCode: http.StatusOK, expectedBody: "hello"},
		{name: "multiple slashes in middle", input: "/api//v1///users", expectedCode: http.StatusOK, expectedBody: "users"},
		{name: "current directory dot", input: "/.", expectedCode: http.StatusNotFound, expectedBody: "404 page not found\n"},
		{name: "parent directory", input: "/..", expectedCode: http.StatusNotFound, expectedBody: "404 page not found\n"},
		{name: "relative path no leading slash", input: "/hello", expectedCode: http.StatusOK, expectedBody: "hello"},
		{name: "parent directory traversal", input: "/foo/../bar", expectedCode: http.StatusOK, expectedBody: "bar"},
		{name: "parent directory with relative path", input: "/../hello", expectedCode: http.StatusOK, expectedBody: "hello"},
		{name: "root path", input: "/", expectedCode: http.StatusNotFound, expectedBody: "404 page not found\n"},
		{name: "empty path", input: "/", expectedCode: http.StatusNotFound, expectedBody: "404 page not found\n"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.input, http.NoBody)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, tc.expectedCode, rec.Code, "Status code mismatch")
			assert.Equal(t, tc.expectedBody, rec.Body.String(), "Response body mismatch")
		})
	}
}

// TestRouter_DoubleSlashPath_POST verifies that POST requests with double slashes
// are normalized and routed correctly to the POST handler.
func TestRouter_DoubleSlashPath_POST(t *testing.T) {
	router := NewRouter()

	getHandlerCalled := false
	postHandlerCalled := false

	// Register both GET and POST handlers for /hello
	router.Add(http.MethodGet, "/hello", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		getHandlerCalled = true

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("GET handler"))
	}))

	router.Add(http.MethodPost, "/hello", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		postHandlerCalled = true

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("POST handler"))
	}))

	tests := []struct {
		desc string
		path string
	}{
		{desc: "POST request to /hello", path: "/hello"},
		{desc: "POST request to //hello", path: "//hello"},
		{desc: "POST request to ////hello", path: "////hello"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			getHandlerCalled = false
			postHandlerCalled = false

			req := httptest.NewRequest(http.MethodPost, tc.path, http.NoBody)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code, "Status code mismatch")
			assert.True(t, postHandlerCalled, "POST handler should be called")
			assert.False(t, getHandlerCalled, "GET handler should NOT be called")
			assert.Equal(t, "POST handler", rec.Body.String(), "Response body mismatch")
			assert.Empty(t, rec.Header().Get("Location"), "No redirect should be issued")
		})
	}
}

func Test_StaticFileServing_Static(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name             string
		setupFiles       func() error
		path             string
		staticServerPath string
		expectedCode     int
		expectedBody     string
	}{
		{
			name: "Serve existing file from /static",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("Hello, World!"), 0600)
			},
			path:             "/static/test.txt",
			staticServerPath: "/static",
			expectedCode:     http.StatusOK,
			expectedBody:     "Hello, World!",
		},
		{
			name: "Serve existing file from /",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("Hello, Root!"), 0600)
			},
			path:             "/test.txt",
			staticServerPath: "/",
			expectedCode:     http.StatusOK,
			expectedBody:     "Hello, Root!",
		},
		{
			name: "Serve existing file from /public",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("Hello, Public!"), 0600)
			},
			path:             "/public/test.txt",
			staticServerPath: "/public",
			expectedCode:     http.StatusOK,
			expectedBody:     "Hello, Public!",
		},
		{
			name: "Serve 404.html for non-existent file",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "404.html"), []byte("<html>404 Not Found</html>"), 0600)
			},
			path:             "/static/nonexistent.html",
			staticServerPath: "/static",
			expectedCode:     http.StatusNotFound,
			expectedBody:     "<html>404 Not Found</html>",
		},
		{
			name: "Serve default 404 message when 404.html is missing",
			setupFiles: func() error {
				return os.Remove(filepath.Join(tempDir, "404.html"))
			},
			path:             "/static/nonexistent.html",
			staticServerPath: "/static",
			expectedCode:     http.StatusNotFound,
			expectedBody:     "404 Not Found",
		},
		{
			name: "Access forbidden OpenAPI JSON",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, DefaultSwaggerFileName), []byte(`{"openapi": "3.0.0"}`), 0600)
			},
			path:             "/static/openapi.json",
			staticServerPath: "/static",
			expectedCode:     http.StatusForbidden,
			expectedBody:     "403 Forbidden",
		},
		{
			name: "Serving File with no Read permission",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "restricted.txt"), []byte("Restricted content"), 0000)
			},
			path:             "/static/restricted.txt",
			staticServerPath: "/static",
			expectedCode:     http.StatusInternalServerError,
			expectedBody:     "500 Internal Server Error",
		},
	}

	runStaticFileTests(t, tempDir, testCases)
}

func Test_isRestrictedFile(t *testing.T) {
	tests := []struct {
		name          string
		directoryName string
		url           string
		absPath       string
		expected      bool
	}{
		{
			name:          "file inside static directory is not restricted",
			directoryName: "/app/public",
			url:           "/index.html",
			absPath:       "/app/public/index.html",
			expected:      false,
		},
		{
			name:          "openapi.json inside static directory is restricted",
			directoryName: "/app/public",
			url:           "/openapi.json",
			absPath:       "/app/public/openapi.json",
			expected:      true,
		},
		{
			name:          "file outside static directory is restricted",
			directoryName: "/app/public",
			url:           "/secret.txt",
			absPath:       "/app/secret.txt",
			expected:      true,
		},
		{
			name:          "sibling directory with shared prefix is restricted",
			directoryName: "/app/public",
			url:           "/secret.txt",
			absPath:       "/app/publicother/secret.txt",
			expected:      true,
		},
		{
			name:          "nested file inside static directory is not restricted",
			directoryName: "/app/public",
			url:           "/sub/page.html",
			absPath:       "/app/public/sub/page.html",
			expected:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := staticFileConfig{directoryName: tc.directoryName}
			result := cfg.isRestrictedFile(tc.url, tc.absPath)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func runStaticFileTests(t *testing.T, tempDir string, testCases []struct {
	name             string
	setupFiles       func() error
	path             string
	staticServerPath string
	expectedCode     int
	expectedBody     string
}) {
	t.Helper()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.setupFiles(); err != nil {
				t.Fatalf("Failed to set up files: %v", err)
			}

			logger := logging.NewMockLogger(logging.DEBUG)

			router := NewRouter()
			router.AddStaticFiles(logger, tc.staticServerPath, tempDir)

			req := httptest.NewRequest(http.MethodGet, tc.path, http.NoBody)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedCode, w.Code)
			assert.Equal(t, tc.expectedBody, strings.TrimSpace(w.Body.String()))
		})
	}
}

// discardingResponseWriter is a zero-cost ResponseWriter for benchmarks
// that should measure routing/handler cost without including the per-iter
// allocation of httptest.NewRecorder. Shared across the http-package
// benchmarks (router, responder).
type discardingResponseWriter struct {
	h http.Header
}

func (d *discardingResponseWriter) Header() http.Header {
	if d.h == nil {
		d.h = http.Header{}
	}

	return d.h
}

func (*discardingResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (*discardingResponseWriter) WriteHeader(int)             {}

// noopHandler returns an http.Handler that discards the request and
// writes nothing. Used by router benchmarks to isolate router cost from
// any handler-side work.
func noopHandler() http.Handler {
	return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
}

// BenchmarkRouter_Get_Static measures the per-request cost of routing
// a request to an exact-match route (no path parameters). Closest
// approximation to TFB's /plaintext routing cost.
//
// Hot path under measurement:
//   - Router.ServeHTTP path normalization (ServeHTTP, this file)
//   - gorilla/mux.Router.ServeHTTP route matching
//
// PR targets that should move this number:
//   - PR-12 (path-clean fast path) — small win
//   - PR-N (gorilla/mux → chi) — largest win, separate initiative
func BenchmarkRouter_Get_Static(b *testing.B) {
	r := NewRouter()
	r.Add(http.MethodGet, "/plaintext", noopHandler())

	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/plaintext", http.NoBody)
	w := &discardingResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

// BenchmarkRouter_Get_PathParam measures the per-request cost of routing
// to a path with a parameter ({id}). Includes gorilla/mux's parameter
// extraction overhead, which is part of why mux is slower than radix-tree
// routers (chi, httprouter).
func BenchmarkRouter_Get_PathParam(b *testing.B) {
	r := NewRouter()
	r.Add(http.MethodGet, "/users/{id}", noopHandler())

	req := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/users/42", http.NoBody)
	w := &discardingResponseWriter{}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}
