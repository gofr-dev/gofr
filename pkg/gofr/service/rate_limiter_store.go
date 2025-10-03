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
// Updated to use integer-only token math for simplicity
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

-- Refill tokens (integer math only)
local delta = math.max(0, (now - last_refill)/1e9)
local tokens_to_add = math.floor(delta * refill_rate)
local new_tokens = math.min(burst, tokens + tokens_to_add)

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

// RedisRateLimiterStore implements RateLimiterStore using Redis.
type RedisRateLimiterStore struct {
	client *gofrRedis.Redis
}

func NewRedisRateLimiterStore(client *gofrRedis.Redis) *RedisRateLimiterStore {
	return &RedisRateLimiterStore{client: client}
}

func (r *RedisRateLimiterStore) Allow(ctx context.Context, key string, config RateLimiterConfig) (bool, time.Duration, error) {
	now := time.Now().UnixNano()
	cmd := r.client.Eval(
		ctx,
		tokenBucketScript,
		[]string{"gofr:ratelimit:" + key},
		config.Burst,                   // ARGV[1]: burst
		config.Requests,                // ARGV[2]: requests
		int64(config.Window.Seconds()), // ARGV[3]: window_seconds
		now,                            // ARGV[4]: now (nanoseconds)
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
