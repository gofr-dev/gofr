package service

import (
	"context"
	"time"

	"gofr.dev/pkg/gofr/datasource/redis"
)

type HTTPCacher interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte) error
}

type Cache struct {
	redis *redis.Redis
	TTL   time.Duration
}

func NewCache(ttl time.Duration) HTTPCacher {
	c := &Cache{TTL: ttl}

	// need to initialize a redis Client or inject it

	return c
}

func (c *Cache) apply(h *httpService) {
	h.cache = c
}

func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	cmd := c.redis.Get(ctx, key)
	if cmd.Err() != nil {
		return nil, cmd.Err()
	}

	return cmd.Bytes()
}

func (c *Cache) Set(ctx context.Context, key string, value []byte) error {
	cmd := c.redis.Set(ctx, key, value, c.TTL)

	return cmd.Err()
}
