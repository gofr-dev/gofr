package service

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

// tokenBucketScript is a Lua script for atomic token bucket rate limiting in Redis.
//
//nolint:gosec // This is a Lua script for Redis, not credentials
const tokenBucketScript = `
local key = KEYS[1]
local burst = tonumber(ARGV[1])
local requests = tonumber(ARGV[2])
local window_seconds = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

-- Calculate refill rate as requests per second
local refill_rate = requests / window_seconds

-- Fetch bucket
local bucket = redis.call("HMGET", key, "tokens", "last_refill")
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if tokens == nil then
tokens = burst
last_refill = now
end

-- Refill tokens
local delta = math.max(0, (now - last_refill)/1e9)
local new_tokens = math.min(burst, tokens + delta * refill_rate)

local allowed = 0
local retryAfter = 0

if new_tokens >= 1 then
allowed = 1
new_tokens = new_tokens - 1
else
retryAfter = math.ceil((1 - new_tokens) / refill_rate * 1000) -- ms
end

redis.call("HSET", key, "tokens", new_tokens, "last_refill", now)
redis.call("EXPIRE", key, 600)

return {allowed, retryAfter}
`

// DistributedRateLimiter implements Redis-based distributed rate limiting using Token Bucket algorithm.
// Strategy: Token Bucket with Redis Lua scripts for atomic operations
// - Suitable for: Multi-instance production deployments
// - Benefits: True distributed limiting across all service instances
// - Performance: Single Redis call per rate limit check with atomic Lua execution

// distributedRateLimiter with metrics support.
type distributedRateLimiter struct {
	config  RateLimiterConfig
	store   RateLimiterStore
	logger  Logger
	metrics Metrics
	HTTP
}

func NewDistributedRateLimiter(config RateLimiterConfig, h HTTP) HTTP {
	httpSvc := h.(*httpService)

	rl := &distributedRateLimiter{
		config:  config,
		store:   NewRedisRateLimiterStore(config.RedisClient),
		logger:  httpSvc.Logger,
		metrics: httpSvc.Metrics,
		HTTP:    h,
	}

	return rl
}

// Safe Redis result parsing.
func toInt64(i any) (int64, error) {
	switch v := i.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("%w: %T", errInvalidRedisResultType, i)
	}
}

// checkRateLimit for distributed version with metrics.
func (rl *distributedRateLimiter) checkRateLimit(req *http.Request) error {
	serviceKey := rl.config.KeyFunc(req)

	allowed, retryAfter, err := rl.store.Allow(context.Background(), serviceKey, rl.config)
	if err != nil {
		rl.logger.Log("Rate limiter store error, allowing request", "error", err)

		rl.metrics.IncrementCounter(context.Background(), "app_rate_limiter_errors_total", "service", serviceKey, "type", "store_error")

		return nil
	}

	rl.metrics.IncrementCounter(context.Background(), "app_rate_limiter_requests_total", "service", serviceKey)

	if !allowed {
		rl.metrics.IncrementCounter(context.Background(), "app_rate_limiter_denied_total", "service", serviceKey)

		rl.logger.Debug("Distributed rate limit exceeded", "service", serviceKey, "retry_after", retryAfter)

		return &RateLimitError{ServiceKey: serviceKey, RetryAfter: retryAfter}
	}

	return nil
}

// GetWithHeaders performs rate-limited HTTP GET request with custom headers.
func (rl *distributedRateLimiter) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

// PostWithHeaders performs rate-limited HTTP POST request with custom headers.
func (rl *distributedRateLimiter) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

// PatchWithHeaders performs rate-limited HTTP PATCH request with custom headers.
func (rl *distributedRateLimiter) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPatch, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

// PutWithHeaders performs rate-limited HTTP PUT request with custom headers.
func (rl *distributedRateLimiter) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

// DeleteWithHeaders performs rate-limited HTTP DELETE request with custom headers.
func (rl *distributedRateLimiter) DeleteWithHeaders(ctx context.Context, path string, body []byte,
	headers map[string]string) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}

// Get performs rate-limited HTTP GET request.
func (rl *distributedRateLimiter) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Get(ctx, path, queryParams)
}

// Post performs rate-limited HTTP POST request.
func (rl *distributedRateLimiter) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Post(ctx, path, queryParams, body)
}

// Patch performs rate-limited HTTP PATCH request.
func (rl *distributedRateLimiter) Patch(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPatch, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Patch(ctx, path, queryParams, body)
}

// Put performs rate-limited HTTP PUT request.
func (rl *distributedRateLimiter) Put(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Put(ctx, path, queryParams, body)
}

// Delete performs rate-limited HTTP DELETE request.
func (rl *distributedRateLimiter) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	fullURL := buildFullURL(path, rl.HTTP)
	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, fullURL, http.NoBody)

	if err := rl.checkRateLimit(req); err != nil {
		return nil, err
	}

	return rl.HTTP.Delete(ctx, path, body)
}
