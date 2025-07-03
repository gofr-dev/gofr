package inmemory

import (
	"sync"
	"time"

	"gofr.dev/pkg/cache"
)

type entry struct {
	value 		interface{}
	expiresAt 	time.Time
	lastUsed 	time.Time
}

type inMemoryCache struct {
	mu 			sync.RWMutex
	items 		map[string]entry
	ttl 		time.Duration
	maxItems 	int
	quit 		chan struct{}
	closed 		bool
}

// Configure the cache
type Option func(*inMemoryCache)

// WithTTL sets the TTL for the cache
func WithTTL(ttl time.Duration) Option {
	return func(c *inMemoryCache) {
		c.ttl = ttl
	}
}

// WithMaxItems sets the maximum number of items in the cache
func WithMaxItems(maxItems int) Option {
	return func(c *inMemoryCache) {
		c.maxItems = maxItems
	}
}

func NewInMemoryCache(opts ...Option) cache.Cache {
	c := &inMemoryCache{
		items:    make(map[string]entry),
		ttl:      time.Minute,
		maxItems: 0,
		quit:     make(chan struct{}),
	}

	for _, opt := range opts {
		opt(c)
	}

	go c.startCleanup()
	return c
}

func (c *inMemoryCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	
	c.cleanupExpired(now)

	// If we're at capacity and adding a new key, evict LRU item
	if c.maxItems > 0 && len(c.items) >= c.maxItems {
		if _, exists := c.items[key]; !exists {
			// Need to evict one item to make room for new key
			c.evictLRU()
		}
	}

	c.items[key] = entry{
		value: value,
		expiresAt: now.Add(c.ttl),
		lastUsed: now,
	}
}

func (c *inMemoryCache) Get(key string) (interface{}, bool) {
	c.mu.Lock() // to update lastUsed
	defer c.mu.Unlock()
	
	e, found := c.items[key]
	if !found {
		return nil, false
	}
	
	// Check if expired
	if time.Now().After(e.expiresAt) {
		delete(c.items, key)
		return nil, false
	}

	// Update last used time
	e.lastUsed = time.Now()
	c.items[key] = e

	return e.value, true
}


func (c *inMemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

func (c *inMemoryCache) Exists(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	e, found := c.items[key]
	if !found {
		return false
	}

	// Check if expired
	if time.Now().After(e.expiresAt) {
		// Don't delete here to avoid lock upgrade issues
		// Let the cleanup goroutine handle it
		return false
	}

	return true
}

func (c *inMemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]entry)
}

// Close stops the cleanup goroutine
func (c *inMemoryCache) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.closed {
		close(c.quit)
		c.closed = true
	}
}

// cleanupExpiredItems removes expired items (must be called with write lock)
func (c *inMemoryCache) cleanupExpired(now time.Time) {
	for k, e := range c.items {
		if now.After(e.expiresAt) {
			delete(c.items, k)
		}
	}
}

// evictLRU removes the least recently used item (must be called with write lock)
func (c *inMemoryCache) evictLRU() {
	if len(c.items) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	first := true

	for k, e := range c.items {
		if first || e.lastUsed.Before(oldestTime) {
			oldestKey = k
			oldestTime = e.lastUsed
			first = false
		}
	}

	if oldestKey != "" {
		delete(c.items, oldestKey)
	}
}

// Periodically clean up expired items
func (c *inMemoryCache) startCleanup() {
	if c.ttl <= 0 {
		<-c.quit 
		return
	}

	ticker := time.NewTicker(c.ttl)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			c.mu.Lock()
			for k, e := range c.items {
				if now.After(e.expiresAt) {
					delete(c.items, k)
				}
			}
			c.mu.Unlock()
		case <-c.quit:
			return
		}
	}
}
