package dbresolver

import (
	"sync"
	"sync/atomic"
)

// queryCache implements a thread-safe, size-limited cache for query classification.
type queryCache struct {
	cache sync.Map
	size  atomic.Int64
	max   int64
}

func newQueryCache(maxSize int64) *queryCache {
	return &queryCache{
		max: maxSize,
	}
}

func (c *queryCache) get(key string) (value, exists bool) {
	val, found := c.cache.Load(key)
	if !found {
		return false, false
	}

	boolVal, ok := val.(bool)
	if !ok {
		return false, false
	}

	return boolVal, true
}

func (c *queryCache) set(key string, value bool) {
	// Simple bounded cache - reject if full
	if c.size.Load() >= c.max {
		return
	}

	// Only increment size if it's a new key.
	if _, loaded := c.cache.LoadOrStore(key, value); !loaded {
		c.size.Add(1)
	}
}
