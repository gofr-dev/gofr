package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	gofrRedis "gofr.dev/pkg/gofr/datasource/redis"
)

const (
	cleanupInterval = 5 * time.Minute  // How often to clean up unused buckets
	bucketTTL       = 10 * time.Minute // How long to keep unused buckets
)

// RateLimiterStore abstracts the storage and cleanup for rate limiter buckets.
type RateLimiterStore interface {
	Allow(ctx context.Context, key string, config RateLimiterConfig) (allowed bool, retryAfter time.Duration, err error)
	StartCleanup(ctx context.Context)
	StopCleanup()
}

// tokenBucket with simplified integer-only token handling.
type tokenBucket struct {
	tokens         int64 // Current tokens
	lastRefillTime int64 // Unix nano timestamp
	maxTokens      int64 // Maximum tokens
	refillRate     int64 // Tokens per second (as integer)
}

// bucketEntry holds bucket with last access time for cleanup.
type bucketEntry struct {
	bucket     *tokenBucket
	lastAccess int64 // Unix timestamp
}

// newTokenBucket creates a new token bucket with integer-only math.
func newTokenBucket(config *RateLimiterConfig) *tokenBucket {
	maxTokens := int64(config.Burst)
	refillRate := int64(config.RequestsPerSecond())

	return &tokenBucket{
		tokens:         maxTokens,
		lastRefillTime: time.Now().UnixNano(),
		maxTokens:      maxTokens,
		refillRate:     refillRate,
	}
}

// allow checks if a token can be consumed.
func (tb *tokenBucket) allow() (allowed bool, waitTime time.Duration) {
	now := time.Now().UnixNano()

	// Calculate tokens to add based on elapsed time
	elapsed := now - atomic.LoadInt64(&tb.lastRefillTime)
	tokensToAdd := elapsed * tb.refillRate / int64(time.Second)

	// Update tokens atomically
	for {
		oldTokens := atomic.LoadInt64(&tb.tokens)
		newTokens := oldTokens + tokensToAdd

		if newTokens > tb.maxTokens {
			newTokens = tb.maxTokens
		}

		// Early return if not enough tokens
		if newTokens < 1 {
			waitTime := time.Duration((1-newTokens)*int64(time.Second)/tb.refillRate) * time.Nanosecond
			if waitTime < time.Millisecond {
				waitTime = time.Millisecond
			}

			return false, waitTime
		}

		// Try to consume a token
		if atomic.CompareAndSwapInt64(&tb.tokens, oldTokens, newTokens-1) {
			atomic.StoreInt64(&tb.lastRefillTime, now)

			return true, 0
		}
	}
}

// LocalRateLimiterStore implements RateLimiterStore using in-memory buckets.
type LocalRateLimiterStore struct {
	buckets *sync.Map
	stopCh  chan struct{}
}

func NewLocalRateLimiterStore() *LocalRateLimiterStore {
	return &LocalRateLimiterStore{
		buckets: &sync.Map{},
	}
}

func (l *LocalRateLimiterStore) Allow(_ context.Context, key string, config RateLimiterConfig) (bool, time.Duration, error) {
	now := time.Now().Unix()
	entry, _ := l.buckets.LoadOrStore(key, &bucketEntry{
		bucket:     newTokenBucket(&config),
		lastAccess: now,
	})

	bucketEntry := entry.(*bucketEntry)

	atomic.StoreInt64(&bucketEntry.lastAccess, now)

	allowed, retryAfter := bucketEntry.bucket.allow()

	return allowed, retryAfter, nil
}

func (l *LocalRateLimiterStore) StartCleanup(ctx context.Context) {
	l.stopCh = make(chan struct{})

	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				l.cleanupExpiredBuckets()
			case <-l.stopCh:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (l *LocalRateLimiterStore) StopCleanup() {
	if l.stopCh != nil {
		close(l.stopCh)
	}
}

func (l *LocalRateLimiterStore) cleanupExpiredBuckets() {
	cutoff := time.Now().Unix() - int64(bucketTTL.Seconds())
	cleaned := 0

	l.buckets.Range(func(key, value any) bool {
		entry := value.(*bucketEntry)
		if atomic.LoadInt64(&entry.lastAccess) < cutoff {
			l.buckets.Delete(key)

			cleaned++
		}

		return true
	})
}

// tokenBucketScript is a Lua script for atomic token bucket rate limiting in Redis.
// Tokens are stored as a float so that partial refills carry over between calls, and the refill rate is
// derived from the window in nanoseconds so that sub-second and non-integer windows are honored.
//
//nolint:gosec // This is a Lua script for Redis, not credentials
const tokenBucketScript = `
local key = KEYS[1]
local burst = tonumber(ARGV[1])
local requests = tonumber(ARGV[2])
local window_ns = tonumber(ARGV[3])
local now = tonumber(ARGV[4])

-- Fetch bucket
local bucket = redis.call("HMGET", key, "tokens", "last_refill")
local tokens = tonumber(bucket[1])
local last_refill = tonumber(bucket[2])

if tokens == nil then
    tokens = burst
    last_refill = now
end

-- Refill continuously and keep the fraction. The refill clock never moves backwards, so a caller whose
-- clock lags behind another caller's does not earn the same elapsed time twice.
if now > last_refill then
    tokens = tokens + (now - last_refill) * requests / window_ns
    last_refill = now
end

tokens = math.min(burst, tokens)

local allowed = 0
local retry_after = 0

if tokens >= 1 then
    allowed = 1
    tokens = tokens - 1
else
    retry_after = math.ceil((1 - tokens) * window_ns / requests / 1e6) -- ms
end

redis.call("HSET", key, "tokens", tokens, "last_refill", last_refill)
redis.call("EXPIRE", key, 600)

return {allowed, retry_after}
`

// RedisRateLimiterStore implements RateLimiterStore using Redis.
type RedisRateLimiterStore struct {
	client *gofrRedis.Redis
}

func NewRedisRateLimiterStore(client *gofrRedis.Redis) *RedisRateLimiterStore {
	return &RedisRateLimiterStore{client: client}
}

func (r *RedisRateLimiterStore) Allow(ctx context.Context, key string, config RateLimiterConfig) (bool, time.Duration, error) {
	return r.allowAt(ctx, key, config, time.Now().UnixNano())
}

// allowAt runs the token bucket script for key at the given Unix nanosecond time.
func (r *RedisRateLimiterStore) allowAt(ctx context.Context, key string, config RateLimiterConfig,
	now int64) (bool, time.Duration, error) {
	cmd := r.client.Eval(
		ctx,
		tokenBucketScript,
		[]string{"gofr:ratelimit:" + key},
		config.Burst,                // ARGV[1]: burst
		config.Requests,             // ARGV[2]: requests
		config.Window.Nanoseconds(), // ARGV[3]: window_ns
		now,                         // ARGV[4]: now (nanoseconds)
	)

	result, err := cmd.Result()
	if err != nil {
		return true, 0, err // Fail open
	}

	resultArray, ok := result.([]any)
	if !ok || len(resultArray) != 2 {
		return true, 0, errInvalidRedisResultType // Fail open
	}

	allowed, _ := toInt64(resultArray[0])
	retryAfterMs, _ := toInt64(resultArray[1])

	return allowed == 1, time.Duration(retryAfterMs) * time.Millisecond, nil
}

func (*RedisRateLimiterStore) StartCleanup(_ context.Context) {
	// No-op: Redis handles cleanup automatically via EXPIRE commands in Lua script.
}

func (*RedisRateLimiterStore) StopCleanup() {
	// No-op: Redis handles cleanup automatically.
}

// toInt64 safely converts Redis result to int64.
func toInt64(i any) (int64, error) {
	switch v := i.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		return int64(v), nil
	case string:
		if v == "" {
			return 0, nil
		}

		return strconv.ParseInt(v, 10, 64)
	default:
		return 0, fmt.Errorf("%w: %T", errInvalidRedisResultType, i)
	}
}
