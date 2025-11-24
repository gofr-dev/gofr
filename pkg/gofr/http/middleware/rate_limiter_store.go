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
type RateLimiterStore interface {
	Allow(ctx context.Context, key string, config RateLimiterConfig) (allowed bool, retryAfter time.Duration, err error)
	StartCleanup(ctx context.Context)
	StopCleanup()
}

// memoryRateLimiterStore implements RateLimiterStore using in-memory token buckets.
type memoryRateLimiterStore struct {
	limiters sync.Map // map[string]*limiterEntry
	stopCh   chan struct{}
}

type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess int64 // Unix timestamp for cleanup
}

// NewMemoryRateLimiterStore creates a new in-memory rate limiter store.
func NewMemoryRateLimiterStore() RateLimiterStore {
	return &memoryRateLimiterStore{}
}

// Allow checks if a request should be allowed based on the rate limit.
func (m *memoryRateLimiterStore) Allow(_ context.Context, key string, config RateLimiterConfig) (bool, time.Duration, error) {
	now := time.Now().Unix()

	// Get or create limiter for this key
	val, _ := m.limiters.LoadOrStore(key, &limiterEntry{
		limiter:    rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.Burst),
		lastAccess: now,
	})

	entry := val.(*limiterEntry)
	atomic.StoreInt64(&entry.lastAccess, now)

	// Check if request is allowed
	if !entry.limiter.Allow() {
		// Calculate retry-after duration
		reservation := entry.limiter.Reserve()
		if !reservation.OK() {
			return false, time.Second, nil
		}

		delay := reservation.Delay()
		reservation.Cancel() // Don't actually consume the token

		return false, delay, nil
	}

	return true, 0, nil
}

// StartCleanup starts a background goroutine to clean up stale limiters.
func (m *memoryRateLimiterStore) StartCleanup(ctx context.Context) {
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
}

// StopCleanup stops the cleanup goroutine.
func (m *memoryRateLimiterStore) StopCleanup() {
	if m.stopCh != nil {
		close(m.stopCh)
	}
}

// cleanup removes stale limiters that haven't been accessed recently.
func (m *memoryRateLimiterStore) cleanup(staleThreshold time.Duration) {
	cutoff := time.Now().Unix() - int64(staleThreshold.Seconds())

	m.limiters.Range(func(key, value any) bool {
		entry := value.(*limiterEntry)
		if atomic.LoadInt64(&entry.lastAccess) < cutoff {
			m.limiters.Delete(key)
		}

		return true
	})
}
