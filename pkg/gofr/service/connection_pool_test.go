package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestConnectionPoolConfig_AddOption_CustomSettings(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
	}

	mockHTTPService := &httpService{
		Client: &http.Client{},
	}

	result := config.AddOption(mockHTTPService)

	assert.Equal(t, mockHTTPService, result)

	transport, ok := mockHTTPService.Client.Transport.(*http.Transport)

	assert.True(t, ok, "Transport should be of type *http.Transport")
	assert.Equal(t, 100, transport.MaxIdleConns)
	assert.Equal(t, 10, transport.MaxIdleConnsPerHost)
	assert.Equal(t, 30*time.Second, transport.IdleConnTimeout)
}

func TestConnectionPoolConfig_AddOption_ZeroValuesUseDefaults(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxIdleConns:        0,
		MaxIdleConnsPerHost: 0,
		IdleConnTimeout:     0,
	}

	mockHTTPService := &httpService{
		Client: &http.Client{},
	}

	result := config.AddOption(mockHTTPService)

	assert.Equal(t, mockHTTPService, result)

	transport, _ := mockHTTPService.Client.Transport.(*http.Transport)

	assert.Equal(t, 100, transport.MaxIdleConns)
	assert.Equal(t, 90*time.Second, transport.IdleConnTimeout)
}

func TestConnectionPoolConfig_AddOption_PartialConfiguration(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxIdleConnsPerHost: 20,
	}

	mockHTTPService := &httpService{
		Client: &http.Client{},
	}

	result := config.AddOption(mockHTTPService)

	assert.Equal(t, mockHTTPService, result)

	transport, _ := mockHTTPService.Client.Transport.(*http.Transport)

	assert.Equal(t, 100, transport.MaxIdleConns)
	assert.Equal(t, 20, transport.MaxIdleConnsPerHost)
	assert.Equal(t, 90*time.Second, transport.IdleConnTimeout)
}

func TestConnectionPoolConfig_AddOption_NonHTTPService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &ConnectionPoolConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
	}

	mockService := NewMockHTTP(ctrl)

	result := config.AddOption(mockService)

	assert.Equal(t, mockService, result)
}

func TestConnectionPoolConfig_AddOption_WithCircuitBreakerWrapper(t *testing.T) {
	baseService := &httpService{
		Client: &http.Client{},
	}

	wrappedService := NewCircuitBreaker(CircuitBreakerConfig{
		Threshold: 3,
		Interval:  1 * time.Second,
	}, baseService)

	config := &ConnectionPoolConfig{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 15,
		IdleConnTimeout:     60 * time.Second,
	}

	result := config.AddOption(wrappedService)

	assert.Equal(t, wrappedService, result)

	transport, ok := baseService.Client.Transport.(*http.Transport)

	assert.True(t, ok, "Transport should be of type *http.Transport")
	assert.Equal(t, 50, transport.MaxIdleConns)
	assert.Equal(t, 15, transport.MaxIdleConnsPerHost)
	assert.Equal(t, 60*time.Second, transport.IdleConnTimeout)
}

func TestConnectionPoolConfig_AddOption_WithRetryWrapper(t *testing.T) {
	baseService := &httpService{
		Client: &http.Client{},
	}

	wrappedService := (&RetryConfig{MaxRetries: 3}).AddOption(baseService)

	config := &ConnectionPoolConfig{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 15,
		IdleConnTimeout:     60 * time.Second,
	}

	result := config.AddOption(wrappedService)

	assert.Equal(t, wrappedService, result)

	transport, _ := baseService.Client.Transport.(*http.Transport)

	assert.Equal(t, 50, transport.MaxIdleConns)
	assert.Equal(t, 15, transport.MaxIdleConnsPerHost)
	assert.Equal(t, 60*time.Second, transport.IdleConnTimeout)
}

func TestConnectionPoolConfig_AddOption_WithAuthWrapper(t *testing.T) {
	baseService := &httpService{
		Client: &http.Client{},
	}

	wrappedService := &authProvider{
		auth: func(_ context.Context, headers map[string]string) (map[string]string, error) {
			return headers, nil
		},
		HTTP: baseService,
	}

	config := &ConnectionPoolConfig{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 15,
		IdleConnTimeout:     60 * time.Second,
	}

	result := config.AddOption(wrappedService)

	assert.Equal(t, wrappedService, result)

	transport, ok := baseService.Client.Transport.(*http.Transport)

	assert.True(t, ok, "Transport should be of type *http.Transport")
	assert.Equal(t, 50, transport.MaxIdleConns)
	assert.Equal(t, 15, transport.MaxIdleConnsPerHost)
	assert.Equal(t, 60*time.Second, transport.IdleConnTimeout)
}

func TestConnectionPoolConfig_AddOption_WithCustomHealthWrapper(t *testing.T) {
	baseService := &httpService{
		Client: &http.Client{},
	}

	wrappedService := (&HealthConfig{
		HealthEndpoint: "/health",
		Timeout:        5,
	}).AddOption(baseService)

	config := &ConnectionPoolConfig{
		MaxIdleConns:        50,
		MaxIdleConnsPerHost: 15,
		IdleConnTimeout:     60 * time.Second,
	}

	result := config.AddOption(wrappedService)

	assert.Equal(t, wrappedService, result)

	transport, ok := baseService.Client.Transport.(*http.Transport)

	assert.True(t, ok, "Transport should be of type *http.Transport")
	assert.Equal(t, 50, transport.MaxIdleConns)
	assert.Equal(t, 15, transport.MaxIdleConnsPerHost)
	assert.Equal(t, 60*time.Second, transport.IdleConnTimeout)
}

func TestConnectionPoolConfig_Validate_ValidConfiguration(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
	}

	err := config.Validate()

	assert.NoError(t, err)
}

func TestConnectionPoolConfig_Validate_NegativeMaxIdleConns(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxIdleConns:        -1,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
	}

	err := config.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MaxIdleConns cannot be negative")
}

func TestConnectionPoolConfig_Validate_NegativeMaxIdleConnsPerHost(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: -1,
		IdleConnTimeout:     30 * time.Second,
	}

	err := config.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "MaxIdleConnsPerHost cannot be negative")
}

func TestConnectionPoolConfig_Validate_NegativeIdleConnTimeout(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     -1 * time.Second,
	}

	err := config.Validate()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "IdleConnTimeout cannot be negative")
}

func TestConnectionPoolConfig_Validate_ZeroValuesAreValid(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxIdleConns:        0,
		MaxIdleConnsPerHost: 0,
		IdleConnTimeout:     0,
	}

	err := config.Validate()

	assert.NoError(t, err)
}

func TestConnectionPoolConfig_ClonesDefaultTransport(t *testing.T) {
	config := &ConnectionPoolConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
	}

	baseService := &httpService{
		Client: &http.Client{},
	}

	config.AddOption(baseService)

	transport, ok := baseService.Client.Transport.(*http.Transport)
	assert.True(t, ok, "Transport should be of type *http.Transport")

	defaultTransport := http.DefaultTransport.(*http.Transport)

	assert.Equal(t, defaultTransport.TLSHandshakeTimeout, transport.TLSHandshakeTimeout)
	assert.NotNil(t, transport.Proxy, "Proxy function should be set from default transport")
}
