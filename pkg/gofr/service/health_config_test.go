package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

func setupMockMetrics(t *testing.T) *MockMetrics {
	t.Helper()

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	return mockMetric
}

func TestHealthConfig_AddOption_SetsParentHealthEndpoint(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()

	mockMetric := setupMockMetrics(t)

	svc := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric,
		&CircuitBreakerConfig{Threshold: 3, Interval: time.Second},
	)

	healthConfig := HealthConfig{
		HealthEndpoint: "breeds",
		Timeout:        10,
	}

	result := healthConfig.AddOption(svc)

	// Verify httpService parent has health config set
	httpSvc := extractHTTPService(svc)
	require.NotNil(t, httpSvc)
	assert.Equal(t, "breeds", httpSvc.healthEndpoint)
	assert.Equal(t, 10, httpSvc.healthTimeout)

	// Verify customHealthService is returned
	customHealth, ok := result.(*customHealthService)
	assert.True(t, ok)
	assert.Equal(t, "breeds", customHealth.healthEndpoint)
	assert.Equal(t, 10, customHealth.timeout)
}

func TestHealthConfig_AddOption_DefaultTimeout(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()

	mockMetric := setupMockMetrics(t)

	svc := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric)

	healthConfig := HealthConfig{
		HealthEndpoint: "health",
		// Timeout not set - should use default
	}

	result := healthConfig.AddOption(svc)

	// Verify default timeout is used
	httpSvc := extractHTTPService(svc)
	require.NotNil(t, httpSvc)
	assert.Equal(t, "health", httpSvc.healthEndpoint)
	assert.Equal(t, defaultTimeout, httpSvc.healthTimeout)

	customHealth, ok := result.(*customHealthService)
	assert.True(t, ok)
	assert.Equal(t, defaultTimeout, customHealth.timeout)
}

func TestHealthConfig_AddOption_WithRetryAndCircuitBreaker(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()

	mockMetric := setupMockMetrics(t)

	// Create service with circuit breaker and retry
	svc := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric,
		&CircuitBreakerConfig{Threshold: 3, Interval: time.Second},
		&RetryConfig{MaxRetries: 3},
	)

	healthConfig := HealthConfig{
		HealthEndpoint: "status",
		Timeout:        15,
	}

	result := healthConfig.AddOption(svc)

	// Verify httpService parent has health config set
	httpSvc := extractHTTPService(svc)
	require.NotNil(t, httpSvc)
	assert.Equal(t, "status", httpSvc.healthEndpoint)
	assert.Equal(t, 15, httpSvc.healthTimeout)

	// Verify customHealthService wraps the chain
	customHealth, ok := result.(*customHealthService)
	assert.True(t, ok)
	assert.Equal(t, "status", customHealth.healthEndpoint)
}

func TestCircuitBreaker_UsesParentHealthEndpoint(t *testing.T) {
	// Server that returns 502 for /fail, 200 for /custom-health
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/custom-health":
			w.WriteHeader(http.StatusOK)
		case "/fail":
			w.WriteHeader(http.StatusBadGateway)
		case "/.well-known/alive":
			w.WriteHeader(http.StatusNotFound) // Default endpoint not available
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	mockMetric := setupMockMetrics(t)

	// Create service with circuit breaker AND health config
	svc := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric,
		&CircuitBreakerConfig{Threshold: 1, Interval: 200 * time.Millisecond},
		&HealthConfig{HealthEndpoint: "custom-health", Timeout: 5},
	)

	// First request returns 502 - failure count becomes 1
	resp, err := svc.Get(t.Context(), "fail", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	resp.Body.Close()

	// Second request - circuit opens and returns ErrCircuitOpen
	resp, err = svc.Get(t.Context(), "fail", nil)
	if err != nil {
		require.ErrorIs(t, err, ErrCircuitOpen)
		return
	}

	defer resp.Body.Close()

	require.ErrorIs(t, err, ErrCircuitOpen)

	// Wait for interval to pass
	time.Sleep(500 * time.Millisecond)

	// Circuit should recover using /custom-health (from parent httpService)
	resp, err = svc.Get(t.Context(), "success", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestCircuitBreaker_UsesDefaultHealthEndpoint_WhenNoHealthConfig(t *testing.T) {
	// Server where default health endpoint returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/alive":
			w.WriteHeader(http.StatusNotFound) // Default endpoint not available
		case "/fail":
			w.WriteHeader(http.StatusBadGateway)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	mockMetric := setupMockMetrics(t)

	// Create service with circuit breaker but NO health config
	svc := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric,
		&CircuitBreakerConfig{Threshold: 1, Interval: 200 * time.Millisecond},
	)

	// First request returns 502
	resp, err := svc.Get(t.Context(), "fail", nil)
	if err != nil {
		require.ErrorIs(t, err, ErrCircuitOpen)
		return
	}

	defer resp.Body.Close()

	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)

	// Second request - circuit opens
	resp, err = svc.Get(t.Context(), "fail", nil)
	if err != nil {
		require.ErrorIs(t, err, ErrCircuitOpen)
		return
	}

	defer resp.Body.Close()

	require.ErrorIs(t, err, ErrCircuitOpen)

	// Wait for interval
	time.Sleep(500 * time.Millisecond)

	// Circuit should NOT recover because /.well-known/alive returns 404
	resp, err = svc.Get(t.Context(), "success", nil)
	if err != nil {
		require.Error(t, err)
		return
	}

	defer resp.Body.Close()

	require.ErrorIs(t, err, ErrCircuitOpen)
}
