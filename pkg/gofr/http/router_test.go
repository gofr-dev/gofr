package http

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
)

func TestRouter(t *testing.T) {
	cfg := map[string]string{"HTTP_PORT": "8000", "LOG_LEVEL": "INFO"}
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
	cfg := map[string]string{"HTTP_PORT": "8000", "LOG_LEVEL": "INFO"}
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

func TestRouter_AddStaticFiles(t *testing.T) {
	cfg := map[string]string{"HTTP_PORT": "8000", "LOG_LEVEL": "INFO"}
	_ = container.NewContainer(config.NewMockConfig(cfg))

	createTestFileAndDirectory(t, "testDir")

	defer os.RemoveAll("testDir")

	time.Sleep(1 * time.Second)

	currentWorkingDir, _ := os.Getwd()

	// Create a new router instance using the mock container
	router := NewRouter()
	router.AddStaticFiles("/gofr", currentWorkingDir+"/testDir")

	// Send a request to the test handler
	req := httptest.NewRequest(http.MethodGet, "/gofr/indexTest.html", http.NoBody)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Verify the response
	assert.Equal(t, http.StatusOK, rec.Code)

	// Send a request to the test handler
	req = httptest.NewRequest(http.MethodGet, "/gofr/openapi.json", http.NoBody)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Verify the response
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func createTestFileAndDirectory(t *testing.T, dirName string) {
	t.Helper()

	htmlContent := []byte("<html><head><title>Test Static File</title></head><body><p>Testing Static File</p></body></html>")

	const indexHTML = "indexTest.html"

	directory := "./" + dirName

	if err := os.Mkdir("./"+dirName, os.ModePerm); err != nil {
		t.Fatalf("Couldn't create a "+dirName+" directory, error: %s", err)
	}

	file, err := os.Create(filepath.Join(directory, indexHTML))
	if err != nil {
		t.Fatalf("Couldn't create %s file", indexHTML)
	}

	_, err = file.Write(htmlContent)
	if err != nil {
		t.Fatalf("Couldn't write to %s file", indexHTML)
	}

	file.Close()

	file, err = os.Create(filepath.Join(directory, "openapi.json"))
	if err != nil {
		t.Fatalf("Couldn't create %s file", indexHTML)
	}

	file.Close()
}
