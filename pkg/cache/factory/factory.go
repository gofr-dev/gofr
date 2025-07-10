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

func WithLogger(logger observability.Logger) Option {
	return func() interface{} {
		return logger
	}
}

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
