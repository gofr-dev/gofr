package middleware

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiterStore abstracts the storage and cleanup for rate limiter buckets.
// This interface matches the one defined in pkg/gofr/service for consistency.
//
// Note: The config parameter in Allow() is provided for interface compatibility.
// Implementations may use a stored configuration and ignore this parameter.
type RateLimiterStore interface {
	Allow(ctx context.Context, key string, config RateLimiterConfig) (allowed bool, retryAfter time.Duration, err error)
	StartCleanup(ctx context.Context)
	StopCleanup()
}

// memoryRateLimiterStore implements RateLimiterStore using in-memory token buckets.
type memoryRateLimiterStore struct {
	limiters    sync.Map // map[string]*limiterEntry
	keyCount    int64    // atomic counter for tracking number of keys
	maxKeys     int64    // maximum allowed keys (0 = unlimited)
	stopCh      chan struct{}
	cleanupOnce sync.Once
	stopOnce    sync.Once
	config      RateLimiterConfig // Store config for consistency
}

type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess int64 // Unix timestamp for cleanup
}

// Default limits for delay calculation bounds checking.
const (
	minDelay       = time.Millisecond
	maxDelay       = time.Minute
	defaultMaxKeys = 100000
)

// NewMemoryRateLimiterStore creates a new in-memory rate limiter store.
// The config is stored to ensure consistent rate limiting for all keys.
func NewMemoryRateLimiterStore(config RateLimiterConfig) RateLimiterStore {
	maxKeys := config.MaxKeys
	if maxKeys == 0 {
		maxKeys = defaultMaxKeys // Default max keys to prevent memory exhaustion
	}

	return &memoryRateLimiterStore{
		config:  config,
		maxKeys: maxKeys,
	}
}

// Allow checks if a request should be allowed based on the rate limit.
func (m *memoryRateLimiterStore) Allow(_ context.Context, key string, _ RateLimiterConfig) (bool, time.Duration, error) {
	now := time.Now().Unix()

	// Use stored config for consistency across all keys
	cfg := m.config

	// Get or create limiter for this key
	// Fix 1: Check loaded flag to avoid unnecessary object creation when entry already exists
	val, loaded := m.limiters.LoadOrStore(key, &limiterEntry{
		limiter:    rate.NewLimiter(rate.Limit(cfg.RequestsPerSecond), cfg.Burst),
		lastAccess: now,
	})

	entry := val.(*limiterEntry)

	// If entry was loaded (already existed), update lastAccess atomically
	// If it was stored (new entry), lastAccess is already set correctly
	if loaded {
		atomic.StoreInt64(&entry.lastAccess, now)
	} else {
		// New entry was stored - increment key count
		// Fix 5: Track number of keys to prevent memory exhaustion
		newCount := atomic.AddInt64(&m.keyCount, 1)
		if m.maxKeys > 0 && newCount > m.maxKeys {
			// Exceeded max keys - remove the entry we just added and fail open
			m.limiters.Delete(key)
			atomic.AddInt64(&m.keyCount, -1)
			// Fail open to prevent service denial
			return true, 0, nil
		}
	}

	// Fix 3: Use only Reserve() instead of Allow() + Reserve() to avoid race conditions
	// Reserve() atomically checks and reserves a token, giving accurate delay information
	reservation := entry.limiter.Reserve()
	if !reservation.OK() {
		// Should not happen with valid config, but handle gracefully
		// Fix 4: Use bounds-checked delay calculation
		return false, m.calculateSafeDelay(cfg.RequestsPerSecond), nil
	}

	delay := reservation.Delay()
	if delay > 0 {
		// Request would need to wait - cancel reservation and return the delay
		reservation.Cancel()
		return false, delay, nil
	}

	// Request is allowed immediately (delay == 0)
	return true, 0, nil
}

// calculateSafeDelay calculates delay with bounds checking to prevent overflow or zero values.
// Fix 4: Ensures delay is always within reasonable bounds.
func (*memoryRateLimiterStore) calculateSafeDelay(requestsPerSecond float64) time.Duration {
	if requestsPerSecond <= 0 {
		return maxDelay
	}

	delay := time.Duration(float64(time.Second) / requestsPerSecond)

	if delay < minDelay {
		return minDelay
	}

	if delay > maxDelay {
		return maxDelay
	}

	return delay
}

// StartCleanup starts a background goroutine to clean up stale limiters.
// This method is safe to call multiple times - only one cleanup goroutine will be started.
func (m *memoryRateLimiterStore) StartCleanup(ctx context.Context) {
	m.cleanupOnce.Do(func() {
		m.stopCh = make(chan struct{})

		go func() {
			const cleanupInterval = 5 * time.Minute

			const staleThreshold = 10 * time.Minute

			ticker := time.NewTicker(cleanupInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					m.cleanup(staleThreshold)
				case <-m.stopCh:
					return
				case <-ctx.Done():
					return
				}
			}
		}()
	})
}

// StopCleanup stops the cleanup goroutine.
// This method is safe to call multiple times.
func (m *memoryRateLimiterStore) StopCleanup() {
	m.stopOnce.Do(func() {
		if m.stopCh != nil {
			close(m.stopCh)
		}
	})
}

// cleanup removes stale limiters that haven't been accessed recently.
func (m *memoryRateLimiterStore) cleanup(staleThreshold time.Duration) {
	cutoff := time.Now().Unix() - int64(staleThreshold.Seconds())

	m.limiters.Range(func(key, value any) bool {
		entry := value.(*limiterEntry)
		if atomic.LoadInt64(&entry.lastAccess) < cutoff {
			m.limiters.Delete(key)
			// Decrement key count when removing stale entries
			atomic.AddInt64(&m.keyCount, -1)
		}

		return true
	})
}
