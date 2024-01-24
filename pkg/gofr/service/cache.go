package service

import (
	"context"
	"time"
)

type HTTPCacher interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error
	Del(ctx context.Context, key string)
}

type Cache struct {
	HTTPCacher
	ttl time.Duration
}

func NewCache(h HTTPCacher, ttl time.Duration) *Cache {
	return &Cache{h, ttl}
}

func (c *Cache) apply(h *httpService) {
	h.cache = c
}
