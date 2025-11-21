package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
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

func (m *rateLimiterMockMetrics) getCount(name string) int {
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

	handler := RateLimiter(config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	assert.Equal(t, 1, metrics.getCount("app_http_rate_limit_exceeded_total"))
}

func TestRateLimiter_PerIPLimit(t *testing.T) {
	metrics := newRateLimiterMockMetrics()
	config := RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		PerIP:             true,
	}

	handler := RateLimiter(config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	handler := RateLimiter(config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	handler := RateLimiter(config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup
	successCount := 0
	rateLimitedCount := 0
	var mu sync.Mutex

	// Send 20 concurrent requests from same IP
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(index int) {
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
		}(i)
	}

	wg.Wait()

	// Due to timing/race conditions in concurrent tests, we allow a small tolerance
	// The important thing is that rate limiting occurred
	assert.GreaterOrEqual(t, successCount, 9, "Should allow approximately burst size requests")
	assert.LessOrEqual(t, successCount, 11, "Should not allow significantly more than burst size")
	assert.Greater(t, rateLimitedCount, 0, "Should have some rate limited requests")
	assert.Equal(t, 20, successCount+rateLimitedCount, "Total requests should be 20")
}

func TestRateLimiter_TokenRefill(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping time-based test in short mode")
	}

	metrics := newRateLimiterMockMetrics()
	config := RateLimiterConfig{
		RequestsPerSecond: 5,  // 5 requests per second
		Burst:             2,
		PerIP:             false,
	}

	handler := RateLimiter(config, metrics)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	ip := getIP(req)
	assert.Equal(t, "203.0.113.1", ip, "Should extract first IP from X-Forwarded-For")
}

func TestGetIP_XRealIP(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Real-IP", "203.0.113.5")
	req.RemoteAddr = "192.168.1.1:12345"

	ip := getIP(req)
	assert.Equal(t, "203.0.113.5", ip, "Should extract IP from X-Real-IP")
}

func TestGetIP_RemoteAddr(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.RemoteAddr = "192.168.1.1:12345"

	ip := getIP(req)
	assert.Equal(t, "192.168.1.1", ip, "Should extract IP from RemoteAddr")
}

func TestGetIP_Priority(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("X-Forwarded-For", "203.0.113.1")
	req.Header.Set("X-Real-IP", "203.0.113.2")
	req.RemoteAddr = "192.168.1.1:12345"

	ip := getIP(req)
	assert.Equal(t, "203.0.113.1", ip, "X-Forwarded-For should have highest priority")
}
