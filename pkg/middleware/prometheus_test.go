package middleware

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

// Helper function to create a new test router with PrometheusMiddleware.
func newTestRouter() *mux.Router {
	r := mux.NewRouter()
	r.Use(PrometheusMiddleware)
	r.HandleFunc("/api/v1/users", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	r.HandleFunc("/api/v1/products/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.HandleFunc("/api/v1/orders", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.HandleFunc("/api/v1/orders/{id}/reviews", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	r.HandleFunc("/.well-known/health-check", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return r
}

// Helper function to create a new test request.
func newTestRequest(method, path string) *http.Request {
	return httptest.NewRequest(method, path, http.NoBody)
}

func TestPrometheusMiddleware(t *testing.T) {
	r := newTestRouter()

	testCase := []struct {
		method         string
		path           string
		expectedStatus int
	}{
		{"GET", "/api/v1/users", http.StatusOK},
		{"GET", "/api/v1/products/123", http.StatusOK},
		{"GET", "/.well-known/health-check", http.StatusOK},
	}

	for i, tc := range testCase {
		req := newTestRequest(tc.method, tc.path)
		w := httptest.NewRecorder()

		r.ServeHTTP(w, req)

		if w.Code != tc.expectedStatus {
			assert.Equal(t, tc.expectedStatus, w.Code,
				"TestCase[%d] Failed: Expected status code %d, but got %d for %s %s", i, tc.expectedStatus, w.Code, tc.method, tc.path)
		}
	}
}

func TestGetSystemStats(t *testing.T) {
	s := getSystemStats()

	assert.Greater(t, int(s.numGoRoutines), 0, "Expected non-negative numGoRoutines, got %f", s.numGoRoutines)

	assert.Greater(t, int(s.alloc), 0, "Expected non-negative alloc, got %f", s.alloc)

	assert.Greater(t, int(s.totalAlloc), 0, "Expected non-negative totalAlloc, got %f", s.totalAlloc)

	assert.Greater(t, int(s.sys), 0, "Expected non-negative sys, got %f", s.sys)

	assert.GreaterOrEqualf(t, int(s.numGC), 0, "Expected non-negative numGC, got %v", int(s.numGC))
}

func TestPushDeprecatedFeature(t *testing.T) {
	// Reset Prometheus metrics before running the test
	prometheus.DefaultRegisterer.Unregister(deprecatedFeatureCount)

	// Set environment variables
	err := os.Setenv("APP_NAME", "TestApp")
	if err != nil {
		return
	}

	err = os.Setenv("APP_VERSION", "1.0.0")

	if err != nil {
		return
	}

	// Call PushDeprecatedFeature with a sample featureName
	featureName := "old_feature"
	PushDeprecatedFeature(featureName)

	// Verify the metric value
	metricValue := testutil.ToFloat64(deprecatedFeatureCount.With(prometheus.Labels{
		"appName":     "TestApp",
		"appVersion":  "1.0.0",
		"featureName": featureName,
	}))
	assert.Equal(t, 1.0, metricValue, "Unexpected metric value")
}
