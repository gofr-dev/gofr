package service

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestConnectionPoolConfig_AddOption(t *testing.T) {
	tests := []struct {
		name   string
		config *ConnectionPoolConfig
		want   *http.Transport
	}{
		{
			name: "custom connection pool settings",
			config: &ConnectionPoolConfig{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
			},
			want: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     30 * time.Second,
			},
		},
		{
			name: "zero values",
			config: &ConnectionPoolConfig{
				MaxIdleConns:        0,
				MaxIdleConnsPerHost: 0,
				IdleConnTimeout:     0,
			},
			want: &http.Transport{
				MaxIdleConns:        0,
				MaxIdleConnsPerHost: 0,
				IdleConnTimeout:     0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock HTTP service
			mockHTTPService := &httpService{
				Client: &http.Client{},
			}

			// Apply the connection pool configuration
			result := tt.config.AddOption(mockHTTPService)

			// Verify the result is still the same service
			assert.Equal(t, mockHTTPService, result)

			// Verify the transport was configured correctly
			transport, ok := mockHTTPService.Client.Transport.(*http.Transport)
			assert.True(t, ok, "Transport should be of type *http.Transport")
			assert.Equal(t, tt.want.MaxIdleConns, transport.MaxIdleConns)
			assert.Equal(t, tt.want.MaxIdleConnsPerHost, transport.MaxIdleConnsPerHost)
			assert.Equal(t, tt.want.IdleConnTimeout, transport.IdleConnTimeout)
		})
	}
}

func TestConnectionPoolConfig_AddOption_NonHTTPService(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	config := &ConnectionPoolConfig{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     30 * time.Second,
	}

	// Create a mock service that's not an httpService
	mockService := NewMockHTTP(ctrl)

	// Apply the configuration
	result := config.AddOption(mockService)

	// Should return the same service unchanged
	assert.Equal(t, mockService, result)
}