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

// TestRouter_PathNormalization tests the path normalization function directly.
func TestRouter_PathNormalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{input: "/hello", expected: "/hello"},
		{input: "//hello", expected: "/hello"},
		{input: "///hello", expected: "/hello"},
		{input: "/hello//world", expected: "/hello/world"},
		{input: "//hello//world//", expected: "/hello/world/"},
		{input: "/", expected: "/"},
		{input: "//", expected: "/"},
		{input: "///", expected: "/"},
		{input: "", expected: "/"},
		{input: "/api//v1///users", expected: "/api/v1/users"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := normalizePathSlashes(tc.input)
			assert.Equal(t, tc.expected, result, "Path normalization failed")
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
			name: "Serve existing file",
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
			name: "Serve default 404 message when 404.html is missing",
			setupFiles: func() error {
				// Don't create 404.html, just return nil to test default 404 behavior
				return nil
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
