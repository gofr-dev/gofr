// package factory provides factory functions for creating cache instances.
// It simplifies the creation of different cache types (e.g., in-memory, Redis)
// with a unified and configurable approach.
package factory

import (
	"context"
	"time"

	"gofr.dev/pkg/cache"
	"gofr.dev/pkg/cache/inmemory"
	"gofr.dev/pkg/cache/observability"
	"gofr.dev/pkg/cache/redis"
)

type Option func() interface{}

// WithLogger returns an Option that sets a custom logger for a cache.
// The provided logger must implement the observability.Logger interface.
func WithLogger(logger observability.Logger) Option {
	return func() interface{} {
		return logger
	}
}

// NewInMemoryCache creates a new in-memory cache instance with the specified configuration.
//
// Parameters:
//   - ctx: The context for initialization.
//   - name: A logical name for the cache, used in logging and metrics.
//   - ttl: The default time-to-live for cache entries.
//   - maxItems: The maximum number of items the cache can hold. LRU eviction is used if exceeded.
//   - opts: Optional configurations, such as a custom logger or metrics collector.
//
// Returns a cache.Cache instance or an error if initialization fails.
func NewInMemoryCache(ctx context.Context, name string, ttl time.Duration, maxItems int, opts ...interface{}) (cache.Cache, error) {
	var inMemoryOpts []inmemory.Option

	inMemoryOpts = append(inMemoryOpts, inmemory.WithName(name))
	inMemoryOpts = append(inMemoryOpts, inmemory.WithTTL(ttl))
	inMemoryOpts = append(inMemoryOpts, inmemory.WithMaxItems(maxItems))

	for _, opt := range opts {
		switch v := opt.(type) {
		case observability.Logger:
			inMemoryOpts = append(inMemoryOpts, inmemory.WithLogger(v))
		case *observability.Metrics:
			inMemoryOpts = append(inMemoryOpts, inmemory.WithMetrics(v))
		case Option:
			if logger, ok := v().(observability.Logger); ok {
				inMemoryOpts = append(inMemoryOpts, inmemory.WithLogger(logger))
			}
		}
	}

	return inmemory.NewInMemoryCache(inMemoryOpts...)
}

// NewRedisCache creates a new Redis-backed cache instance.
//
// Parameters:
//   - ctx: The context for initialization and connection verification.
//   - name: A logical name for the cache, used in logging and metrics.
//   - ttl: The default time-to-live for cache entries.
//   - opts: Optional configurations, such as a custom logger, metrics collector, or Redis connection details (address, password, DB).
//
// Returns a cache.Cache instance or an error if the connection to Redis fails.
func NewRedisCache(ctx context.Context, name string, ttl time.Duration, opts ...interface{}) (cache.Cache, error) {
	var redisOpts []redis.Option

	redisOpts = append(redisOpts, redis.WithName(name))
	redisOpts = append(redisOpts, redis.WithTTL(ttl))

	for _, opt := range opts {
		switch v := opt.(type) {
		case observability.Logger:
			redisOpts = append(redisOpts, redis.WithLogger(v))
		case *observability.Metrics:
			redisOpts = append(redisOpts, redis.WithMetrics(v))
		case string:
			redisOpts = append(redisOpts, redis.WithAddr(v))
		case Option:
			if logger, ok := v().(observability.Logger); ok {
				redisOpts = append(redisOpts, redis.WithLogger(logger))
			}
		}
	}

	return redis.NewRedisCache(ctx, redisOpts...)
}

// NewCache is a generic factory that creates a cache instance based on the specified type.
// It acts as a dispatcher to the more specific factory functions like NewInMemoryCache or NewRedisCache.
//
// Parameters:
//   - ctx: The context for initialization.
//   - cacheType: The type of cache to create ("inmemory" or "redis"). Defaults to "inmemory".
//   - name: A logical name for the cache.
//   - ttl: The default time-to-live for entries.
//   - maxItems: The maximum number of items (only applicable to in-memory cache).
//   - opts: Optional configurations passed to the underlying cache constructor.
//
// Returns a cache.Cache instance or an error if initialization fails.
func NewCache(ctx context.Context, cacheType string, name string, ttl time.Duration, maxItems int, opts ...interface{}) (cache.Cache, error) {
	switch cacheType {
	case "redis":
		return NewRedisCache(ctx, name, ttl, opts...)
	case "inmemory":
		return NewInMemoryCache(ctx, name, ttl, maxItems, opts...)
	default:
		return NewInMemoryCache(ctx, name, ttl, maxItems, opts...)
	}
}
