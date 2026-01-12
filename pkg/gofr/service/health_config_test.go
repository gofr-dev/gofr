package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

func TestUpdateCircuitBreakerHealthConfig_DirectCircuitBreaker(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()

	svc := &httpService{
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 3, Interval: time.Second}, svc)

	updateCircuitBreakerHealthConfig(cb, "custom-health", 15)

	assert.Equal(t, "custom-health", cb.healthEndpoint)
	assert.Equal(t, 15, cb.healthTimeout)
}

func TestUpdateCircuitBreakerHealthConfig_ThroughRetryProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()

	svc := &httpService{
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 3, Interval: time.Second}, svc)
	retry := &retryProvider{HTTP: cb, maxRetries: 3}

	updateCircuitBreakerHealthConfig(retry, "health-endpoint", 20)

	assert.Equal(t, "health-endpoint", cb.healthEndpoint)
	assert.Equal(t, 20, cb.healthTimeout)
}

func TestUpdateCircuitBreakerHealthConfig_ThroughRateLimiter(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()

	svc := &httpService{
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 3, Interval: time.Second}, svc)
	rl := &rateLimiter{HTTP: cb}

	updateCircuitBreakerHealthConfig(rl, "status", 30)

	assert.Equal(t, "status", cb.healthEndpoint)
	assert.Equal(t, 30, cb.healthTimeout)
}

func TestUpdateCircuitBreakerHealthConfig_ThroughAuthProvider(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()

	svc := &httpService{
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 3, Interval: time.Second}, svc)
	auth := &authProvider{HTTP: cb}

	updateCircuitBreakerHealthConfig(auth, "ping", 25)

	assert.Equal(t, "ping", cb.healthEndpoint)
	assert.Equal(t, 25, cb.healthTimeout)
}

func TestUpdateCircuitBreakerHealthConfig_ThroughCustomHeader(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()

	svc := &httpService{
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 3, Interval: time.Second}, svc)
	ch := &customHeader{HTTP: cb}

	updateCircuitBreakerHealthConfig(ch, "ready", 10)

	assert.Equal(t, "ready", cb.healthEndpoint)
	assert.Equal(t, 10, cb.healthTimeout)
}

func TestHealthConfig_AddOption_UpdatesCircuitBreaker(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()

	svc := &httpService{
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 3, Interval: time.Second}, svc)

	healthConfig := HealthConfig{
		HealthEndpoint: "breeds",
		Timeout:        10,
	}

	result := healthConfig.AddOption(cb)

	// Verify circuit breaker was updated with health config
	assert.Equal(t, "breeds", cb.healthEndpoint)
	assert.Equal(t, 10, cb.healthTimeout)

	// Verify result is a customHealthService wrapping the circuit breaker
	customHealth, ok := result.(*customHealthService)
	assert.True(t, ok)
	assert.Equal(t, "breeds", customHealth.healthEndpoint)
	assert.Equal(t, 10, customHealth.timeout)
}

func TestHealthConfig_AddOption_DefaultTimeout(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()

	svc := &httpService{
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 3, Interval: time.Second}, svc)

	// HealthConfig without explicit timeout
	healthConfig := HealthConfig{
		HealthEndpoint: "health",
	}

	result := healthConfig.AddOption(cb)

	// Verify default timeout (5) is used
	assert.Equal(t, "health", cb.healthEndpoint)
	assert.Equal(t, defaultTimeout, cb.healthTimeout)

	customHealth, ok := result.(*customHealthService)
	assert.True(t, ok)
	assert.Equal(t, defaultTimeout, customHealth.timeout)
}

func TestUpdateCircuitBreakerHealthConfig_DeepNestedChain(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()

	svc := &httpService{
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	// Create a deeply nested chain: customHeader -> rateLimiter -> retryProvider -> circuitBreaker -> httpService
	cb := NewCircuitBreaker(CircuitBreakerConfig{Threshold: 3, Interval: time.Second}, svc)
	retry := &retryProvider{HTTP: cb, maxRetries: 3}
	rl := &rateLimiter{HTTP: retry}
	ch := &customHeader{HTTP: rl}

	updateCircuitBreakerHealthConfig(ch, "deep-health", 42)

	assert.Equal(t, "deep-health", cb.healthEndpoint)
	assert.Equal(t, 42, cb.healthTimeout)
}
