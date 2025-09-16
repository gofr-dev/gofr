package service

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	gofrRedis "gofr.dev/pkg/gofr/datasource/redis"
	"gofr.dev/pkg/gofr/logging"
)

func newHTTPService(t *testing.T) *httpService {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	return &httpService{
		Client: http.DefaultClient,
		url:    srv.URL,
		Logger: logging.NewMockLogger(logging.INFO),
		Tracer: otel.Tracer("gofr-http-client"),
	}
}

func TestRateLimiterConfig_Validate(t *testing.T) {
	t.Run("invalid RPS", func(t *testing.T) {
		cfg := RateLimiterConfig{RequestsPerSecond: 0, Burst: 1}
		err := cfg.Validate()
		require.Error(t, err)
		assert.ErrorIs(t, err, errInvalidRequestRate)
	})

	t.Run("invalid Burst", func(t *testing.T) {
		cfg := RateLimiterConfig{RequestsPerSecond: 1, Burst: 0}
		err := cfg.Validate()
		require.Error(t, err)
		assert.ErrorIs(t, err, errInvalidBurstSize)
	})

	t.Run("sets default KeyFunc when nil", func(t *testing.T) {
		cfg := RateLimiterConfig{RequestsPerSecond: 1.5, Burst: 2}
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

func TestAddOption_InvalidConfigReturnsOriginal(t *testing.T) {
	h := newHTTPService(t)
	cfg := RateLimiterConfig{RequestsPerSecond: 0, Burst: 1} // invalid
	out := cfg.AddOption(h)
	assert.Same(t, h, out)
}

func TestAddOption_LocalLimiter(t *testing.T) {
	h := newHTTPService(t)
	cfg := RateLimiterConfig{RequestsPerSecond: 2, Burst: 3}
	out := cfg.AddOption(h)

	_, isLocal := out.(*localRateLimiter)
	assert.True(t, isLocal, "expected *localRateLimiter")

	assert.NotNil(t, cfg.KeyFunc)
}

func TestAddOption_DistributedLimiter(t *testing.T) {
	h := newHTTPService(t)
	cfg := RateLimiterConfig{
		RequestsPerSecond: 5,
		Burst:             5,
		RedisClient:       new(gofrRedis.Redis),
	}

	out := cfg.AddOption(h)
	_, isDist := out.(*distributedRateLimiter)

	assert.True(t, isDist, "expected *distributedRateLimiter")
}

func TestRateLimitError(t *testing.T) {
	err := &RateLimitError{ServiceKey: "svc-x", RetryAfter: 1500 * time.Millisecond}

	assert.Contains(t, err.Error(), "svc-x")
	assert.Contains(t, err.Error(), "retry after")
	assert.Equal(t, http.StatusTooManyRequests, err.StatusCode())

	assert.NotErrorIs(t, err, errInvalidBurstSize, "unexpected error match")
}
