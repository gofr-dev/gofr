package factory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
		name     string
		ctx      context.Context
		cacheName string
		ttl      time.Duration
		maxItems int
		opts     []interface{}
		expErr   bool
	}{
		{
			name:     "Successful creation with no options",
			ctx:      context.Background(),
			cacheName: "test-inmemory",
			ttl:      5 * time.Minute,
			maxItems: 100,
			opts:     nil,
			expErr:   false,
		},
		{
			name:     "Successful creation with logger option",
			ctx:      context.Background(),
			cacheName: "test-inmemory-with-logger",
			ttl:      10 * time.Minute,
			maxItems: 50,
			opts:     []interface{}{observability.NewStdLogger()},
			expErr:   false,
		},
		{
			name:     "Successful creation with metrics option",
			ctx:      context.Background(),
			cacheName: "test-inmemory-with-metrics",
			ttl:      10 * time.Minute,
			maxItems: 50,
			opts:     []interface{}{observability.NewMetrics("test", "inmemory")},
			expErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewInMemoryCache(tt.ctx, tt.cacheName, tt.ttl, tt.maxItems, tt.opts...)

			if tt.expErr {
				assert.Error(t, err, "Expected an error for %v", tt.name)
			} else {
				assert.NoError(t, err, "Did not expect an error for %v", tt.name)
				assert.NotNil(t, c, "Expected a cache instance for %v", tt.name)
			}
		})
	}
}

func TestNewRedisCache(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		cacheName string
		ttl     time.Duration
		opts    []interface{}
		expErr  bool
	}{
		{
			name:    "Initialization without options",
			ctx:     context.Background(),
			cacheName: "test-redis",
			ttl:     5 * time.Minute,
			opts:    nil,
			expErr:  false,
		},
		{
			name:    "Initialization with address",
			ctx:     context.Background(),
			cacheName: "test-redis-with-addr",
			ttl:     10 * time.Minute,
			opts:    []interface{}{"localhost:6379"},
			expErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewRedisCache(tt.ctx, tt.cacheName, tt.ttl, tt.opts...)

			if tt.expErr {
				assert.Error(t, err, "Expected an error for %v", tt.name)
			} else {
				assert.NoError(t, err, "Did not expect an error for %v", tt.name)
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
			c, err := NewCache(context.Background(), tt.cacheType, "test-cache", 5*time.Minute, 100)

			if tt.expErr {
				assert.Error(t, err, "Expected an error for %v", tt.name)
			} else {
				assert.NoError(t, err, "Did not expect an error for %v", tt.name)
				assert.NotNil(t, c, "Expected a cache instance for %v", tt.name)
			}
		})
	}
}
