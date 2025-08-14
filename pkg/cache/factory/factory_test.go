package factory

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/cache/observability"
)

func TestNewInMemoryCache(t *testing.T) {
	tests := []struct {
		name      string
		cacheName string
		ttl       time.Duration
		maxItems  int
		opts      []Option
		expErr    bool
	}{
		{
			name:      "Successful creation with no options",
			cacheName: "test-inmemory",
			ttl:       5 * time.Minute,
			maxItems:  100,
			opts:      nil,
			expErr:    false,
		},
		{
			name:      "Successful creation with logger option",
			cacheName: "test-inmemory-with-logger",
			ttl:       10 * time.Minute,
			maxItems:  50,
			opts:      []Option{WithObservabilityLogger(observability.NewStdLogger())},
			expErr:    false,
		},
		{
			name:      "Successful creation with metrics option",
			cacheName: "test-inmemory-with-metrics",
			ttl:       10 * time.Minute,
			maxItems:  50,
			opts:      []Option{WithMetrics(observability.NewMetrics("test", "inmemory"))},
			expErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalRegistry := prometheus.DefaultRegisterer
			prometheus.DefaultRegisterer = prometheus.NewRegistry()
			t.Cleanup(func() {
				prometheus.DefaultRegisterer = originalRegistry
			})

			c, err := NewInMemoryCache(t.Context(), tt.cacheName, tt.ttl, tt.maxItems, tt.opts...)

			if tt.expErr {
				assert.Error(t, err, "Expected an error for %v", tt.name)
			} else {
				require.NoError(t, err, "Did not expect an error for %v", tt.name)
				assert.NotNil(t, c, "Expected a cache instance for %v", tt.name)
			}
		})
	}
}

func TestNewRedisCache(t *testing.T) {
	tests := []struct {
		name      string
		cacheName string
		ttl       time.Duration
		opts      []Option
		expErr    bool
	}{
		{
			name:      "Initialization without options",
			cacheName: "test-redis",
			ttl:       5 * time.Minute,
			opts:      nil,
			expErr:    false,
		},
		{
			name:      "Initialization with address",
			cacheName: "test-redis-with-addr",
			ttl:       10 * time.Minute,
			opts:      []Option{WithRedisAddr("localhost:6379")},
			expErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalRegistry := prometheus.DefaultRegisterer
			prometheus.DefaultRegisterer = prometheus.NewRegistry()
			t.Cleanup(func() {
				prometheus.DefaultRegisterer = originalRegistry
			})

			c, err := NewRedisCache(t.Context(), tt.cacheName, tt.ttl, tt.opts...)

			if tt.expErr {
				assert.Error(t, err, "Expected an error for %v", tt.name)
			} else {
				require.NoError(t, err, "Did not expect an error for %v", tt.name)
				assert.NotNil(t, c, "Expected a cache instance for %v", tt.name)
			}
		})
	}
}

func TestNewCache(t *testing.T) {
	tests := []struct {
		name      string
		cacheType string
		opts      []Option
		expErr    bool
	}{
		{
			name:      "Create inmemory cache",
			cacheType: "inmemory",
			opts:      nil,
			expErr:    false,
		},
		{
			name:      "Create redis cache",
			cacheType: "redis",
			opts:      nil,
			expErr:    false,
		},
		{
			name:      "Create default cache (inmemory)",
			cacheType: "unknown",
			opts:      nil,
			expErr:    false,
		},
		{
			name:      "Create with empty type (default to inmemory)",
			cacheType: "",
			opts:      nil,
			expErr:    false,
		},
		{
			name:      "Create redis cache with options",
			cacheType: "redis",
			opts:      []Option{WithRedisAddr("localhost:6379")},
			expErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalRegistry := prometheus.DefaultRegisterer
			prometheus.DefaultRegisterer = prometheus.NewRegistry()
			t.Cleanup(func() {
				prometheus.DefaultRegisterer = originalRegistry
			})

			c, err := NewCache(t.Context(), tt.cacheType, "test-cache", 5*time.Minute, 100, tt.opts...)

			if tt.expErr {
				assert.Error(t, err, "Expected an error for %v", tt.name)
			} else {
				require.NoError(t, err, "Did not expect an error for %v", tt.name)
				assert.NotNil(t, c, "Expected a cache instance for %v", tt.name)
			}
		})
	}
}
