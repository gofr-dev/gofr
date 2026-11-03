package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gofrRedis "gofr.dev/pkg/gofr/datasource/redis"
)

var (
	errRedisDown         = errors.New("redis down")
	errRedisArgsMismatch = errors.New("redis args mismatch")
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

	store.cleanupExpiredBuckets()

	_, exists := store.buckets.Load(key)
	assert.False(t, exists)
}

func TestLocalRateLimiterStore_StartAndStopCleanup(t *testing.T) {
	store := NewLocalRateLimiterStore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	store.StartCleanup(ctx)
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

func TestTokenBucket_AllowEdgeCases(t *testing.T) {
	testCases := []struct {
		desc       string
		config     RateLimiterConfig
		lastRefill time.Duration // offset applied to lastRefillTime before calling allow
		expAllowed bool
		expWait    time.Duration
		expTokens  int64
	}{
		{
			desc:       "refill after long idle is capped at burst",
			config:     RateLimiterConfig{Requests: 10, Burst: 3, Window: time.Second},
			lastRefill: -time.Hour,
			expAllowed: true,
			expWait:    0,
			expTokens:  2,
		},
		{
			desc:       "sub-millisecond wait is clamped to one millisecond",
			config:     RateLimiterConfig{Requests: 1e10, Burst: 0, Window: time.Second},
			expAllowed: false,
			expWait:    time.Millisecond,
			expTokens:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			tb := newTokenBucket(&tc.config)
			tb.lastRefillTime += int64(tc.lastRefill)

			allowed, wait := tb.allow()

			assert.Equal(t, tc.expAllowed, allowed)
			assert.Equal(t, tc.expWait, wait)
			assert.Equal(t, tc.expTokens, tb.tokens)
		})
	}
}

func TestRedisRateLimiterStore_Allow(t *testing.T) {
	cfg := RateLimiterConfig{Requests: 5, Burst: 2, Window: 10 * time.Second}

	testCases := []struct {
		desc       string
		mockCall   func(mock redismock.ClientMock)
		expAllowed bool
		expRetry   time.Duration
		expErr     error
	}{
		{
			desc:       "request allowed",
			mockCall:   func(mock redismock.ClientMock) { expectRateLimitEval(mock).SetVal([]any{int64(1), int64(0)}) },
			expAllowed: true,
			expRetry:   0,
		},
		{
			desc:       "request denied with retry after",
			mockCall:   func(mock redismock.ClientMock) { expectRateLimitEval(mock).SetVal([]any{int64(0), int64(250)}) },
			expAllowed: false,
			expRetry:   250 * time.Millisecond,
		},
		{
			desc:       "redis error fails open",
			mockCall:   func(mock redismock.ClientMock) { expectRateLimitEval(mock).SetErr(errRedisDown) },
			expAllowed: true,
			expRetry:   0,
			expErr:     errRedisDown,
		},
		{
			desc:       "non array result fails open",
			mockCall:   func(mock redismock.ClientMock) { expectRateLimitEval(mock).SetVal("unexpected") },
			expAllowed: true,
			expRetry:   0,
			expErr:     errInvalidRedisResultType,
		},
		{
			desc:       "result of wrong length fails open",
			mockCall:   func(mock redismock.ClientMock) { expectRateLimitEval(mock).SetVal([]any{int64(1)}) },
			expAllowed: true,
			expRetry:   0,
			expErr:     errInvalidRedisResultType,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			client, mock := redismock.NewClientMock()
			tc.mockCall(mock)

			store := NewRedisRateLimiterStore(&gofrRedis.Redis{Client: client})

			allowed, retry, err := store.Allow(t.Context(), "svc", cfg)

			assert.Equal(t, tc.expAllowed, allowed)
			assert.Equal(t, tc.expRetry, retry)
			require.ErrorIs(t, err, tc.expErr)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// expectRateLimitEval expects the token bucket EVAL for key "svc", matching every argument except the
// current timestamp, which differs on each call.
func expectRateLimitEval(mock redismock.ClientMock) *redismock.ExpectedCmd {
	return mock.CustomMatch(func(expected, actual []any) error {
		if len(actual) != len(expected) {
			return errRedisArgsMismatch
		}

		// Compare everything but the trailing "now" argument.
		for i := 0; i < len(expected)-1; i++ {
			if expected[i] != actual[i] {
				return errRedisArgsMismatch
			}
		}

		return nil
	}).ExpectEval(tokenBucketScript, []string{"gofr:ratelimit:svc"}, 2, float64(5), int64(10*time.Second), int64(0))
}

func TestRedisRateLimiterStore_CleanupIsNoOp(t *testing.T) {
	client, mock := redismock.NewClientMock()
	store := NewRedisRateLimiterStore(&gofrRedis.Redis{Client: client})

	store.StartCleanup(t.Context())
	store.StopCleanup()

	// Cleanup relies on the EXPIRE in the Lua script, so it must not issue any Redis command.
	require.NoError(t, mock.ExpectationsWereMet())
}

type redisBucketStep struct {
	at         time.Duration // offset from the pinned base time
	expAllowed bool
	expRetry   time.Duration
}

// TestRedisRateLimiterStore_AllowAt runs the token bucket script against miniredis with pinned
// timestamps, so unlike TestRedisRateLimiterStore_Allow above it exercises the Lua script itself.
func TestRedisRateLimiterStore_AllowAt(t *testing.T) {
	testCases := []struct {
		desc  string
		cfg   RateLimiterConfig
		seed  map[string]string // hash fields pre-set on the key (old-format data)
		steps []redisBucketStep
	}{
		{
			desc: "30 per minute refills every 2s while polled every 500ms",
			cfg:  RateLimiterConfig{Requests: 30, Window: time.Minute, Burst: 1},
			steps: []redisBucketStep{
				{0, true, 0}, {500 * time.Millisecond, false, 1500 * time.Millisecond},
				{time.Second, false, time.Second}, {1500 * time.Millisecond, false, 500 * time.Millisecond},
				{2 * time.Second, true, 0}, {2500 * time.Millisecond, false, 1500 * time.Millisecond},
			},
		},
		{
			desc: "5 per second polled every 100ms",
			cfg:  RateLimiterConfig{Requests: 5, Window: time.Second, Burst: 1},
			steps: []redisBucketStep{
				{0, true, 0}, {100 * time.Millisecond, false, 100 * time.Millisecond},
				{200 * time.Millisecond, true, 0}, {300 * time.Millisecond, false, 100 * time.Millisecond},
				{400 * time.Millisecond, true, 0},
			},
		},
		{
			desc: "sub-second window 5 per 500ms",
			cfg:  RateLimiterConfig{Requests: 5, Window: 500 * time.Millisecond, Burst: 5},
			steps: []redisBucketStep{
				{0, true, 0}, {0, true, 0}, {0, true, 0}, {0, true, 0}, {0, true, 0},
				{0, false, 100 * time.Millisecond}, {100 * time.Millisecond, true, 0},
			},
		},
		{
			desc: "non-integer window 3 per 1500ms",
			cfg:  RateLimiterConfig{Requests: 3, Window: 1500 * time.Millisecond, Burst: 1},
			steps: []redisBucketStep{
				{0, true, 0}, {400 * time.Millisecond, false, 100 * time.Millisecond}, {500 * time.Millisecond, true, 0},
			},
		},
		{
			desc: "partial refill is kept across consumes",
			cfg:  RateLimiterConfig{Requests: 1, Window: time.Second, Burst: 2},
			steps: []redisBucketStep{
				{0, true, 0}, {600 * time.Millisecond, true, 0}, {1200 * time.Millisecond, true, 0},
				{1300 * time.Millisecond, false, 700 * time.Millisecond},
			},
		},
		{
			desc: "refill after long idle is capped at burst",
			cfg:  RateLimiterConfig{Requests: 10, Window: time.Second, Burst: 3},
			steps: []redisBucketStep{
				{0, true, 0}, {time.Hour, true, 0}, {time.Hour, true, 0}, {time.Hour, true, 0},
				{time.Hour, false, 100 * time.Millisecond},
			},
		},
		{
			desc: "lagging clock does not re-credit elapsed time",
			cfg:  RateLimiterConfig{Requests: 1, Window: time.Second, Burst: 1},
			steps: []redisBucketStep{
				{0, true, 0}, {2 * time.Second, true, 0}, {time.Second, false, time.Second},
				{2500 * time.Millisecond, false, 500 * time.Millisecond},
			},
		},
		{
			desc:  "key written by the previous script version",
			cfg:   RateLimiterConfig{Requests: 1, Window: time.Second, Burst: 5},
			seed:  map[string]string{"tokens": "0", "last_refill": "1.767225600000000000e+18"},
			steps: []redisBucketStep{{500 * time.Millisecond, false, 500 * time.Millisecond}, {time.Second, true, 0}},
		},
	}

	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano()

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			runRedisBucketSteps(t, base, tc.cfg, tc.seed, tc.steps)
		})
	}
}

func runRedisBucketSteps(t *testing.T, base int64, cfg RateLimiterConfig, seed map[string]string, steps []redisBucketStep) {
	t.Helper()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	t.Cleanup(func() { _ = client.Close() })

	store := NewRedisRateLimiterStore(&gofrRedis.Redis{Client: client})

	for field, val := range seed {
		mr.HSet("gofr:ratelimit:svc", field, val)
	}

	for i, s := range steps {
		allowed, retry, err := store.allowAt(t.Context(), "svc", cfg, base+int64(s.at))
		require.NoError(t, err)
		assert.Equal(t, s.expAllowed, allowed, "step %d", i)
		assert.Equal(t, s.expRetry, retry, "step %d", i)
	}

	assert.Equal(t, 10*time.Minute, mr.TTL("gofr:ratelimit:svc"))
}
