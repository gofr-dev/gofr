package service

import (
	"context"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	backoffAttemptThreshold = 3
	unknownServiceKey       = "unknown"
	methodHTTP              = "http"
	methodHTTPS             = "https"
)

// tokenBucket with fractional accumulator for better precision.
type tokenBucket struct {
	tokens           int64      // Current tokens (scaled by scale)
	fractionalTokens float64    // Fractional remainder to avoid precision loss
	lastRefillTime   int64      // Unix nano timestamp
	maxTokens        int64      // Maximum tokens (scaled by scale)
	refillPerNano    float64    // Tokens per nanosecond (float64 for precision)
	fracMutex        sync.Mutex // Protects fractionalTokens
}

// LocalRateLimiter implements in-memory rate limiting using the Token Bucket algorithm.
// Strategy: Token Bucket with fractional precision for sub-1 RPS support
// - Suitable for: Single-instance deployments, development, testing
// - Limitations: Per-instance limiting only, not suitable for multi-instance production
// - Performance: Lock-free atomic operations with CAS loops

// localRateLimiter with metrics support.
type localRateLimiter struct {
	config  RateLimiterConfig
	store   RateLimiterStore
	logger  Logger
	metrics Metrics
	HTTP
}

// bucketEntry holds bucket with last access time for cleanup.
type bucketEntry struct {
	bucket     *tokenBucket
	lastAccess int64 // Unix timestamp
}

const (
	scale           int64 = 1e9                    // Scaling factor (typed constant)
	cleanupInterval       = 5 * time.Minute        // How often to clean up unused buckets
	bucketTTL             = 10 * time.Minute       // How long to keep unused buckets
	maxCASAttempts        = 10                     // ✅ FIX: Max CAS attempts
	maxCASTime            = 100 * time.Microsecond // ✅ FIX: Max CAS time
)

// NewLocalRateLimiter creates a new local rate limiter with metrics.
func NewLocalRateLimiter(config RateLimiterConfig, h HTTP, store RateLimiterStore) HTTP {
	httpSvc := h.(*httpService)

	rl := &localRateLimiter{
		config:  config,
		store:   store,
		logger:  httpSvc.Logger,
		metrics: httpSvc.Metrics,
		HTTP:    h,
	}

	go rl.cleanupRoutine()

	return rl
}

// newTokenBucket creates a new atomic token bucket with proper float64 scaling.
func newTokenBucket(config *RateLimiterConfig) *tokenBucket {
	maxScaled := int64(config.Burst) * scale

	requestsPerSecond := config.RequestsPerSecond()
	refillPerNanoFloat := requestsPerSecond * float64(scale) / float64(time.Second)

	return &tokenBucket{
		tokens:           maxScaled,
		fractionalTokens: 0.0,
		lastRefillTime:   time.Now().UnixNano(),
		maxTokens:        maxScaled,
		refillPerNano:    refillPerNanoFloat,
	}
}

// allow with enhanced precision and metrics.
func (tb *tokenBucket) allow() (allowed bool, waitTime time.Duration, tokensRemaining int64) {
	start := time.Now()

	for attempt := 0; attempt < maxCASAttempts && time.Since(start) < maxCASTime; attempt++ {
		now := time.Now().UnixNano()
		newTokens := tb.refillTokens(now)

		if newTokens < scale {
			retry := tb.calculateRetry(newTokens)
			tb.advanceTime(now)

			return false, retry, newTokens
		}

		if tb.consumeToken(newTokens, now) {
			return true, 0, newTokens - scale
		}

		tb.backoff(attempt)
	}

	return false, time.Second, 0
}

// refillTokens calculates and returns new token count after refilling based on elapsed time.
func (tb *tokenBucket) refillTokens(now int64) int64 {
	oldTime := atomic.LoadInt64(&tb.lastRefillTime)
	oldTokens := atomic.LoadInt64(&tb.tokens)

	elapsed := now - oldTime
	if elapsed < 0 {
		elapsed = 0
	}

	tb.fracMutex.Lock()
	tokensToAddFloat := float64(elapsed)*tb.refillPerNano + tb.fractionalTokens
	tokensToAdd := int64(tokensToAddFloat)
	tb.fractionalTokens = tokensToAddFloat - float64(tokensToAdd)
	tb.fracMutex.Unlock()

	newTokens := oldTokens + tokensToAdd
	if newTokens > tb.maxTokens {
		newTokens = tb.maxTokens
	}

	return newTokens
}

// calculateRetry computes the precise time duration until the next token becomes available.
func (tb *tokenBucket) calculateRetry(tokens int64) time.Duration {
	if tb.refillPerNano == 0 {
		return time.Second
	}

	missing := float64(scale - tokens)
	nanos := missing / tb.refillPerNano

	retry := time.Duration(nanos)
	if retry < time.Second {
		retry = time.Second
	}

	return retry
}

func (tb *tokenBucket) advanceTime(now int64) {
	oldTime := atomic.LoadInt64(&tb.lastRefillTime)
	atomic.CompareAndSwapInt64(&tb.lastRefillTime, oldTime, now)
}

func (tb *tokenBucket) consumeToken(tokens, now int64) bool {
	oldTokens := atomic.LoadInt64(&tb.tokens)

	if atomic.CompareAndSwapInt64(&tb.tokens, oldTokens, tokens-scale) {
		atomic.StoreInt64(&tb.lastRefillTime, now)

		return true
	}

	return false
}

func (*tokenBucket) backoff(attempt int) {
	if attempt < backoffAttemptThreshold {
		runtime.Gosched()
	} else {
		time.Sleep(time.Microsecond)
	}
}

// buildFullURL constructs an absolute URL by combining the base service URL with the given path.
func buildFullURL(path string, httpSvc HTTP) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}

	// Get base URL from embedded HTTP service
	httpSvcImpl, ok := httpSvc.(*httpService)
	if !ok {
		return path
	}

	base := strings.TrimRight(httpSvcImpl.url, "/")
	if base == "" {
		return path
	}

	// Ensure path starts with /
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return base + path
}

// checkRateLimit with custom keying support.
func (rl *localRateLimiter) checkRateLimit(req *http.Request) error {
	serviceKey := rl.config.KeyFunc(req)

	allowed, retryAfter, _ := rl.store.Allow(req.Context(), serviceKey, rl.config)

	rl.updateRateLimiterMetrics(req.Context(), serviceKey, allowed, 0)

	if !allowed {
		rl.logger.Debug("Rate limit exceeded", "service", serviceKey, "rate", rl.config.RequestsPerSecond, ""+
			"burst", rl.config.Burst, "retry_after", retryAfter)

		return &RateLimitError{ServiceKey: serviceKey, RetryAfter: retryAfter}
	}

	return nil
}

// updateRateLimiterMetrics follows GoFr's updateMetrics pattern.
func (rl *localRateLimiter) updateRateLimiterMetrics(ctx context.Context, serviceKey string, allowed bool, tokensAvailable float64) {
	if rl.metrics != nil {
		rl.metrics.IncrementCounter(ctx, "app_rate_limiter_requests_total", "service", serviceKey)

		if !allowed {
			rl.metrics.IncrementCounter(ctx, "app_rate_limiter_denied_total", "service", serviceKey)
		}

		rl.metrics.SetGauge("app_rate_limiter_tokens_available", tokensAvailable, "service", serviceKey)
	}
}

// cleanupRoutine removes unused buckets.
func (rl *localRateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Unix() - int64(bucketTTL.Seconds())
		cleaned := 0

		localStore, ok := rl.store.(*LocalRateLimiterStore)
		if !ok {
			continue // Not a local store, skip cleanup
		}

		localStore.buckets.Range(func(key, value any) bool {
			entry := value.(*bucketEntry)

			if atomic.LoadInt64(&entry.lastAccess) < cutoff {
				localStore.buckets.Delete(key)

				cleaned++
			}

			return true
		})

		if cleaned > 0 {
			rl.logger.Debug("Cleaned up rate limiter buckets", "count", cleaned)
		}
	}
}

// GetWithHeaders performs rate-limited HTTP GET request with custom headers.
func (rl *localRateLimiter) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

// PostWithHeaders performs rate-limited HTTP POST request with custom headers.
func (rl *localRateLimiter) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

// PatchWithHeaders performs rate-limited HTTP PATCH request with custom headers.
func (rl *localRateLimiter) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPatch, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

// PutWithHeaders performs rate-limited HTTP PUT request with custom headers.
func (rl *localRateLimiter) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

// DeleteWithHeaders performs rate-limited HTTP DELETE request with custom headers.
func (rl *localRateLimiter) DeleteWithHeaders(ctx context.Context, path string, body []byte,
	headers map[string]string) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}

// Get performs rate-limited HTTP GET request.
func (rl *localRateLimiter) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Get(ctx, path, queryParams)
}

// Post performs rate-limited HTTP POST request.
func (rl *localRateLimiter) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Post(ctx, path, queryParams, body)
}

// Patch performs rate-limited HTTP PATCH request.
func (rl *localRateLimiter) Patch(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPatch, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Patch(ctx, path, queryParams, body)
}

// Put performs rate-limited HTTP PUT request.
func (rl *localRateLimiter) Put(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Put(ctx, path, queryParams, body)
}

// Delete performs rate-limited HTTP DELETE request.
func (rl *localRateLimiter) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Delete(ctx, path, body)
}
