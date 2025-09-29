package service

import (
	"context"
	"sync"
	"time"

	gofrRedis "gofr.dev/pkg/gofr/datasource/redis"
)

// RateLimiterStore abstracts the storage for rate limiter buckets.
type RateLimiterStore interface {
	Allow(ctx context.Context, key string, config RateLimiterConfig) (allowed bool, retryAfter time.Duration, err error)
}

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

// LocalRateLimiterStore implements RateLimiterStore using in-memory buckets.
type LocalRateLimiterStore struct {
	buckets *sync.Map
}

func NewLocalRateLimiterStore() *LocalRateLimiterStore {
	return &LocalRateLimiterStore{buckets: &sync.Map{}}
}

func (l *LocalRateLimiterStore) Allow(_ context.Context, key string, config RateLimiterConfig) (bool, time.Duration, error) {
	now := time.Now().Unix()
	entry, _ := l.buckets.LoadOrStore(key, &bucketEntry{
		bucket:     newTokenBucket(&config),
		lastAccess: now,
	})
	bucketEntry := entry.(*bucketEntry)
	allowed, retryAfter, _ := bucketEntry.bucket.allow()

	return allowed, retryAfter, nil
}
