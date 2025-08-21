package inmemory

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"gofr.dev/pkg/cache"
	"gofr.dev/pkg/cache/observability"
)

// Common errors.
var (
	// ErrCacheClosed is returned when an operation is attempted on a closed cache.
	ErrCacheClosed = errors.New("cache is closed")
	// ErrEmptyKey is returned when an operation is attempted with an empty key.
	ErrEmptyKey = errors.New("key cannot be empty")
	// ErrNilValue is returned when a nil value is provided to Set.
	ErrNilValue = errors.New("value cannot be nil")
	// ErrInvalidMaxItems is returned when a negative value is provided for maxItems.
	ErrInvalidMaxItems = errors.New("maxItems must be non-negative")
)

// node represents an element in the LRU doubly linked list.
// Used for O(1) insert, remove, and move-to-front operations.
type node struct {
	key        string
	prev, next *node
}

type entry struct {
	value     any
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

	name    string
	logger  observability.Logger
	metrics *observability.Metrics
	tracer  *trace.Tracer
}

type Option func(*inMemoryCache) error

// WithTTL sets the default time-to-live (TTL) for all entries in the cache.
// Items will be automatically removed after this duration has passed since they were last set.
// A TTL of zero or less disables automatic expiration.
func WithTTL(ttl time.Duration) Option {
	return func(c *inMemoryCache) error {
		c.ttl = ttl
		return nil
	}
}

// WithMaxItems sets the maximum number of items the cache can hold.
// When this limit is reached, the least recently used (LRU) item is evicted
// to make space for a new one. A value of 0 means no limit.
func WithMaxItems(maxItems int) Option {
	return func(c *inMemoryCache) error {
		if maxItems < 0 {
			return ErrInvalidMaxItems
		}

		c.maxItems = maxItems

		return nil
	}
}

// WithName sets a descriptive name for the cache instance.
// This name is used in logs and metrics to identify the cache.
func WithName(name string) Option {
	return func(c *inMemoryCache) error {
		if name != "" {
			c.name = name
		}

		return nil
	}
}

// WithLogger provides a custom logger for the cache.
// If not provided, a default standard library logger is used.
func WithLogger(logger observability.Logger) Option {
	return func(c *inMemoryCache) error {
		if logger != nil {
			c.logger = logger
		}

		return nil
	}
}

// WithMetrics provides a metrics collector for the cache.
// If provided, the cache will record metrics for operations like hits, misses, and sets.
func WithMetrics(m *observability.Metrics) Option {
	return func(c *inMemoryCache) error {
		if m != nil {
			c.metrics = m
		}

		return nil
	}
}

// validateKey ensures key is non-empty.
func validateKey(key string) error {
	if key == "" {
		return ErrEmptyKey
	}

	return nil
}

// withSpan creates a span for cache operations and ensures proper context propagation.
func (c *inMemoryCache) withSpan(ctx context.Context, operation, key string) (context.Context, trace.Span) {
	if c.tracer == nil {
		return ctx, trace.SpanFromContext(ctx)
	}

	tracer := *c.tracer
	spanCtx, span := tracer.Start(ctx, fmt.Sprintf("cache.%s", operation),
		trace.WithAttributes(
			attribute.String("cache.name", c.name),
			attribute.String("cache.key", key),
			attribute.String("cache.operation", operation),
		))

	return spanCtx, span
}

// NewInMemoryCache creates and returns a new in-memory cache instance.
// It takes zero or more Option functions to customize its configuration.
// By default, it creates a cache with a 1-minute TTL and no item limit.
// It also starts a background goroutine for periodic cleanup of expired items.
func NewInMemoryCache(ctx context.Context, opts ...Option) (cache.Cache, error) {
	c := &inMemoryCache{
		items:    make(map[string]entry),
		ttl:      time.Minute,
		maxItems: 0,
		quit:     make(chan struct{}),
		logger:   observability.NewStdLogger(),
		metrics:  observability.NewMetrics("gofr", "inmemory_cache"),
	}

	for _, opt := range opts {
		if err := opt(c); err != nil {
			return nil, err
		}
	}

	c.logger.Infof(ctx, "Cache '%s' initialized with TTL=%s, MaxItems=%d", c.name, c.ttl, c.maxItems)

	// Start cleanup goroutine
	go c.startCleanup(ctx)

	return c, nil
}

func (c *inMemoryCache) UseTracer(tracer trace.Tracer) {
	c.tracer = &tracer
}

// Set adds or updates a key-value pair in the cache.
// If the key already exists, its value is updated, and it's marked as the most recently used item.
// If the cache is at capacity, the least recently used item is evicted.
// This operation is thread-safe.
func (c *inMemoryCache) Set(ctx context.Context, key string, value any) error {
	spanCtx, span := c.withSpan(ctx, "set", key)
	defer span.End()

	if err := validateKey(key); err != nil {
		c.logger.Errorf(spanCtx, "Set failed: %v", err)
		return err
	}

	if value == nil {
		c.logger.Errorf(spanCtx, "Set failed: %v", ErrNilValue)
		return ErrNilValue
	}

	now := time.Now()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Clean up expired entries (O(n) TTL scan)
	removed := c.cleanupExpired(now)
	if removed > 0 {
		c.logger.Debugf(spanCtx, "Cleaned %d expired items during Set", removed)
	}

	// If present, update value and move node to front (most recently used)
	if ent, ok := c.items[key]; ok {
		ent.value = value
		ent.expiresAt = c.computeExpiry(now)
		c.items[key] = ent
		// O(1) move-to-front
		c.moveToFront(ent.node)

		duration := time.Since(now)
		c.logger.LogRequest(spanCtx, "INFO", "SET", "UPDATE", duration, key)
		c.metrics.Sets().WithLabelValues(c.name).Inc()
		c.metrics.Items().WithLabelValues(c.name).Set(float64(len(c.items)))
		c.metrics.Latency().WithLabelValues(c.name, "set").Observe(duration.Seconds())

		return nil
	}

	// Evict if at capacity (O(1) eviction)
	if c.maxItems > 0 && len(c.items) >= c.maxItems {
		c.evictTail(spanCtx)
	}

	// Insert new node at head (most recently used)
	node := &node{key: key}
	// O(1) insert at front
	c.insertAtFront(node)
	c.items[key] = entry{value: value, expiresAt: c.computeExpiry(now), node: node}

	duration := time.Since(now)
	c.logger.LogRequest(spanCtx, "INFO", "SET", "CREATE", duration, key)
	c.metrics.Sets().WithLabelValues(c.name).Inc()
	c.metrics.Items().WithLabelValues(c.name).Set(float64(len(c.items)))
	c.metrics.Latency().WithLabelValues(c.name, "set").Observe(duration.Seconds())

	return nil
}

// Get retrieves the value for a given key.
// If the key is found and not expired, it returns the value and true.
// It also marks the accessed item as the most recently used.
// If the key is not found or has expired, it returns nil and false.
// This operation is thread-safe.
func (c *inMemoryCache) Get(ctx context.Context, key string) (value any, found bool, err error) {
	spanCtx, span := c.withSpan(ctx, "get", key)
	defer span.End()

	if err := validateKey(key); err != nil {
		c.logger.Errorf(spanCtx, "Get failed: %v", err)
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

		duration := time.Since(time.Now())
		c.logger.Missf(spanCtx, "GET", duration, key)
		c.metrics.Misses().WithLabelValues(c.name).Inc()
		c.metrics.Latency().WithLabelValues(c.name, "get").Observe(duration.Seconds())

		return nil, false, nil
	}

	// Hit: move node to front to mark as most recently used (O(1))
	start := time.Now()

	c.moveToFront(ent.node)

	duration := time.Since(start) // ✅ This now measures actual processing time

	c.logger.Hitf(spanCtx, "GET", duration, key)
	c.metrics.Hits().WithLabelValues(c.name).Inc()
	c.metrics.Latency().WithLabelValues(c.name, "get").Observe(duration.Seconds())

	return ent.value, true, nil
}

// Delete removes a key from the cache.
// If the key does not exist, the operation is a no-op.
// This operation is thread-safe.
func (c *inMemoryCache) Delete(ctx context.Context, key string) error {
	spanCtx, span := c.withSpan(ctx, "delete", key)
	defer span.End()

	if err := validateKey(key); err != nil {
		c.logger.Errorf(spanCtx, "Delete failed: %v", err)
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if ent, existed := c.items[key]; existed {
		// O(1) removal from LRU list
		c.removeNode(ent.node)
		delete(c.items, key)
		c.logger.Debugf(spanCtx, "Deleted key '%s'", key)
		c.metrics.Deletes().WithLabelValues(c.name).Inc()
		c.metrics.Items().WithLabelValues(c.name).Set(float64(len(c.items)))
	}

	c.metrics.Latency().WithLabelValues(c.name, "delete").Observe(time.Since(time.Now()).Seconds())

	return nil
}

// Exists checks if a key exists in the cache and has not expired.
// It returns true if the key is present and valid, false otherwise.
// This operation does not update the item's recency.
// This operation is thread-safe.
func (c *inMemoryCache) Exists(ctx context.Context, key string) (bool, error) {
	spanCtx, span := c.withSpan(ctx, "exists", key)
	defer span.End()

	if err := validateKey(key); err != nil {
		c.logger.Errorf(spanCtx, "Exists failed: %v", err)
		return false, err
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	if ent, ok := c.items[key]; ok && time.Now().Before(ent.expiresAt) {
		return true, nil
	}

	return false, nil
}

// Clear removes all items from the cache.
// This operation is thread-safe.
func (c *inMemoryCache) Clear(ctx context.Context) error {
	spanCtx, span := c.withSpan(ctx, "clear", "")
	defer span.End()

	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.items)
	c.items = make(map[string]entry)
	c.head, c.tail = nil, nil
	c.logger.Infof(spanCtx, "Cleared cache '%s', removed %d items", c.name, count)
	c.metrics.Items().WithLabelValues(c.name).Set(0)

	return nil
}

// Close stops the background cleanup goroutine and marks the cache as closed.
// Subsequent operations on the cache may fail. Calling Close on an already closed
// cache returns ErrCacheClosed.
// This operation is thread-safe.
func (c *inMemoryCache) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		c.logger.Warnf(ctx, "Close called on already closed cache '%s'", c.name)
		return ErrCacheClosed
	}

	close(c.quit)
	c.closed = true
	c.logger.Infof(ctx, "Cache '%s' closed", c.name)

	return nil
}

// returns expiration for now.
func (c *inMemoryCache) computeExpiry(now time.Time) time.Time {
	if c.ttl <= 0 {
		return now
	}

	return now.Add(c.ttl)
}

// unlinks expired entries.
func (c *inMemoryCache) cleanupExpired(now time.Time) int {
	var removed int

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

// removes the least-recently used item.
func (c *inMemoryCache) evictTail(ctx context.Context) {
	if c.tail == nil {
		return
	}

	key := c.tail.key
	c.removeNode(c.tail)
	delete(c.items, key)
	c.logger.Debugf(ctx, "Evicted key '%s'", key)
	c.metrics.Evicts().WithLabelValues(c.name).Inc()
	c.metrics.Items().WithLabelValues(c.name).Set(float64(len(c.items)))
}

// places n at the head.
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

// unlinks then inserts n at head.
func (c *inMemoryCache) moveToFront(n *node) {
	if c.head == n {
		return
	}

	c.removeNode(n)

	c.insertAtFront(n)
}

// unlinks n from the list.
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

// runs periodic TTL cleanup.
func (c *inMemoryCache) startCleanup(ctx context.Context) {
	interval := c.ttl / 4
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	c.logger.Infof(ctx, "Started cleanup every %v for cache '%s'", interval, c.name)

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			if !c.closed {
				removed := c.cleanupExpired(time.Now())
				if removed > 0 {
					c.logger.Debugf(ctx, "Cleanup removed %d items, remaining %d", removed, len(c.items))
				}
			}
			c.mu.Unlock()

		case <-ctx.Done():
			c.logger.Infof(ctx, "Context canceled: stopping cleanup for cache '%s'", c.name)
			return

		case <-c.quit:
			c.logger.Infof(ctx, "Quit channel closed: stopping cleanup for cache '%s'", c.name)
			return
		}
	}
}

const (
	DefaultTTL      = 5 * time.Minute
	DefaultMaxItems = 1000
	DebugMaxItems   = 100
)

// NewDefaultCache creates a cache with sensible default settings for general use.
// It is configured with a 5-minute TTL and a 1000-item limit.
func NewDefaultCache(ctx context.Context, name string) (cache.Cache, error) {
	return NewInMemoryCache(
		ctx,
		WithName(name),
		WithTTL(DefaultTTL),
		WithMaxItems(DefaultMaxItems),
	)
}

// NewDebugCache creates a cache with settings suitable for debugging.
// It has a short TTL (1 minute) and a small capacity (100 items).
func NewDebugCache(ctx context.Context, name string) (cache.Cache, error) {
	return NewInMemoryCache(
		ctx,
		WithName(name),
		WithTTL(1*time.Minute),
		WithMaxItems(DebugMaxItems),
	)
}

// NewProductionCache creates a cache with settings suitable for production environments.
// It requires explicit configuration for TTL and maximum item count.
func NewProductionCache(ctx context.Context, name string, ttl time.Duration, maxItems int) (cache.Cache, error) {
	return NewInMemoryCache(
		ctx,
		WithName(name),
		WithTTL(ttl),
		WithMaxItems(maxItems),
	)
}
