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
		opts      []Option
		expErr    bool
	}{
		{
			name:      "Successful creation with no options",
			cacheName: "test-inmemory",
			opts:      nil,
			expErr:    false,
		},
		{
			name:      "Successful creation with TTL and max items",
			cacheName: "test-inmemory-with-config",
			opts:      []Option{WithTTL(5 * time.Minute), WithMaxItems(100)},
			expErr:    false,
		},
		{
			name:      "Successful creation with logger option",
			cacheName: "test-inmemory-with-logger",
			opts:      []Option{WithObservabilityLogger(observability.NewStdLogger()), WithTTL(10 * time.Minute), WithMaxItems(50)},
			expErr:    false,
		},
		{
			name:      "Successful creation with metrics option",
			cacheName: "test-inmemory-with-metrics",
			opts:      []Option{WithMetrics(observability.NewMetrics("test", "inmemory")), WithTTL(10 * time.Minute), WithMaxItems(50)},
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

			c, err := NewInMemoryCache(t.Context(), tt.cacheName, tt.opts...)

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
		opts      []Option
		expErr    bool
	}{
		{
			name:      "Initialization without options",
			cacheName: "test-redis",
			opts:      nil,
			expErr:    false,
		},
		{
			name:      "Initialization with address and TTL",
			cacheName: "test-redis-with-addr",
			opts:      []Option{WithRedisAddr("localhost:6379"), WithTTL(10 * time.Minute)},
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

			c, err := NewRedisCache(t.Context(), tt.cacheName, tt.opts...)

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
			opts:      []Option{WithTTL(5 * time.Minute), WithMaxItems(100)},
			expErr:    false,
		},
		{
			name:      "Create redis cache",
			cacheType: "redis",
			opts:      []Option{WithTTL(5 * time.Minute)},
			expErr:    false,
		},
		{
			name:      "Create default cache (inmemory)",
			cacheType: "unknown",
			opts:      []Option{WithTTL(5 * time.Minute), WithMaxItems(100)},
			expErr:    false,
		},
		{
			name:      "Create with empty type (default to inmemory)",
			cacheType: "",
			opts:      []Option{WithTTL(5 * time.Minute), WithMaxItems(100)},
			expErr:    false,
		},
		{
			name:      "Create redis cache with options",
			cacheType: "redis",
			opts:      []Option{WithRedisAddr("localhost:6379"), WithTTL(5 * time.Minute)},
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

			c, err := NewCache(t.Context(), tt.cacheType, "test-cache", tt.opts...)

			if tt.expErr {
				assert.Error(t, err, "Expected an error for %v", tt.name)
			} else {
				require.NoError(t, err, "Did not expect an error for %v", tt.name)
				assert.NotNil(t, c, "Expected a cache instance for %v", tt.name)
			}
		})
	}
}
