package service

import (
	"context"
	"errors"
	"math"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
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

func TestLocalRateLimiterStore_CleanupExpiredBuckets(t *testing.T) {
	store := NewLocalRateLimiterStore()
	cfg := RateLimiterConfig{Requests: 1, Burst: 1, Window: time.Second}
	key := "cleanup-key"

	_, _, err := store.Allow(t.Context(), key, cfg)
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
		allowed, wait := tb.allowAt(base + int64(step.at))

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
			desc: "unvalidated +Inf rate with zero window uses a 1ns interval",
			cfg:  RateLimiterConfig{Requests: math.Inf(1), Window: 0, Burst: 1},
			steps: []bucketStep{
				{at: 0, expAllowed: true},
				{at: 0, expWait: time.Millisecond},
				{at: 1, expAllowed: true},
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
		{desc: "zero window clamps to 1ns", cfg: RateLimiterConfig{Requests: 1, Window: 0}, expected: 1},
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

			results[i], _ = tb.allowAt(now)
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
	}).ExpectEval(tokenBucketScript, []string{"gofr:ratelimit:svc"}, 2, float64(5), int64(10), int64(0))
}

func TestRedisRateLimiterStore_CleanupIsNoOp(t *testing.T) {
	client, mock := redismock.NewClientMock()
	store := NewRedisRateLimiterStore(&gofrRedis.Redis{Client: client})

	store.StartCleanup(t.Context())
	store.StopCleanup()

	// Cleanup relies on the EXPIRE in the Lua script, so it must not issue any Redis command.
	require.NoError(t, mock.ExpectationsWereMet())
}
