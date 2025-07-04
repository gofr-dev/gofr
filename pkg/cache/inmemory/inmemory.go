package inmemory

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"gofr.dev/pkg/cache"
)

// Common errors
var (
	ErrCacheClosed     = errors.New("cache is closed")
	ErrEmptyKey        = errors.New("key cannot be empty")
	ErrNilValue        = errors.New("value cannot be nil")
	ErrInvalidMaxItems = errors.New("maxItems must be non-negative")
)

// defines the logging level
type LogLevel int

const (
	LogLevelSilent LogLevel = iota
	LogLevelError
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

func (l LogLevel) String() string {
	switch l {
	case LogLevelSilent:
		return "SILENT"
	case LogLevelError:
		return "ERROR"
	case LogLevelWarn:
		return "WARN"
	case LogLevelInfo:
		return "INFO"
	case LogLevelDebug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

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

type cacheStats struct {
	hits        int64
	misses      int64
	sets        int64
	deletes     int64
	evictions   int64
	cleanupRuns int64
	errors      int64
}


type inMemoryCache struct {
	mu       sync.RWMutex
	items    map[string]entry
	ttl      time.Duration
	maxItems int64
	quit     chan struct{}
	closed   bool

	// LRU list head and tail
	head, tail *node

	name     string
	logLevel LogLevel
	stats    cacheStats
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
func WithMaxItems(maxItems int64) Option {
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

// WithLogLevel sets the logging level
func WithLogLevel(level LogLevel) Option {
	return func(c *inMemoryCache) error {
		c.logLevel = level
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
		logLevel: LogLevelInfo,
	}
	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, fmt.Errorf("failed to configure cache: %w", err)
		}
	}
	c.logInfo("Cache '%s' initialized with TTL=%v, MaxItems=%d", c.name, c.ttl, c.maxItems)

	// Start periodic cleanup if TTL > 0
	if c.ttl > 0 {
		go c.startCleanup()
	} else {
		c.logWarn("TTL disabled; items will not expire automatically")
	}
	return c, nil
}

// Set inserts or updates a cache entry and marks it as most recently used
func (c *inMemoryCache) Set(key string, value interface{}) error {
	if err := c.validateKey(key); err != nil {
		c.logError("Set failed: %v", err)
		c.stats.errors++
		return err
	}
	if value == nil {
		c.logError("Set failed: %v", ErrNilValue)
		c.stats.errors++
		return ErrNilValue
	}

	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()

	// Clean up expired entries (O(n) TTL scan)
	removed := c.cleanupExpired(now)
	if removed > 0 {
		c.logDebug("Cleaned %d expired items during Set", removed)
	}

	// If present, update value and move node to front (most recently used)
	if ent, ok := c.items[key]; ok {
		ent.value = value
		ent.expiresAt = c.computeExpiry(now)
		c.items[key] = ent
		// O(1) move-to-front
		c.moveToFront(ent.node)
		c.stats.sets++
		return nil
	}

	// Evict if at capacity (O(1) eviction)
	if c.maxItems > 0 && int64(len(c.items)) >= c.maxItems {
		c.evictTail()
	}

	// Insert new node at head (most recently used)
	nd := &node{key: key}
	// O(1) insert at front
	c.insertAtFront(nd)
	c.items[key] = entry{value: value, expiresAt: c.computeExpiry(now), node: nd}
	c.stats.sets++
	c.logDebug("Set key '%s'", key)
	return nil
}

// Get retrieves a cache entry and updates its recency
func (c *inMemoryCache) Get(key string) (interface{}, bool, error) {
	if err := c.validateKey(key); err != nil {
		c.logError("Get failed: %v", err)
		c.stats.errors++
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
		c.stats.misses++
		c.logDebug("Cache miss for key '%s'", key)
		return nil, false, nil
	}

	// Hit: move node to front to mark as most recently used (O(1))
	c.moveToFront(ent.node)
	c.stats.hits++
	c.logDebug("Cache hit for key '%s'", key)
	return ent.value, true, nil
}

func (c *inMemoryCache) Delete(key string) error {
	if err := c.validateKey(key); err != nil {
		c.logError("Delete failed: %v", err)
		c.stats.errors++
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if ent, existed := c.items[key]; existed {
		// O(1) removal from LRU list
		c.removeNode(ent.node)
		delete(c.items, key)
		c.stats.deletes++
		c.logDebug("Deleted key '%s'", key)
	}
	return nil
}

func (c *inMemoryCache) Exists(key string) (bool, error) {
	if err := c.validateKey(key); err != nil {
		c.logError("Exists failed: %v", err)
		c.stats.errors++
		return false, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	if ent, ok := c.items[key]; ok && time.Now().Before(ent.expiresAt) {
		return true, nil
	}
	return false, nil
}

func (c *inMemoryCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.items)
	c.items = make(map[string]entry)
	c.head, c.tail = nil, nil
	c.stats.deletes += int64(count)
	c.logInfo("Cleared cache '%s', removed %d items", c.name, count)
	return nil
}

func (c *inMemoryCache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		c.logWarn("Close called on already closed cache '%s'", c.name)
		return ErrCacheClosed
	}
	close(c.quit)
	c.closed = true
	c.logInfo("Cache '%s' closed", c.name)
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
	if removed > 0 {
		c.stats.cleanupRuns++
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
	c.stats.evictions++
	c.logDebug("Evicted key '%s'", key)
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

	c.logInfo("Started cleanup every %v for cache '%s'", interval, c.name)
	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			if !c.closed {
				removed := c.cleanupExpired(time.Now())
				if removed > 0 {
					c.logDebug("Cleanup removed %d items, remaining %d", removed, len(c.items))
				}
			}
			c.mu.Unlock()
		case <-c.quit:
			c.logInfo("Stopping cleanup for cache '%s'", c.name)
			return
		}
	}
}

// Logging helpers
func (c *inMemoryCache) logError(fmtStr string, args ...interface{}) {
	if c.logLevel >= LogLevelError {
		log.Printf("[CACHE:%s] ERROR: "+fmtStr, append([]interface{}{c.name}, args...)...)
	}
}
func (c *inMemoryCache) logWarn(fmtStr string, args ...interface{}) {
	if c.logLevel >= LogLevelWarn {
		log.Printf("[CACHE:%s] WARN: "+fmtStr, append([]interface{}{c.name}, args...)...)
	}
}
func (c *inMemoryCache) logInfo(fmtStr string, args ...interface{}) {
	if c.logLevel >= LogLevelInfo {
		log.Printf("[CACHE:%s] INFO: "+fmtStr, append([]interface{}{c.name}, args...)...)
	}
}
func (c *inMemoryCache) logDebug(fmtStr string, args ...interface{}) {
	if c.logLevel >= LogLevelDebug {
		log.Printf("[CACHE:%s] DEBUG: "+fmtStr, append([]interface{}{c.name}, args...)...)
	}
}

// Convenience constructors
func NewDefaultCache() (cache.Cache, error) {
	return NewInMemoryCache(
		WithName("default"),
		WithTTL(5*time.Minute),
		WithMaxItems(1000),
		WithLogLevel(LogLevelInfo),
	)
}

func NewDebugCache(name string) (cache.Cache, error) {
	return NewInMemoryCache(
		WithName(name),
		WithTTL(1*time.Minute),
		WithMaxItems(100),
		WithLogLevel(LogLevelDebug),
	)
}

func NewProductionCache(name string, ttl time.Duration, maxItems int64) (cache.Cache, error) {
	return NewInMemoryCache(
		WithName(name),
		WithTTL(ttl),
		WithMaxItems(maxItems),
		WithLogLevel(LogLevelWarn),
	)
}
