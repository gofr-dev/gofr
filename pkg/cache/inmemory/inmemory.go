package inmemory

import (
	"sync"
	"time"

	"gofr.dev/pkg/cache"
)

type inMemoryCache struct {
	sync.RWMutex
	items map[string]interface{}
}

func NewInMemoryCache(ttl time.Duration) cache.Cache {
	return &inMemoryCache{
		items: make(map[string]interface{}),
	}
}

// TODO: implement helpers
func (c *inMemoryCache) Get(key string) (interface{}, bool) {
	c.RLock()
	defer c.RUnlock()

	value, found := c.items[key]
	return value, found
}

func (c *inMemoryCache) Set(key string, value interface{}) {
	c.Lock()
	defer c.Unlock()

	c.items[key] = value
}

func (c *inMemoryCache) Delete(key string) {
	c.Lock()
	defer c.Unlock()

	delete(c.items, key)
}

func (c *inMemoryCache) Exists(key string) bool {
	c.RLock()
	defer c.RUnlock()

	_, found := c.items[key]
	return found
}

func (c *inMemoryCache) Clear() {
	c.Lock()
	defer c.Unlock()

	c.items = make(map[string]interface{})
}
