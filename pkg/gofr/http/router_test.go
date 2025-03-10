package http

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
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

func TestServeExistingFile(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test file
	testContent := []byte("Hello, World!")

	err := os.WriteFile(filepath.Join(tempDir, "test.txt"), testContent, 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	router := NewRouter()
	router.AddStaticFiles("/static", tempDir)

	req := httptest.NewRequest(http.MethodGet, "/static/test.txt", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), testContent) {
		t.Errorf("Expected body %q, got %q", testContent, w.Body.Bytes())
	}
}

func TestNonExistentFileWith404Page(t *testing.T) {
	tempDir := t.TempDir()

	// Create a 404.html file
	notFoundContent := []byte("<html>404 Not Found</html>")

	err := os.WriteFile(filepath.Join(tempDir, "404.html"), notFoundContent, 0600)
	if err != nil {
		t.Fatalf("Failed to create 404.html: %v", err)
	}

	router := NewRouter()
	router.AddStaticFiles("/static", tempDir)

	req := httptest.NewRequest(http.MethodGet, "/static/nonexistent.html", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status code 404, got %d", w.Code)
	}

	if !bytes.Equal(w.Body.Bytes(), notFoundContent) {
		t.Errorf("Expected body %q, got %q", notFoundContent, w.Body.Bytes())
	}
}

func TestNonExistentFileWithout404Page(t *testing.T) {
	tempDir := t.TempDir() // No 404.html

	router := NewRouter()
	router.AddStaticFiles("/static", tempDir)

	req := httptest.NewRequest(http.MethodGet, "/static/nonexistent.html", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status code 404, got %d", w.Code)
	}

	expectedBody := "404 not found"
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %q, got %q", expectedBody, w.Body.String())
	}
}

func TestAccessOpenAPIJSONForbidden(t *testing.T) {
	tempDir := t.TempDir()

	// Create openapi.json file
	openAPIContent := []byte(`{"openapi": "3.0.0"}`)

	err := os.WriteFile(filepath.Join(tempDir, DefaultSwaggerFileName), openAPIContent, 0600)
	if err != nil {
		t.Fatalf("Failed to create openapi.json: %v", err)
	}

	router := NewRouter()
	router.AddStaticFiles("/static", tempDir)

	req := httptest.NewRequest(http.MethodGet, "/static/openapi.json", http.NoBody)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status code 403, got %d", w.Code)
	}

	expectedBody := "403 forbidden"
	if w.Body.String() != expectedBody {
		t.Errorf("Expected body %q, got %q", expectedBody, w.Body.String())
	}
}
