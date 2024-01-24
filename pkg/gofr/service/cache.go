package service

import (
	"context"
	"net/http"
	"sync"
	"time"
)

type HTTPCacher interface {
	Get(ctx context.Context, key string) *http.Response
	Set(ctx context.Context, key string, value *http.Response)
}

type cacheEntry struct {
	resp    *http.Response
	setTime int64
}

type TTLMap struct {
	entry map[string]cacheEntry
	m     sync.Mutex
}
type Cache struct {
	cacher *TTLMap
	TTL    time.Duration
}

func (c *Cache) apply(h *httpService) {
	cacher := &TTLMap{entry: make(map[string]cacheEntry)}

	go func() {
		ticker := time.NewTicker(time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case t := <-ticker.C:
				cacher.m.Lock()
				for k, v := range cacher.entry {
					if t.Unix()-v.setTime > int64(c.TTL.Seconds()) {
						delete(cacher.entry, k)
					}
				}
				cacher.m.Unlock()
			}
		}

	}()

	c.cacher = cacher
	h.cache = c
}

func (c *Cache) Get(ctx context.Context, key string) *http.Response {
	c.cacher.m.Lock()
	v, ok := c.cacher.entry[key]
	c.cacher.m.Unlock()
	if !ok {
		return nil
	}

	return v.resp
}

func (c *Cache) Set(ctx context.Context, key string, value *http.Response) {
	c.cacher.m.Lock()
	c.cacher.entry[key] = cacheEntry{
		resp:    value,
		setTime: time.Now().Unix(),
	}
	c.cacher.m.Unlock()
}
