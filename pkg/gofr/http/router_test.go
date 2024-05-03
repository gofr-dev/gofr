package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/testutil"
)

func TestRouter(t *testing.T) {
	log := testutil.StdoutOutputForFunc(func() {
		cfg := map[string]string{"HTTP_PORT": "8000", "LOG_LEVEL": "INFO"}
		c := container.NewContainer(config.NewMockConfig(cfg))

		c.Metrics().NewCounter("test-counter", "test")

		// Create a new router instance using the mock container
		router := NewRouter(c)

		// Add a test handler to the router
		router.Add("GET", "/test", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Send a request to the test handler
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Verify the response
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	// verify if middleware logger is properly functioning inside new router
	if !strings.Contains(log, "\"method\":\"GET\",\"ip\":\"192.0.2.1:1234\",\"uri\":\"/test\",\"response\":200") {
		t.Errorf("TestRouter Failed! expected log not found: %v", log)
	}
}

func TestRouterWithMiddleware(t *testing.T) {
	log := testutil.StdoutOutputForFunc(func() {
		cfg := map[string]string{"HTTP_PORT": "8000", "LOG_LEVEL": "INFO"}
		c := container.NewContainer(config.NewMockConfig(cfg))

		c.Metrics().NewCounter("test-counter", "test")

		// Create a new router instance using the mock container
		router := NewRouter(c)

		router.UseMiddleware(func(inner http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Test-Middleware", "applied")
				inner.ServeHTTP(w, r)
			})
		})

		// Add a test handler to the router
		router.Add("GET", "/test", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		// Send a request to the test handler
		req := httptest.NewRequest("GET", "/test", http.NoBody)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		// Verify the response
		assert.Equal(t, http.StatusOK, rec.Code)
		// checking if the testMiddleware has added the required header in the response properly.
		testHeaderValue := rec.Header().Get("X-Test-Middleware")
		assert.Equal(t, "applied", testHeaderValue, "Test_UseMiddleware Failed! header value mismatch.")
	})

	// verify if middleware logger is properly functioning inside new router
	if !strings.Contains(log, "\"method\":\"GET\",\"ip\":\"192.0.2.1:1234\",\"uri\":\"/test\",\"response\":200") {
		t.Errorf("TestRouter Failed! expected log not found: %v", log)
	}
}
