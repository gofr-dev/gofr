// package factory provides factory functions for creating cache instances.
// It simplifies the creation of different cache types (e.g., in-memory, Redis)
// with a unified and configurable approach using the functional options pattern.
package factory

import (
	"context"
	"time"

	"gofr.dev/pkg/cache"
	"gofr.dev/pkg/cache/inmemory"
	"gofr.dev/pkg/cache/observability"
	"gofr.dev/pkg/cache/redis"
)

// config holds the configuration for creating cache instances.
type config struct {
	inMemoryOptions []inmemory.Option
	redisOptions    []redis.Option
}

type Option func(*config)

// WithObservabilityLogger returns an Option that sets a custom logger for the cache.
func WithObservabilityLogger(logger observability.Logger) Option {
	return func(c *config) {
		c.inMemoryOptions = append(c.inMemoryOptions, inmemory.WithLogger(logger))
		c.redisOptions = append(c.redisOptions, redis.WithLogger(logger))
	}
}

// WithObservabilityLoggerFunc returns an Option that sets a logger using a provider function.
func WithObservabilityLoggerFunc(f func() observability.Logger) Option {
	return func(c *config) {
		logger := f()
		c.inMemoryOptions = append(c.inMemoryOptions, inmemory.WithLogger(logger))
		c.redisOptions = append(c.redisOptions, redis.WithLogger(logger))
	}
}

// WithMetrics returns an Option that sets a metrics collector for the cache.
func WithMetrics(metrics *observability.Metrics) Option {
	return func(c *config) {
		c.inMemoryOptions = append(c.inMemoryOptions, inmemory.WithMetrics(metrics))
		c.redisOptions = append(c.redisOptions, redis.WithMetrics(metrics))
	}
}

// WithRedisAddr returns an Option that sets the connection address for a Redis cache.
func WithRedisAddr(addr string) Option {
	return func(c *config) {
		c.redisOptions = append(c.redisOptions, redis.WithAddr(addr))
	}
}

// NewInMemoryCache creates a new in-memory cache instance with the specified configuration.
// Parameters:
//   - ctx: The context for initialization.
//   - name: A logical name for the cache, used in logging and metrics.
//   - ttl: The default time-to-live for cache entries.
//   - maxItems: The maximum number of items the cache can hold. LRU eviction is used if exceeded.
//   - opts: Optional configurations, such as a custom logger or metrics collector.
//
// Returns a cache.Cache instance or an error if initialization fails.
func NewInMemoryCache(ctx context.Context, name string, ttl time.Duration, maxItems int, opts ...Option) (cache.Cache, error) {
	cfg := &config{
		inMemoryOptions: []inmemory.Option{
			inmemory.WithName(name),
			inmemory.WithTTL(ttl),
			inmemory.WithMaxItems(maxItems),
		},
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return inmemory.NewInMemoryCache(ctx, cfg.inMemoryOptions...)
}

// NewRedisCache creates a new Redis-backed cache instance.
// Parameters:
//   - ctx: The context for initialization and connection verification.
//   - name: A logical name for the cache, used in logging and metrics.
//   - ttl: The default time-to-live for cache entries.
//   - opts: Optional configurations, such as a custom logger, metrics collector, or Redis connection details.
//
// Returns a cache.Cache instance or an error if the connection to Redis fails.
func NewRedisCache(ctx context.Context, name string, ttl time.Duration, opts ...Option) (cache.Cache, error) {
	cfg := &config{
		redisOptions: []redis.Option{
			redis.WithName(name),
			redis.WithTTL(ttl),
		},
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return redis.NewRedisCache(ctx, cfg.redisOptions...)
}

// NewCache is a generic factory that creates a cache instance based on the specified type.
// It acts as a dispatcher to the more specific factory functions like NewInMemoryCache or NewRedisCache.
// Parameters:
//   - ctx: The context for initialization.
//   - cacheType: The type of cache to create ("inmemory" or "redis"). Defaults to "inmemory".
//   - name: A logical name for the cache.
//   - ttl: The default time-to-live for entries.
//   - maxItems: The maximum number of items (only applicable to in-memory cache).
//   - opts: Optional configurations passed to the underlying cache constructor.
//
// Returns a cache.Cache instance or an error if initialization fails.
func NewCache(ctx context.Context, cacheType, name string, ttl time.Duration, maxItems int, opts ...Option) (cache.Cache, error) {
	switch cacheType {
	case "redis":
		return NewRedisCache(ctx, name, ttl, opts...)
	default: // "inmemory" is the default
		return NewInMemoryCache(ctx, name, ttl, maxItems, opts...)
	}
}
