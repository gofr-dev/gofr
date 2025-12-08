package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type rateLimiterMockMetrics struct {
	mu       sync.Mutex
	counters map[string]int
}

func newRateLimiterMockMetrics() *rateLimiterMockMetrics {
	return &rateLimiterMockMetrics{
		counters: make(map[string]int),
	}
}

func (m *rateLimiterMockMetrics) IncrementCounter(_ context.Context, name string, _ ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.counters[name]++
}

func (m *rateLimiterMockMetrics) GetCounter(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.counters[name]
}

func TestRateLimiter_GlobalLimit(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		PerIP:             false,
	}

	handler := RateLimiter(config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}))

	// First 2 requests should succeed (burst)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "Request %d should succeed", i+1)
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "Request should be rate limited")

	// Verify metric was incremented
	assert.Equal(t, 1, metrics.GetCounter("app_http_rate_limit_exceeded_total"))
}

func TestRateLimiter_PerIPLimit(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		PerIP:             true,
	}

	handler := RateLimiter(config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// IP1: First 2 requests should succeed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// IP1: 3rd request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)

	// IP2: Should still be able to make requests (different limiter)
	req = httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.RemoteAddr = "192.168.1.2:54321"
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestRateLimiter_SkipHealthEndpoints(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := RateLimiterConfig{
		RequestsPerSecond: 1,
		Burst:             1,
		PerIP:             false,
	}

	handler := RateLimiter(config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Health endpoints should not be rate limited
	healthPaths := []string{"/.well-known/health", "/.well-known/alive"}

	for _, path := range healthPaths {
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code, "Health endpoint %s should not be rate limited", path)
		}
	}
}

func TestRateLimiter_ConcurrentRequests(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             10,
		PerIP:             true,
	}

	handler := RateLimiter(config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup

	successCount := 0
	rateLimitedCount := 0

	var mu sync.Mutex

	// Send 20 concurrent requests from same IP
	for i := 0; i < 20; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
			req.RemoteAddr = "192.168.1.1:12345"
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			mu.Lock()

			if rr.Code == http.StatusOK {
				successCount++
			} else if rr.Code == http.StatusTooManyRequests {
				rateLimitedCount++
			}

			mu.Unlock()
		}()
	}

	wg.Wait()

	// Due to timing/race conditions in concurrent tests, we allow a small tolerance
	// The important thing is that rate limiting occurred
	assert.GreaterOrEqual(t, successCount, 9, "Should allow approximately burst size requests")
	assert.LessOrEqual(t, successCount, 11, "Should not allow significantly more than burst size")
	assert.Positive(t, rateLimitedCount, "Should have some rate limited requests")
	assert.Equal(t, 20, successCount+rateLimitedCount, "Total requests should be 20")
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping time-based test in short mode")
	}

	metrics := newRateLimiterMockMetrics()
	config := RateLimiterConfig{
		RequestsPerSecond: 5, // 5 requests per second
		Burst:             2,
		PerIP:             false,
	}

	handler := RateLimiter(config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Use up burst
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)
	}

	// Next request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)

	// Wait for token refill (200ms = 1 token at 5 req/sec)
	time.Sleep(220 * time.Millisecond)

	// Should succeed now
	req = httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetIP_XForwardedFor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1")
	req.RemoteAddr = "192.168.1.1:12345"

	ip := getIP(req, true)
	assert.Equal(t, "203.0.113.1", ip, "Should extract first IP from X-Forwarded-For when trusting proxies")

	// Without trusting proxies, should use RemoteAddr
	ip = getIP(req, false)
	assert.Equal(t, "192.168.1.1", ip, "Should use RemoteAddr when not trusting proxies")
}

func TestGetIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Real-IP", "203.0.113.5")
	req.RemoteAddr = "192.168.1.1:12345"

	ip := getIP(req, true)
	assert.Equal(t, "203.0.113.5", ip, "Should extract IP from X-Real-IP when trusting proxies")

	// Without trusting proxies, should use RemoteAddr
	ip = getIP(req, false)
	assert.Equal(t, "192.168.1.1", ip, "Should use RemoteAddr when not trusting proxies")
}

func TestGetIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.RemoteAddr = "192.168.1.1:12345"

	ip := getIP(req, false)
	assert.Equal(t, "192.168.1.1", ip, "Should extract IP from RemoteAddr")
}

func TestGetIP_Priority(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("X-Real-IP", "203.0.113.2")
	req.RemoteAddr = "192.168.1.1:12345"

	ip := getIP(req, true)
	assert.Equal(t, "203.0.113.1", ip, "X-Forwarded-For should have highest priority when trusting proxies")
}

func TestRateLimiter_RetryAfterHeader(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             1,
		PerIP:             false,
	}

	handler := RateLimiter(config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should succeed
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	// Second request should be rate limited and include Retry-After header
	req = httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.NotEmpty(t, rr.Header().Get("Retry-After"), "Retry-After header should be set")
}

func TestRateLimiterConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  RateLimiterConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: RateLimiterConfig{
				RequestsPerSecond: 10,
				Burst:             20,
				PerIP:             true,
			},
			wantErr: false,
		},
		{
			name: "zero RequestsPerSecond",
			config: RateLimiterConfig{
				RequestsPerSecond: 0,
				Burst:             20,
				PerIP:             true,
			},
			wantErr: true,
		},
		{
			name: "negative RequestsPerSecond",
			config: RateLimiterConfig{
				RequestsPerSecond: -5,
				Burst:             20,
				PerIP:             true,
			},
			wantErr: true,
		},
		{
			name: "zero Burst",
			config: RateLimiterConfig{
				RequestsPerSecond: 10,
				Burst:             0,
				PerIP:             true,
			},
			wantErr: true,
		},
		{
			name: "negative Burst",
			config: RateLimiterConfig{
				RequestsPerSecond: 10,
				Burst:             -5,
				PerIP:             true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMemoryRateLimiterStore_StopCleanupMultipleCalls(t *testing.T) {
	t.Helper()

	config := RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             20,
		PerIP:             true,
	}

	store := NewMemoryRateLimiterStore(config).(*memoryRateLimiterStore)
	ctx := context.Background()

	// Start cleanup
	store.StartCleanup(ctx)

	// Stop multiple times - should not panic
	store.StopCleanup()
	store.StopCleanup()
	store.StopCleanup()

	// Test passes if no panic occurs
}

func TestMemoryRateLimiterStore_Cleanup(t *testing.T) {
	config := RateLimiterConfig{
		RequestsPerSecond: 10,
		Burst:             20,
		PerIP:             true,
	}

	store := NewMemoryRateLimiterStore(config).(*memoryRateLimiterStore)
	ctx := context.Background()

	// Add some entries
	allowed1, _, _ := store.Allow(ctx, "ip1", config)
	allowed2, _, _ := store.Allow(ctx, "ip2", config)
	allowed3, _, _ := store.Allow(ctx, "ip3", config)

	assert.True(t, allowed1 && allowed2 && allowed3, "All initial requests should be allowed")

	// Verify entries exist
	count := 0

	store.limiters.Range(func(_, _ any) bool {
		count++
		return true
	})

	assert.Equal(t, 3, count, "Should have 3 entries")

	// Manually trigger cleanup with a threshold that marks all as stale
	// Set lastAccess to past time
	store.limiters.Range(func(_ any, value any) bool {
		entry := value.(*limiterEntry)
		atomic.StoreInt64(&entry.lastAccess, time.Now().Unix()-3600) // 1 hour ago

		return true
	})

	// Run cleanup with 10 minute threshold
	store.cleanup(10 * time.Minute)

	// Verify stale entries were removed
	count = 0

	store.limiters.Range(func(_, _ any) bool {
		count++
		return true
	})

	assert.Equal(t, 0, count, "Stale entries should be removed")
}

func TestRateLimiter_TrustedProxiesEnabled(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		PerIP:             true,
		TrustedProxies:    true, // Trust proxy headers
	}

	handler := RateLimiter(config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Send 2 requests from same X-Forwarded-For IP
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		req.RemoteAddr = "127.0.0.1:12345"               // Proxy IP
		req.Header.Set("X-Forwarded-For", "203.0.113.1") // Client IP
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// 3rd request from same X-Forwarded-For IP should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.1")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "Should rate limit based on X-Forwarded-For IP")

	// Different X-Forwarded-For IP should have separate limit
	req = httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.RemoteAddr = "127.0.0.1:12345"               // Same proxy
	req.Header.Set("X-Forwarded-For", "203.0.113.2") // Different client IP

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code, "Different client IP should have separate rate limit")
}

func TestRateLimiter_TrustedProxiesDisabled(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		PerIP:             true,
		TrustedProxies:    false, // Do not trust proxy headers
	}

	handler := RateLimiter(config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Send 2 requests with same RemoteAddr but different X-Forwarded-For
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
		req.RemoteAddr = "127.0.0.1:12345"
		req.Header.Set("X-Forwarded-For", fmt.Sprintf("203.0.113.%d", i+1)) // Different spoofed IPs
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}
	// 3rd request should be rate limited based on RemoteAddr, ignoring X-Forwarded-For
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.99") // Different spoofed IP

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "Should rate limit based on RemoteAddr, ignoring spoofed headers")
}
