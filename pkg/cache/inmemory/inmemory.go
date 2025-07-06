/*
TODO:
look for tracing and metrics (prome...)
make changes reqd.
priority: dev experiecne
docs about usage
*/

package inmemory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"gofr.dev/pkg/cache"
	"gofr.dev/pkg/cache/observability"
)

// Common errors
var (
	ErrCacheClosed     = errors.New("cache is closed")
	ErrEmptyKey        = errors.New("key cannot be empty")
	ErrNilValue        = errors.New("value cannot be nil")
	ErrInvalidMaxItems = errors.New("maxItems must be non-negative")
)

// node represents an element in the LRU doubly linked list
// Used for O(1) insert, remove, and move-to-front operations
type node struct {
	key        string
	prev, next *node
}

// entry holds the cache value, expiry, and its node in the LRU list
type entry struct {
	value     interface{}
	expiresAt time.Time
	node      *node
}

type inMemoryCache struct {
	mu       sync.RWMutex
	items    map[string]entry
	ttl      time.Duration
	maxItems int
	quit     chan struct{}
	closed   bool

	// LRU list head and tail
	head, tail *node

	name   string
	logger observability.Logger
}

// Option configures the cache
type Option func(*inMemoryCache) error

// WithTTL sets the TTL for the cache
func WithTTL(ttl time.Duration) Option {
	return func(c *inMemoryCache) error {
		c.ttl = ttl
		return nil
	}
}

// WithMaxItems sets the maximum number of items in the cache
func WithMaxItems(maxItems int) Option {
	return func(c *inMemoryCache) error {
		if maxItems < 0 {
			return ErrInvalidMaxItems
		}
		c.maxItems = maxItems
		return nil
	}
}

// WithName sets a friendly name for the cache instance
func WithName(name string) Option {
	return func(c *inMemoryCache) error {
		if name != "" {
			c.name = name
		}
		return nil
	}
}

// WithLogger sets the logger for the cache
func WithLogger(logger observability.Logger) Option {
	return func(c *inMemoryCache) error {
		if logger != nil {
			c.logger = logger
		}
		return nil
	}
}

// validateKey ensures key is non-empty
func (c *inMemoryCache) validateKey(key string) error {
	if key == "" {
		return ErrEmptyKey
	}
	return nil
}

// NewInMemoryCache creates a new cache instance
func NewInMemoryCache(opts ...Option) (cache.Cache, error) {
	c := &inMemoryCache{
		items:    make(map[string]entry),
		ttl:      time.Minute,
		maxItems: 0,
		quit:     make(chan struct{}),
		name:     "default",
		logger:   observability.NewStdLogger(),
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("failed to configure cache: %w", err)
		}
	}
	c.logger.Infof("Cache '%s' initialized with TTL=%v, MaxItems=%d", c.name, c.ttl, c.maxItems)

	// Start periodic cleanup if TTL > 0
	if c.ttl > 0 {
		go c.startCleanup()
	} else {
		c.logger.Warnf("TTL disabled; items will not expire automatically")
	}
	return c, nil
}

// Set inserts or updates a cache entry and marks it as most recently used
func (c *inMemoryCache) Set(ctx context.Context, key string, value interface{}) error {
	if err := c.validateKey(key); err != nil {
		c.logger.Errorf("Set failed: %v", err)
		return err
	}
	if value == nil {
		c.logger.Errorf("Set failed: %v", ErrNilValue)
		return ErrNilValue
	}

	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clean up expired entries (O(n) TTL scan)
	removed := c.cleanupExpired(now)
	if removed > 0 {
		c.logger.Debugf("Cleaned %d expired items during Set", removed)
	}

	// If present, update value and move node to front (most recently used)
	if ent, ok := c.items[key]; ok {
		ent.value = value
		ent.expiresAt = c.computeExpiry(now)
		c.items[key] = ent
		// O(1) move-to-front
		c.moveToFront(ent.node)
		return nil
	}

	// Evict if at capacity (O(1) eviction)
	if c.maxItems > 0 && int(len(c.items)) >= c.maxItems {
		c.evictTail()
	}

	// Insert new node at head (most recently used)
	nd := &node{key: key}
	// O(1) insert at front
	c.insertAtFront(nd)
	c.items[key] = entry{value: value, expiresAt: c.computeExpiry(now), node: nd}
	c.logger.Debugf("Set key '%s'", key)
	return nil
}

// Get retrieves a cache entry and updates its recency
func (c *inMemoryCache) Get(ctx context.Context, key string) (interface{}, bool, error) {
	if err := c.validateKey(key); err != nil {
		c.logger.Errorf("Get failed: %v", err)
		return nil, false, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	ent, ok := c.items[key]
	if !ok || time.Now().After(ent.expiresAt) {
		if ok {
			// Remove expired node (O(1))
			c.removeNode(ent.node)
			delete(c.items, key)
		}
		c.logger.Debugf("Cache miss for key '%s'", key)
		return nil, false, nil
	}

	// Hit: move node to front to mark as most recently used (O(1))
	c.moveToFront(ent.node)
	c.logger.Debugf("Cache hit for key '%s'", key)
	return ent.value, true, nil
}

func (c *inMemoryCache) Delete(ctx context.Context, key string) error {
	if err := c.validateKey(key); err != nil {
		c.logger.Errorf("Delete failed: %v", err)
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if ent, existed := c.items[key]; existed {
		// O(1) removal from LRU list
		c.removeNode(ent.node)
		delete(c.items, key)
		c.logger.Debugf("Deleted key '%s'", key)
	}
	return nil
}

func (c *inMemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	if err := c.validateKey(key); err != nil {
		c.logger.Errorf("Exists failed: %v", err)
		return false, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	if ent, ok := c.items[key]; ok && time.Now().Before(ent.expiresAt) {
		return true, nil
	}
	return false, nil
}

func (c *inMemoryCache) Clear(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.items)
	c.items = make(map[string]entry)
	c.head, c.tail = nil, nil
	c.logger.Infof("Cleared cache '%s', removed %d items", c.name, count)
	return nil
}

func (c *inMemoryCache) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		c.logger.Warnf("Close called on already closed cache '%s'", c.name)
		return ErrCacheClosed
	}
	close(c.quit)
	c.closed = true
	c.logger.Infof("Cache '%s' closed", c.name)
	return nil
}

// returns expiration for now
func (c *inMemoryCache) computeExpiry(now time.Time) time.Time {
	if c.ttl <= 0 {
		return now
	}
	return now.Add(c.ttl)
}

// unlinks expired entries
// (TTL cleanup remains O(n) over map)
func (c *inMemoryCache) cleanupExpired(now time.Time) int {
	removed := 0
	for k, ent := range c.items {
		if now.After(ent.expiresAt) {
			// O(1) remove from LRU list
			c.removeNode(ent.node)
			delete(c.items, k)
			removed++
		}
	}
	return removed
}

// removes the least-recently used item
// O(1) eviction by tail pointer
func (c *inMemoryCache) evictTail() {
	if c.tail == nil {
		return
	}
	key := c.tail.key
	// O(1) remove from list
	c.removeNode(c.tail)
	delete(c.items, key)
	c.logger.Debugf("Evicted key '%s'", key)
}

// places n at the head
// O(1) operation
func (c *inMemoryCache) insertAtFront(n *node) {
	n.prev = nil
	n.next = c.head
	if c.head != nil {
		c.head.prev = n
	}
	c.head = n
	if c.tail == nil {
		c.tail = n
	}
}

// unlinks then inserts n at head
// O(1) remove + O(1) insert = O(1)
func (c *inMemoryCache) moveToFront(n *node) {
	if c.head == n {
		return
	}
	// O(1) unlink
	c.removeNode(n)
	// O(1) insert
	c.insertAtFront(n)
}

// unlinks n from the list
// O(1) operation
func (c *inMemoryCache) removeNode(n *node) {
	if n.prev != nil {
		n.prev.next = n.next
	} else {
		c.head = n.next
	}
	if n.next != nil {
		n.next.prev = n.prev
	} else {
		c.tail = n.prev
	}
	n.prev, n.next = nil, nil
}

// runs periodic TTL cleanup
func (c *inMemoryCache) startCleanup() {
	interval := c.ttl / 4
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	c.logger.Infof("Started cleanup every %v for cache '%s'", interval, c.name)
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			if !c.closed {
				removed := c.cleanupExpired(time.Now())
				if removed > 0 {
					c.logger.Debugf("Cleanup removed %d items, remaining %d", removed, len(c.items))
				}
			}
			c.mu.Unlock()
		case <-c.quit:
			c.logger.Infof("Stopping cleanup for cache '%s'", c.name)
			return
		}
	}
}

// Convenience constructors
func NewDefaultCache() (cache.Cache, error) {
	return NewInMemoryCache(
		WithName("default"),
		WithTTL(5*time.Minute),
		WithMaxItems(1000),
		WithLogger(observability.NewStdLogger()),
	)
}

func NewDebugCache(name string) (cache.Cache, error) {
	return NewInMemoryCache(
		WithName(name),
		WithTTL(1*time.Minute),
		WithMaxItems(100),
		WithLogger(observability.NewStdLogger()),
	)
}

func NewProductionCache(name string, ttl time.Duration, maxItems int) (cache.Cache, error) {
	return NewInMemoryCache(
		WithName(name),
		WithTTL(ttl),
		WithMaxItems(maxItems),
		WithLogger(observability.NewStdLogger()),
	)
}
