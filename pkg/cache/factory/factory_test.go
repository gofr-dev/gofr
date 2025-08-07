package factory

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/cache/observability"
)

func TestWithLogger(t *testing.T) {
	logger := observability.NewStdLogger()
	opt := WithLogger(logger)

	val := opt()

	assert.Equal(t, logger, val, "WithLogger should return the provided logger")
}

func TestNewInMemoryCache(t *testing.T) {
	tests := []struct {
		name      string
		cacheName string
		ttl       time.Duration
		maxItems  int
		getOpts   func() []any
		expErr    bool
	}{
		{
			name:      "Successful creation with no options",
			cacheName: "test-inmemory",
			ttl:       5 * time.Minute,
			maxItems:  100,
			getOpts:   func() []any { return nil },
			expErr:    false,
		},
		{
			name:      "Successful creation with logger option",
			cacheName: "test-inmemory-with-logger",
			ttl:       10 * time.Minute,
			maxItems:  50,
			getOpts:   func() []any { return []any{observability.NewStdLogger()} },
			expErr:    false,
		},
		{
			name:      "Successful creation with metrics option",
			cacheName: "test-inmemory-with-metrics",
			ttl:       10 * time.Minute,
			maxItems:  50,
			getOpts:   func() []any { return []any{observability.NewMetrics("test", "inmemory")} },
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

			opts := tt.getOpts()

			c, err := NewInMemoryCache(t.Context(), tt.cacheName, tt.ttl, tt.maxItems, opts...)

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
		opts      []any
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
			opts:      []any{"localhost:6379"},
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
		expErr    bool
	}{
		{
			name:      "Create inmemory cache",
			cacheType: "inmemory",
			expErr:    false,
		},
		{
			name:      "Create redis cache",
			cacheType: "redis",
			expErr:    false,
		},
		{
			name:      "Create default cache (inmemory)",
			cacheType: "unknown",
			expErr:    false,
		},
		{
			name:      "Create with empty type (default to inmemory)",
			cacheType: "",
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

			c, err := NewCache(t.Context(), tt.cacheType, "test-cache", 5*time.Minute, 100)

			if tt.expErr {
				assert.Error(t, err, "Expected an error for %v", tt.name)
			} else {
				require.NoError(t, err, "Did not expect an error for %v", tt.name)
				assert.NotNil(t, c, "Expected a cache instance for %v", tt.name)
			}
		})
	}
}
