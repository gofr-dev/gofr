package service

import (
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

func TestUpdateCircuitBreakerHealthConfig_DirectCircuitBreaker(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()

	mockMetric := setupMockMetrics(t)

	svc := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric,
		&CircuitBreakerConfig{Threshold: 3, Interval: time.Second},
	)

	cb, ok := svc.(*circuitBreaker)
	require.True(t, ok)

	updateCircuitBreakerHealthConfig(cb, "custom-health", 15)

	assert.Equal(t, "custom-health", cb.healthEndpoint)
	assert.Equal(t, 15, cb.healthTimeout)
}

func TestUpdateCircuitBreakerHealthConfig_ThroughRetryProvider(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()

	mockMetric := setupMockMetrics(t)

	svc := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric,
		&CircuitBreakerConfig{Threshold: 3, Interval: time.Second},
		&RetryConfig{MaxRetries: 3},
	)

	retry, ok := svc.(*retryProvider)
	require.True(t, ok)

	cb, ok := retry.HTTP.(*circuitBreaker)
	require.True(t, ok)

	updateCircuitBreakerHealthConfig(svc, "health-endpoint", 20)

	assert.Equal(t, "health-endpoint", cb.healthEndpoint)
	assert.Equal(t, 20, cb.healthTimeout)
}

func TestHealthConfig_AddOption_UpdatesCircuitBreaker(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()

	mockMetric := setupMockMetrics(t)

	svc := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric,
		&CircuitBreakerConfig{Threshold: 3, Interval: time.Second},
	)

	cb, ok := svc.(*circuitBreaker)
	require.True(t, ok)

	healthConfig := HealthConfig{
		HealthEndpoint: "breeds",
		Timeout:        10,
	}

	result := healthConfig.AddOption(svc)

	assert.Equal(t, "breeds", cb.healthEndpoint)
	assert.Equal(t, 10, cb.healthTimeout)

	customHealth, ok := result.(*customHealthService)
	assert.True(t, ok)
	assert.Equal(t, "breeds", customHealth.healthEndpoint)
	assert.Equal(t, 10, customHealth.timeout)
}

func TestHealthConfig_AddOption_DefaultTimeout(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()

	mockMetric := setupMockMetrics(t)

	svc := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric,
		&CircuitBreakerConfig{Threshold: 3, Interval: time.Second},
	)

	cb, ok := svc.(*circuitBreaker)
	require.True(t, ok)

	healthConfig := HealthConfig{
		HealthEndpoint: "health",
	}

	result := healthConfig.AddOption(svc)

	assert.Equal(t, "health", cb.healthEndpoint)
	assert.Equal(t, defaultTimeout, cb.healthTimeout)

	customHealth, ok := result.(*customHealthService)
	assert.True(t, ok)
	assert.Equal(t, defaultTimeout, customHealth.timeout)
}

func TestUpdateCircuitBreakerHealthConfig_DeepNestedChain(t *testing.T) {
	server := httptest.NewServer(nil)
	defer server.Close()

	mockMetric := setupMockMetrics(t)

	svc := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric,
		&CircuitBreakerConfig{Threshold: 3, Interval: time.Second},
		&RetryConfig{MaxRetries: 3},
	)

	retry, ok := svc.(*retryProvider)
	require.True(t, ok)

	cb, ok := retry.HTTP.(*circuitBreaker)
	require.True(t, ok)

	updateCircuitBreakerHealthConfig(svc, "deep-health", 42)

	assert.Equal(t, "deep-health", cb.healthEndpoint)
	assert.Equal(t, 42, cb.healthTimeout)
}
