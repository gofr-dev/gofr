package gofr

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/cache"
	"gofr.dev/pkg/cache/factory"
	"gofr.dev/pkg/cache/redis"
)

func (a *App) AddInMemoryCache(ctx context.Context, name string, ttl time.Duration, maxItems int) {
	c, err := factory.NewInMemoryCache(
		ctx, name, ttl, maxItems,
	)
	if err != nil {
		a.Logger().Errorf("inmemory cache init failed: %v", err)
		return
	}

	tracer := otel.GetTracerProvider().Tracer("gofr-inmemory-cache")
	c.UseTracer(tracer)

	a.container.AddCache(name, c)
}

func (a *App) AddRedisCache(ctx context.Context, name string, ttl time.Duration, addr string) {
	c, err := factory.NewRedisCache(
		ctx, name, ttl,
		factory.WithRedisAddr(addr),
	)
	if err != nil {
		a.Logger().Errorf("redis cache init failed: %v", err)
		return
	}

	tracer := otel.GetTracerProvider().Tracer("gofr-redis-cache")
	c.UseTracer(tracer)

	a.container.AddCache(name, c)
}

func (a *App) AddRedisCacheDirect(ctx context.Context, name, addr, password string, db int, ttl time.Duration) {
	c, err := redis.NewRedisCache(
		ctx,
		redis.WithName(name),
		redis.WithAddr(addr),
		redis.WithPassword(password),
		redis.WithDB(db),
		redis.WithTTL(ttl),
	)
	if err != nil {
		a.Logger().Errorf("redis cache init failed: %v", err)
		return
	}

	tracer := otel.GetTracerProvider().Tracer("gofr-redis-cache")
	c.UseTracer(tracer)

	a.container.AddCache(name, c)
}

func (a *App) GetCache(name string) cache.Cache {
	return a.container.GetCache(name)
}
