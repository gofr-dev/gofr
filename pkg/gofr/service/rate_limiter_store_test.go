package service

import (
	"context"
	"errors"
	"math"
	"slices"
	"sync"
	"sync/atomic"
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
	allowed, wait, _ := tb.allowAt(0)
	assert.True(t, allowed)
	assert.Zero(t, wait)

	allowed, wait, _ = tb.allowAt(0)
	assert.True(t, allowed)
	assert.Zero(t, wait)

	// Third request should be rate limited
	allowed, wait, _ = tb.allowAt(0)
	assert.False(t, allowed)
	assert.Equal(t, 500*time.Millisecond, wait)
}

func TestLocalRateLimiterStore_Allow(t *testing.T) {
	testCases := []struct {
		desc       string
		cfg        RateLimiterConfig
		allowed    int
		expMaxWait time.Duration
	}{
		{
			desc:       "1 per second, burst 1",
			cfg:        RateLimiterConfig{Requests: 1, Burst: 1, Window: time.Second},
			allowed:    1,
			expMaxWait: time.Second,
		},
		{
			desc:       "30 per minute, burst 30",
			cfg:        RateLimiterConfig{Requests: 30, Burst: 30, Window: time.Minute},
			allowed:    30,
			expMaxWait: 2 * time.Second,
		},
		{
			desc:       "0.5 per second, burst 1",
			cfg:        RateLimiterConfig{Requests: 0.5, Burst: 1, Window: time.Second},
			allowed:    1,
			expMaxWait: 2 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			store := NewLocalRateLimiterStore()
			key := "test-key"

			for range tc.allowed {
				allowed, retry, err := store.Allow(t.Context(), key, tc.cfg)
				require.NoError(t, err)
				assert.True(t, allowed)
				assert.Zero(t, retry)
			}

			allowed, retry, err := store.Allow(t.Context(), key, tc.cfg)
			require.NoError(t, err)
			assert.False(t, allowed)
			assert.GreaterOrEqual(t, retry, time.Millisecond)
			assert.LessOrEqual(t, retry, tc.expMaxWait)
		})
	}
}

func TestLocalRateLimiterStore_AllowDoesNotAllocateForAnExistingKey(t *testing.T) {
	store := NewLocalRateLimiterStore()
	cfg := RateLimiterConfig{Requests: 1, Burst: 1, Window: time.Second}
	ctx := t.Context()

	_, _, err := store.Allow(ctx, "steady-key", cfg)
	require.NoError(t, err)

	allocs := testing.AllocsPerRun(100, func() {
		_, _, _ = store.Allow(ctx, "steady-key", cfg)
	})

	assert.Zero(t, allocs)
}

func BenchmarkLocalRateLimiterStore_Allow(b *testing.B) {
	store := NewLocalRateLimiterStore()
	cfg := RateLimiterConfig{Requests: 1e6, Burst: 1e6, Window: time.Second}
	ctx := b.Context()

	b.ReportAllocs()

	for b.Loop() {
		_, _, _ = store.Allow(ctx, "bench-key", cfg)
	}
}

func TestLocalRateLimiterStore_CleanupExpiredBuckets(t *testing.T) {
	store := NewLocalRateLimiterStore()
	cfg := RateLimiterConfig{Requests: 1, Burst: 1, Window: time.Second}
	key := "cleanup-key"

	_, _, err := store.Allow(t.Context(), key, cfg)
	require.NoError(t, err)

	// Simulate a store that was created, and last used, longer ago than bucketTTL.
	store.epoch = store.epoch.Add(-bucketTTL - time.Minute)

	entry, _ := store.buckets.Load(key)
	atomic.StoreInt64(&entry.(*bucketEntry).lastAccess, 0)

	store.cleanupExpiredBuckets()

	_, exists := store.buckets.Load(key)
	assert.False(t, exists)
}

// countAllowed sends requests for key at the given time until one is denied and returns how many were allowed.
func countAllowed(store *LocalRateLimiterStore, key string, cfg *RateLimiterConfig, now int64) int {
	allowed := 0

	for ok, _ := store.allowAt(key, cfg, now); ok; ok, _ = store.allowAt(key, cfg, now) {
		allowed++
	}

	return allowed
}

// TestLocalRateLimiterStore_CleanupKeepsOwedCredit runs a forced cleanup on a pinned clock and checks how many
// requests the next caller gets: an evicted bucket comes back full, a kept one still owes its credit.
func TestLocalRateLimiterStore_CleanupKeepsOwedCredit(t *testing.T) {
	// 5 per hour refills one token every 12 minutes, so an emptied bucket needs an hour to refill.
	slow := RateLimiterConfig{Requests: 5, Window: time.Hour, Burst: 5}

	testCases := []struct {
		desc       string
		cleanupAt  time.Duration
		expEvicted bool
		expAllowed int // requests allowed at cleanupAt, after the cleanup
	}{
		{desc: "idle past the TTL but still owing credit is kept", cleanupAt: 15 * time.Minute, expAllowed: 1},
		{desc: "idle past the TTL and nearly refilled is kept", cleanupAt: 59 * time.Minute, expAllowed: 4},
		{desc: "idle past the TTL and fully refilled is evicted", cleanupAt: time.Hour, expEvicted: true, expAllowed: 5},
		{desc: "accessed within the TTL is kept", cleanupAt: 5 * time.Minute, expAllowed: 0},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			store := NewLocalRateLimiterStore()
			at := int64(tc.cleanupAt)

			for range slow.Burst {
				allowed, _ := store.allowAt("slow", &slow, 0)
				require.True(t, allowed)
			}

			store.cleanupAt(at)

			_, exists := store.buckets.Load("slow")
			assert.Equal(t, tc.expEvicted, !exists)

			assert.Equal(t, tc.expAllowed, countAllowed(store, "slow", &slow, at))
		})
	}
}

// TestLocalRateLimiterStore_CleanupRacingARequest pins the interleavings where a request loads a bucket just
// before cleanup evicts it: the request must not spend a token on the evicted bucket, it must use a new one.
func TestLocalRateLimiterStore_CleanupRacingARequest(t *testing.T) {
	cfg := RateLimiterConfig{Requests: 1, Window: time.Hour, Burst: 1}
	idle := int64(2 * bucketTTL)

	t.Run("request loaded before the cleanup sees the bucket as evicted", func(t *testing.T) {
		store := NewLocalRateLimiterStore()
		_, _ = store.allowAt("race", &cfg, -int64(time.Hour)) // consumed an hour ago, so refilled by now
		loaded, _ := store.buckets.Load("race")

		store.cleanupAt(idle)

		allowed, _, evicted := loaded.(*bucketEntry).bucket.allowAt(idle)
		assert.False(t, allowed)
		assert.True(t, evicted)
	})

	t.Run("request finding an evicted bucket still mapped uses a new bucket", func(t *testing.T) {
		store := NewLocalRateLimiterStore()
		_, _ = store.allowAt("race", &cfg, -int64(time.Hour))
		stale, _ := store.buckets.Load("race")

		require.True(t, stale.(*bucketEntry).bucket.evictAt(idle)) // cleanup marked it but has not deleted it yet

		allowed, _ := store.allowAt("race", &cfg, idle)
		current, _ := store.buckets.Load("race")

		assert.True(t, allowed)
		assert.NotSame(t, stale, current)
		assert.Equal(t, 0, countAllowed(store, "race", &cfg, idle))
	})

	t.Run("cleanup does not evict a bucket a request consumed from after the idle check", func(t *testing.T) {
		store := NewLocalRateLimiterStore()
		_, _ = store.allowAt("race", &cfg, -int64(time.Hour))
		loaded, _ := store.buckets.Load("race")
		bucket := loaded.(*bucketEntry).bucket

		allowed, _, _ := bucket.allowAt(idle) // the request wins the race against evictAt

		assert.True(t, allowed)
		assert.False(t, bucket.evictAt(idle))
	})
}

func TestLocalRateLimiterStore_ClockIsMonotonic(t *testing.T) {
	store := NewLocalRateLimiterStore()
	cfg := RateLimiterConfig{Requests: 1, Burst: 1, Window: time.Hour}

	_, _, err := store.Allow(t.Context(), "clock-key", cfg)
	require.NoError(t, err)

	elapsed := int64(time.Since(store.epoch))
	entry, _ := store.buckets.Load("clock-key")
	bucketEntry := entry.(*bucketEntry)

	// The epoch keeps its monotonic reading ("m=" in its String form), and bucket times are measured from it
	// rather than from the Unix epoch, so a wall-clock step cannot move them.
	assert.Contains(t, store.epoch.String(), " m=")
	assert.LessOrEqual(t, atomic.LoadInt64(&bucketEntry.lastAccess), elapsed)
	assert.LessOrEqual(t, atomic.LoadInt64(&bucketEntry.bucket.tat), elapsed+int64(time.Hour))
}

func TestLocalRateLimiterStore_StartAndStopCleanup(t *testing.T) {
	store := NewLocalRateLimiterStore()

	ctx, cancel := context.WithCancel(t.Context())
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

type bucketStep struct {
	at         time.Duration // offset from the pinned base time
	expAllowed bool
	expWait    time.Duration
}

// allowedSteps returns n steps at the same instant that are all expected to be allowed.
func allowedSteps(n int, at time.Duration) []bucketStep {
	steps := make([]bucketStep, n)
	for i := range steps {
		steps[i] = bucketStep{at: at, expAllowed: true}
	}

	return steps
}

// runBucketSteps feeds each step to a fresh bucket at a pinned base time and asserts the result.
func runBucketSteps(t *testing.T, cfg *RateLimiterConfig, steps []bucketStep) {
	t.Helper()

	tb := newTokenBucket(cfg)
	base := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano()

	for i, step := range steps {
		allowed, wait, _ := tb.allowAt(base + int64(step.at))

		assert.Equal(t, step.expAllowed, allowed, "step %d", i)
		assert.Equal(t, step.expWait, wait, "step %d", i)
	}
}

func TestTokenBucket_AllowAt(t *testing.T) {
	testCases := []struct {
		desc  string
		cfg   RateLimiterConfig
		steps []bucketStep
	}{
		{
			desc: "fractional rate 0.5/s refills one token every 2s",
			cfg:  RateLimiterConfig{Requests: 0.5, Window: time.Second, Burst: 1},
			steps: []bucketStep{
				{at: 0, expAllowed: true},
				{at: time.Second, expWait: time.Second},
				{at: 2 * time.Second, expAllowed: true},
				{at: 3 * time.Second, expWait: time.Second},
				{at: 4 * time.Second, expAllowed: true},
			},
		},
		{
			desc: "30 per minute allows the burst then one token every 2s",
			cfg:  RateLimiterConfig{Requests: 30, Window: time.Minute, Burst: 30},
			steps: append(allowedSteps(30, 0),
				bucketStep{at: 0, expWait: 2 * time.Second},
				bucketStep{at: 2 * time.Second, expAllowed: true},
			),
		},
		{
			desc: "exactly 1/s with sub-millisecond wait clamped to 1ms",
			cfg:  RateLimiterConfig{Requests: 1, Window: time.Second, Burst: 1},
			steps: []bucketStep{
				{at: 0, expAllowed: true},
				{at: time.Second - 1, expWait: time.Millisecond},
				{at: time.Second, expAllowed: true},
			},
		},
		{
			desc: "partial refill is kept across consumes",
			cfg:  RateLimiterConfig{Requests: 1, Window: time.Second, Burst: 2},
			steps: []bucketStep{
				{at: 0, expAllowed: true},
				{at: 600 * time.Millisecond, expAllowed: true},
				{at: 1200 * time.Millisecond, expAllowed: true},
				{at: 1300 * time.Millisecond, expWait: 700 * time.Millisecond},
			},
		},
		{
			desc: "refill after long idle is capped at burst",
			cfg:  RateLimiterConfig{Requests: 10, Window: time.Second, Burst: 3},
			steps: append([]bucketStep{{at: 0, expAllowed: true}},
				append(allowedSteps(3, time.Hour), bucketStep{at: time.Hour, expWait: 100 * time.Millisecond})...),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			runBucketSteps(t, &tc.cfg, tc.steps)
		})
	}
}

func TestTokenBucket_AllowAtEdgeCases(t *testing.T) {
	testCases := []struct {
		desc  string
		cfg   RateLimiterConfig
		steps []bucketStep
	}{
		{
			desc:  "zero burst always denies with wait clamped to 1ms",
			cfg:   RateLimiterConfig{Requests: 1e10, Window: time.Second, Burst: 0},
			steps: []bucketStep{{at: 0, expWait: time.Millisecond}},
		},
		{
			desc:  "negative burst always denies",
			cfg:   RateLimiterConfig{Requests: 1, Window: time.Second, Burst: -1},
			steps: []bucketStep{{at: 0, expWait: time.Second}},
		},
		{
			desc:  "huge burst is capped without overflow",
			cfg:   RateLimiterConfig{Requests: 1, Window: time.Second, Burst: math.MaxInt},
			steps: allowedSteps(3, 0),
		},
		{
			desc: "unvalidated NaN rate uses the default rate",
			cfg:  RateLimiterConfig{Requests: math.NaN(), Window: time.Minute, Burst: 1},
			steps: []bucketStep{
				{at: 0, expAllowed: true},
				{at: 0, expWait: time.Second},
			},
		},
		{
			desc: "unvalidated +Inf rate with zero window uses the default rate",
			cfg:  RateLimiterConfig{Requests: math.Inf(1), Window: 0, Burst: 1},
			steps: []bucketStep{
				{at: 0, expAllowed: true},
				{at: 1, expWait: time.Second - 1},
				{at: time.Second, expAllowed: true},
			},
		},
		{
			desc: "unvalidated zero window keeps Requests and uses the default window",
			cfg:  RateLimiterConfig{Requests: 10, Window: 0, Burst: 1},
			steps: []bucketStep{
				{at: 0, expAllowed: true},
				{at: 0, expWait: 6 * time.Second},
			},
		},
		{
			desc: "rate that does not divide a second is rounded down to fail closed",
			cfg:  RateLimiterConfig{Requests: 6e8, Window: time.Second, Burst: 1},
			steps: []bucketStep{
				{at: 0, expAllowed: true},
				{at: 1, expWait: time.Millisecond},
				{at: 2, expAllowed: true},
			},
		},
		{
			desc: "tiny rate with burst 1 denies after one request",
			cfg:  RateLimiterConfig{Requests: 1e-300, Window: time.Second, Burst: 1},
			steps: []bucketStep{
				{at: 0, expAllowed: true},
				{at: time.Hour, expWait: time.Duration(maxRefillPeriod) - time.Hour},
			},
		},
		{
			desc: "tiny rate with large burst is limited to the burst that fits the max refill period",
			cfg:  RateLimiterConfig{Requests: 1e-300, Window: time.Second, Burst: 1000},
			steps: []bucketStep{
				{at: 0, expAllowed: true},
				{at: 0, expWait: time.Duration(maxRefillPeriod)},
			},
		},
		{
			desc: "one request per 200 years with burst 2 denies the second request",
			cfg:  RateLimiterConfig{Requests: 1, Window: 200 * 365 * 24 * time.Hour, Burst: 2},
			steps: []bucketStep{
				{at: 0, expAllowed: true},
				{at: 0, expWait: time.Duration(maxRefillPeriod)},
			},
		},
		{
			desc: "slow rate with large burst keeps the configured rate",
			cfg:  RateLimiterConfig{Requests: 1, Window: 24 * time.Hour, Burst: 100000},
			steps: append(allowedSteps(26687, 0),
				bucketStep{at: 0, expWait: 24 * time.Hour},
				bucketStep{at: 24 * time.Hour, expAllowed: true},
				bucketStep{at: 24 * time.Hour, expWait: 24 * time.Hour}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			runBucketSteps(t, &tc.cfg, tc.steps)
		})
	}
}

func TestNewTokenBucket(t *testing.T) {
	testCases := []struct {
		desc             string
		cfg              RateLimiterConfig
		expInterval      int64
		expBurstInterval int64
	}{
		{desc: "regular config", cfg: RateLimiterConfig{Requests: 30, Window: time.Minute, Burst: 30},
			expInterval: int64(2 * time.Second), expBurstInterval: int64(60 * time.Second)},
		{desc: "negative burst holds no tokens", cfg: RateLimiterConfig{Requests: 1, Window: time.Second, Burst: -1},
			expInterval: int64(time.Second), expBurstInterval: 0},
		{desc: "huge burst at 1ns interval is capped at the max refill period", cfg: RateLimiterConfig{Requests: 1e10,
			Window: time.Second, Burst: math.MaxInt}, expInterval: 1, expBurstInterval: maxRefillPeriod},
		{desc: "huge burst keeps the rate and caps the burst credit", cfg: RateLimiterConfig{Requests: 1,
			Window: time.Second, Burst: math.MaxInt}, expInterval: int64(time.Second),
			expBurstInterval: 2305843009 * int64(time.Second)},
		{desc: "tiny rate keeps the max interval and a burst of one", cfg: RateLimiterConfig{Requests: 1e-300,
			Window: time.Second, Burst: 1000}, expInterval: maxRefillPeriod, expBurstInterval: maxRefillPeriod},
		{desc: "one per day with burst 100000 keeps the 24h interval", cfg: RateLimiterConfig{Requests: 1,
			Window: 24 * time.Hour, Burst: 100000}, expInterval: int64(24 * time.Hour),
			expBurstInterval: 26687 * int64(24*time.Hour)},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			tb := newTokenBucket(&tc.cfg)

			assert.Equal(t, tc.expInterval, tb.interval)
			assert.Equal(t, tc.expBurstInterval, tb.burstInterval)
		})
	}
}

func TestRefillInterval(t *testing.T) {
	testCases := []struct {
		desc     string
		cfg      RateLimiterConfig
		expected int64
	}{
		{desc: "30 per minute is one token every 2s", cfg: RateLimiterConfig{Requests: 30, Window: time.Minute},
			expected: int64(2 * time.Second)},
		{desc: "fractional rate", cfg: RateLimiterConfig{Requests: 0.5, Window: time.Second},
			expected: int64(2 * time.Second)},
		{desc: "rate above one per nanosecond clamps to 1ns", cfg: RateLimiterConfig{Requests: 1e10, Window: time.Second},
			expected: 1},
		{desc: "zero window uses the default window", cfg: RateLimiterConfig{Requests: 1, Window: 0},
			expected: int64(time.Minute)},
		{desc: "negative window uses the default window", cfg: RateLimiterConfig{Requests: 10, Window: -time.Second},
			expected: int64(6 * time.Second)},
		{desc: "+Inf rate uses the default rate", cfg: RateLimiterConfig{Requests: math.Inf(1), Window: time.Minute},
			expected: int64(time.Second)},
		{desc: "+Inf rate with zero window uses the default rate and window",
			cfg: RateLimiterConfig{Requests: math.Inf(1), Window: 0}, expected: int64(time.Second)},
		{desc: "rate at or above math.MaxInt uses the default rate",
			cfg: RateLimiterConfig{Requests: 1e19, Window: time.Minute}, expected: int64(time.Second)},
		{desc: "3 per second rounds 333333333.3ns up", cfg: RateLimiterConfig{Requests: 3, Window: time.Second},
			expected: 333333334},
		{desc: "6e8 per second rounds 1.67ns up to 2ns", cfg: RateLimiterConfig{Requests: 6e8, Window: time.Second},
			expected: 2},
		{desc: "one per day is exact", cfg: RateLimiterConfig{Requests: 1, Window: 24 * time.Hour},
			expected: int64(24 * time.Hour)},
		{desc: "tiny rate clamps to the max refill period", cfg: RateLimiterConfig{Requests: 1e-300, Window: time.Second},
			expected: maxRefillPeriod},
		{desc: "zero rate uses the default rate", cfg: RateLimiterConfig{Requests: 0, Window: time.Minute},
			expected: int64(time.Second)},
		{desc: "negative rate uses the default rate", cfg: RateLimiterConfig{Requests: -1, Window: time.Minute},
			expected: int64(time.Second)},
		{desc: "NaN rate uses the default rate", cfg: RateLimiterConfig{Requests: math.NaN(), Window: time.Minute},
			expected: int64(time.Second)},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.expected, refillInterval(&tc.cfg))
		})
	}
}

func TestTokenBucket_ConcurrentBurst(t *testing.T) {
	const callers = 1000

	cfg := RateLimiterConfig{Requests: 1, Window: time.Hour, Burst: 50}
	tb := newTokenBucket(&cfg)
	now := time.Now().UnixNano()
	results := make([]bool, callers)

	var wg sync.WaitGroup

	for i := range results {
		wg.Add(1)

		go func() {
			defer wg.Done()

			results[i], _, _ = tb.allowAt(now)
		}()
	}

	wg.Wait()

	assert.Len(t, slices.DeleteFunc(results, func(allowed bool) bool { return !allowed }), 50)
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
