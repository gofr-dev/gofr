package service

import (
	"crypto/tls"
	"math"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiterConfig_Validate(t *testing.T) {
	testCases := []struct {
		desc        string
		cfg         RateLimiterConfig
		expErr      error
		expRequests float64
		expBurst    int
		expWindow   time.Duration
	}{
		{
			desc:        "zero requests falls back to default",
			cfg:         RateLimiterConfig{Requests: 0, Burst: 100},
			expErr:      errInvalidRequestRate,
			expRequests: defaultRequestsPerMinute,
			expBurst:    100,
			expWindow:   defaultWindow,
		},
		{
			desc:        "negative requests falls back to default",
			cfg:         RateLimiterConfig{Requests: -5, Burst: 100},
			expErr:      errInvalidRequestRate,
			expRequests: defaultRequestsPerMinute,
			expBurst:    100,
			expWindow:   defaultWindow,
		},
		{
			desc:        "NaN requests falls back to default",
			cfg:         RateLimiterConfig{Requests: math.NaN(), Burst: 100},
			expErr:      errInvalidRequestRate,
			expRequests: defaultRequestsPerMinute,
			expBurst:    100,
			expWindow:   defaultWindow,
		},
		{
			desc:        "+Inf requests falls back to default",
			cfg:         RateLimiterConfig{Requests: math.Inf(1), Burst: 100},
			expErr:      errInvalidRequestRate,
			expRequests: defaultRequestsPerMinute,
			expBurst:    100,
			expWindow:   defaultWindow,
		},
		{
			desc:        "fractional rate below one per second is valid",
			cfg:         RateLimiterConfig{Requests: 0.5, Window: time.Second, Burst: 1},
			expRequests: 0.5,
			expBurst:    1,
			expWindow:   time.Second,
		},
		{
			desc:        "exactly one per second is valid",
			cfg:         RateLimiterConfig{Requests: 1, Window: time.Second, Burst: 1},
			expRequests: 1,
			expBurst:    1,
			expWindow:   time.Second,
		},
		{
			desc:        "non-positive burst uses the default burst",
			cfg:         RateLimiterConfig{Requests: 5, Window: time.Second, Burst: 0},
			expRequests: 5,
			expBurst:    defaultBurstCapacity,
			expWindow:   time.Second,
		},
		{
			desc:        "burst less than requests is raised to requests",
			cfg:         RateLimiterConfig{Requests: 5, Burst: 3},
			expErr:      errBurstLessThanRequests,
			expRequests: 5,
			expBurst:    5,
			expWindow:   defaultWindow,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.Validate()

			require.ErrorIs(t, err, tc.expErr)
			assert.InDelta(t, tc.expRequests, tc.cfg.Requests, 0)
			assert.Equal(t, tc.expBurst, tc.cfg.Burst)
			assert.Equal(t, tc.expWindow, tc.cfg.Window)
			assert.NotNil(t, tc.cfg.KeyFunc)
		})
	}
}

func TestRateLimiterConfig_ValidateSetsDefaultKeyFunc(t *testing.T) {
	cfg := RateLimiterConfig{Requests: 1.5, Burst: 2}

	require.Nil(t, cfg.KeyFunc)
	require.NoError(t, cfg.Validate())
	require.NotNil(t, cfg.KeyFunc)
}

func TestDefaultKeyFunc(t *testing.T) {
	t.Run("nil request", func(t *testing.T) {
		assert.Equal(t, "unknown", defaultKeyFunc(nil))
	})

	t.Run("nil URL", func(t *testing.T) {
		req := &http.Request{}

		assert.Equal(t, "unknown", defaultKeyFunc(req))
	})

	t.Run("http derived scheme", func(t *testing.T) {
		req := &http.Request{
			URL: &url.URL{Host: "example.com"},
		}

		assert.Equal(t, "http://example.com", defaultKeyFunc(req))
	})

	t.Run("https derived scheme", func(t *testing.T) {
		req := &http.Request{
			URL: &url.URL{Host: "secure.com"},
			TLS: &tls.ConnectionState{},
		}

		assert.Equal(t, "https://secure.com", defaultKeyFunc(req))
	})

	t.Run("host from req.Host fallback", func(t *testing.T) {
		req := &http.Request{
			URL:  &url.URL{},
			Host: "fallback:9090",
		}

		assert.Equal(t, "http://fallback:9090", defaultKeyFunc(req))
	})

	t.Run("unknown service key when no host present", func(t *testing.T) {
		req := &http.Request{
			URL: &url.URL{},
		}

		assert.Equal(t, "http://unknown", defaultKeyFunc(req))
	})
}

func TestRequestsPerSecond(t *testing.T) {
	cfg := RateLimiterConfig{Requests: 10, Window: 2 * time.Second}

	assert.InEpsilon(t, 5.0, cfg.RequestsPerSecond(), 0.001)
}

func TestRateLimitError_ErrorAndStatusCode(t *testing.T) {
	err := &RateLimitError{ServiceKey: "svc", RetryAfter: 2 * time.Second}

	assert.Contains(t, err.Error(), "rate limit exceeded for service: svc")

	assert.Equal(t, http.StatusTooManyRequests, err.StatusCode())
}
