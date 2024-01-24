package service

import (
	"context"
	"net/http"
	"time"
)

type HTTPCacher interface {
	Get(ctx context.Context, key string) *http.Response
	Set(ctx context.Context, key string, value *http.Response)
}

type Cache struct {
	cacher map[string]*http.Response
	TTL    time.Duration
}

func (c *Cache) apply(h *httpService) {
	c.cacher = make(map[string]*http.Response)

	h.cache = c
}

func (c *Cache) Get(ctx context.Context, key string) *http.Response {
	v, ok := c.cacher[key]
	if !ok {
		return nil
	}

	return v
}

func (c *Cache) Set(ctx context.Context, key string, value *http.Response) {
	c.cacher[key] = value
}
