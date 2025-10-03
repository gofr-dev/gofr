package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestTokenBucket_Allow(t *testing.T) {
	cfg := RateLimiterConfig{Requests: 2, Burst: 2, Window: time.Second}
	tb := newTokenBucket(&cfg)

	// Should allow first two requests
	allowed, wait := tb.allow()
	assert.True(t, allowed)
	assert.Zero(t, wait)

	allowed, wait = tb.allow()
	assert.True(t, allowed)
	assert.Zero(t, wait)

	// Third request should be rate limited
	allowed, wait = tb.allow()
	assert.False(t, allowed)
	assert.GreaterOrEqual(t, wait, time.Millisecond)
}

func TestLocalRateLimiterStore_Allow(t *testing.T) {
	store := NewLocalRateLimiterStore()
	cfg := RateLimiterConfig{Requests: 1, Burst: 1, Window: time.Second}
	key := "test-key"

	allowed, retry, err := store.Allow(context.Background(), key, cfg)
	assert.True(t, allowed)
	assert.Zero(t, retry)
	require.NoError(t, err)

	allowed, retry, err = store.Allow(context.Background(), key, cfg)
	assert.False(t, allowed)
	assert.GreaterOrEqual(t, retry, time.Millisecond)
	assert.NoError(t, err)
}

func TestLocalRateLimiterStore_CleanupExpiredBuckets(t *testing.T) {
	store := NewLocalRateLimiterStore()
	cfg := RateLimiterConfig{Requests: 1, Burst: 1, Window: time.Second}
	key := "cleanup-key"

	_, _, err := store.Allow(context.Background(), key, cfg)
	require.NoError(t, err)

	// Simulate old lastAccess
	entry, _ := store.buckets.Load(key)
	bucketEntry := entry.(*bucketEntry)
	bucketEntry.lastAccess = time.Now().Unix() - int64(bucketTTL.Seconds()) - 1

	log := testutil.StdoutOutputForFunc(func() {
		store.cleanupExpiredBuckets(logging.NewMockLogger(logging.DEBUG))
	})

	_, exists := store.buckets.Load(key)
	assert.False(t, exists)
	assert.Contains(t, log, "Cleaned up rate limiter buckets")
}

func TestLocalRateLimiterStore_StartAndStopCleanup(t *testing.T) {
	store := NewLocalRateLimiterStore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store.StartCleanup(ctx, logging.NewMockLogger(logging.INFO))
	assert.NotNil(t, store.stopCh)

	store.StopCleanup()
}

func TestRedisRateLimiterStore_toInt64_ValidCases(t *testing.T) {
	tests := []struct {
		input    any
		expected int64
	}{
		{int64(5), 5},
		{int(7), 7},
		{float64(3.0), 3},
		{"42", 42},
		{"", 0},
	}

	for _, tc := range tests {
		val, err := toInt64(tc.input)

		require.NoError(t, err)
		assert.Equal(t, tc.expected, val)
	}
}

func TestRedisRateLimiterStore_toInt64_ErrorCases(t *testing.T) {
	_, err := toInt64(struct{}{})

	assert.ErrorIs(t, err, errInvalidRedisResultType)
}
