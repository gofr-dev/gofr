package service

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRateLimiterConfig_Validate(t *testing.T) {
	t.Run("invalid RPS", func(t *testing.T) {
		cfg := RateLimiterConfig{Requests: 0, Burst: 1}
		_ = cfg.Validate()

		assert.Equal(t, int(cfg.Requests), defaultRequestsPerMinute)
	})

	t.Run("burst less than requests", func(t *testing.T) {
		cfg := RateLimiterConfig{Requests: 5, Burst: 3}
		err := cfg.Validate()

		require.Error(t, err)
		assert.ErrorIs(t, err, errBurstLessThanRequests)
	})

	t.Run("sets default KeyFunc when nil", func(t *testing.T) {
		cfg := RateLimiterConfig{Requests: 1.5, Burst: 2}

		require.Nil(t, cfg.KeyFunc)
		require.NoError(t, cfg.Validate())
		require.NotNil(t, cfg.KeyFunc)
	})
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
