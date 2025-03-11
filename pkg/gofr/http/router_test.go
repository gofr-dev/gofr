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

func TestStaticFileServing_Static(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name         string
		setupFiles   func() error
		path         string
		expectedCode int
		expectedBody string
	}{
		{
			name: "Serve existing file from /static",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("Hello, World!"), 0600)
			},
			path:         "/static/test.txt",
			expectedCode: http.StatusOK,
			expectedBody: "Hello, World!",
		},
		{
			name: "Serve 404.html for non-existent file",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "404.html"), []byte("<html>404 Not Found</html>"), 0600)
			},
			path:         "/static/nonexistent.html",
			expectedCode: http.StatusNotFound,
			expectedBody: "<html>404 Not Found</html>",
		},
		{
			name: "Serve default 404 message when 404.html is missing",
			setupFiles: func() error {
				return os.Remove(filepath.Join(tempDir, "404.html"))
			},
			path:         "/static/nonexistent.html",
			expectedCode: http.StatusNotFound,
			expectedBody: "404 Not Found",
		},
		{
			name: "Access forbidden OpenAPI JSON",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, DefaultSwaggerFileName), []byte(`{"openapi": "3.0.0"}`), 0600)
			},
			path:         "/static/openapi.json",
			expectedCode: http.StatusForbidden,
			expectedBody: "403 Forbidden",
		},
		{
			name: "Serving File with no Read permission",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "restricted.txt"), []byte("Restricted content"), 0000)
			},
			path:         "/static/restricted.txt",
			expectedCode: http.StatusInternalServerError,
			expectedBody: "500 Internal Server Error",
		},
	}

	runStaticFileTests(t, tempDir, "/static", testCases)
}

// Testing files being served at an endpoint named public.
func TestStaticFileServing_Public(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name         string
		setupFiles   func() error
		path         string
		expectedCode int
		expectedBody string
	}{
		{
			name: "Serve existing file from /public",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("Hello, Public!"), 0600)
			},
			path:         "/public/test.txt",
			expectedCode: http.StatusOK,
			expectedBody: "Hello, Public!",
		},
	}

	runStaticFileTests(t, tempDir, "/public", testCases)
}

// testing files being served at root level.
func TestStaticFileServing_Root(t *testing.T) {
	tempDir := t.TempDir()

	testCases := []struct {
		name         string
		setupFiles   func() error
		path         string
		expectedCode int
		expectedBody string
	}{
		{
			name: "Serve existing file from /",
			setupFiles: func() error {
				return os.WriteFile(filepath.Join(tempDir, "test.txt"), []byte("Hello, Root!"), 0600)
			},
			path:         "/test.txt",
			expectedCode: http.StatusOK,
			expectedBody: "Hello, Root!",
		},
	}

	runStaticFileTests(t, tempDir, "/", testCases)
}

func runStaticFileTests(t *testing.T, tempDir, basePath string, testCases []struct {
	name         string
	setupFiles   func() error
	path         string
	expectedCode int
	expectedBody string
}) {
	t.Helper()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.setupFiles(); err != nil {
				t.Fatalf("Failed to set up files: %v", err)
			}

			logger := logging.NewMockLogger(logging.DEBUG)

			router := NewRouter()
			router.AddStaticFiles(logger, basePath, tempDir)

			req := httptest.NewRequest(http.MethodGet, tc.path, http.NoBody)
			w := httptest.NewRecorder()

			router.ServeHTTP(w, req)

			assert.Equal(t, tc.expectedCode, w.Code)
			assert.Equal(t, tc.expectedBody, strings.TrimSpace(w.Body.String()))
		})
	}
}
