package gofr

import (
	"context"
	"time"

	"gofr.dev/pkg/cache"
	"gofr.dev/pkg/cache/factory"
)

// AddInMemoryCache adds an in-memory cache to the app's container.
func (a *App) AddInMemoryCache(ctx context.Context, name string, ttl time.Duration, maxItems int) {
	c, err := factory.NewInMemoryCache(
		ctx,
		name,
		factory.WithLogger(a.Logger()),
		factory.WithTTL(ttl),
		factory.WithMaxItems(maxItems),
	)
	if err != nil {
		a.Logger().Errorf("inmemory cache init failed: %v", err)
		return
	}

	a.container.AddCache(name, c)
}

// AddRedisCache adds a Redis cache to the app's container.
func (a *App) AddRedisCache(ctx context.Context, name string, ttl time.Duration, addr string) {
	c, err := factory.NewRedisCache(
		ctx,
		name,
		factory.WithLogger(a.Logger()),
		factory.WithTTL(ttl),
		factory.WithRedisAddr(addr),
	)
	if err != nil {
		a.Logger().Errorf("redis cache init failed: %v", err)
		return
	}

	a.container.AddCache(name, c)
}

// AddRedisCacheDirect adds a Redis cache with full configuration.
func (a *App) AddRedisCacheDirect(ctx context.Context, name, addr, password string, db int, ttl time.Duration) {
	c, err := factory.NewRedisCache(
		ctx,
		name,
		factory.WithLogger(a.Logger()),
		factory.WithTTL(ttl),
		factory.WithRedisAddr(addr),
		factory.WithRedisPassword(password),
		factory.WithRedisDB(db),
	)
	if err != nil {
		a.Logger().Errorf("redis cache init failed: %v", err)
		return
	}

	a.container.AddCache(name, c)
}

// GetCache retrieves a cache instance from the app's container by name.
func (a *App) GetCache(name string) cache.Cache {
	return a.container.GetCache(name)
}
