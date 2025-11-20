package rbac

import (
	"net/http"
	"sync"
	"time"
)

// RoleCache caches role lookups to improve performance.
type RoleCache struct {
	cache      map[string]cachedRole
	mu         sync.RWMutex
	ttl        time.Duration
	cleanupCh  chan struct{}
	cleanupWg  sync.WaitGroup
}

type cachedRole struct {
	role      string
	expiresAt time.Time
}

// NewRoleCache creates a new role cache with the specified TTL.
func NewRoleCache(ttl time.Duration) *RoleCache {
	cache := &RoleCache{
		cache:     make(map[string]cachedRole),
		ttl:       ttl,
		cleanupCh: make(chan struct{}),
	}

	// Start cleanup goroutine only if TTL > 0
	if ttl > 0 {
		cache.cleanupWg.Add(1)
		go cache.cleanup()
	}

	return cache
}

// Get retrieves a cached role for the given key.
// Returns the role and true if found and not expired, false otherwise.
func (c *RoleCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, exists := c.cache[key]
	if !exists {
		return "", false
	}

	// Check if expired
	if time.Now().After(cached.expiresAt) {
		return "", false
	}

	return cached.role, true
}

// Set stores a role in the cache with TTL.
func (c *RoleCache) Set(key, role string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = cachedRole{
		role:      role,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Delete removes a key from the cache.
func (c *RoleCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.cache, key)
}

// Clear removes all entries from the cache.
func (c *RoleCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]cachedRole)
}

// cleanup periodically removes expired entries.
func (c *RoleCache) cleanup() {
	defer c.cleanupWg.Done()

	ticker := time.NewTicker(c.ttl / 2) // Cleanup twice as often as TTL
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for key, cached := range c.cache {
				if now.After(cached.expiresAt) {
					delete(c.cache, key)
				}
			}
			c.mu.Unlock()

		case <-c.cleanupCh:
			return
		}
	}
}

// Stop stops the cleanup goroutine.
func (c *RoleCache) Stop() {
	select {
	case <-c.cleanupCh:
		// Already closed
	default:
		close(c.cleanupCh)
	}
	c.cleanupWg.Wait()
}

// CacheKeyGenerator generates cache keys for role lookups.
type CacheKeyGenerator func(req *http.Request) string

// DefaultCacheKeyGenerator generates cache keys from user ID or API key.
func DefaultCacheKeyGenerator(req *http.Request) string {
	// Try to get user ID from various sources
	if userID := req.Header.Get("X-User-ID"); userID != "" {
		return "rbac:user:" + userID
	}

	if apiKey := req.Header.Get("X-Api-Key"); apiKey != "" {
		return "rbac:apikey:" + apiKey
	}

	// Fallback to remote address (less ideal)
	return "rbac:ip:" + req.RemoteAddr
}

