package service

import (
	"context"
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
		cfg := RateLimiterConfig{Requests: 0, Burst: 1}
		err := cfg.Validate()
		require.Error(t, err)
		assert.ErrorIs(t, err, errInvalidRequestRate)
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

func TestAddOption_InvalidConfigReturnsOriginal(t *testing.T) {
	h := newHTTPService(t)
	cfg := RateLimiterConfig{Requests: 0, Burst: 1} // invalid
	out := cfg.AddOption(h)
	assert.Same(t, h, out)
}

func TestAddOption_LocalLimiter(t *testing.T) {
	h := newHTTPService(t)
	cfg := RateLimiterConfig{Requests: 2, Burst: 3}
	out := cfg.AddOption(h)

	_, isLocal := out.(*localRateLimiter)
	assert.True(t, isLocal, "expected *localRateLimiter")

	assert.NotNil(t, cfg.KeyFunc)
}

func TestAddOption_DistributedLimiter(t *testing.T) {
	h := newHTTPService(t)
	cfg := RateLimiterConfig{
		Requests: 5,
		Burst:    5,
		Store:    NewRedisRateLimiterStore(new(gofrRedis.Redis)),
	}

	out := cfg.AddOption(h)
	_, isDist := out.(*distributedRateLimiter)

	assert.True(t, isDist, "expected *distributedRateLimiter")
}

type dummyStore struct{}

func (dummyStore) Allow(_ context.Context, _ string, _ RateLimiterConfig) (allowed bool,
	retryAfter time.Duration, err error) {
	return true, 0, nil
}

func TestNewDistributedRateLimiter_WithHTTPService_Success(t *testing.T) {
	config := RateLimiterConfig{
		Requests: 10,
		Window:   time.Minute,
		Burst:    10,
	}
	h := newHTTPService(t)
	store := &dummyStore{}

	result := NewDistributedRateLimiter(config, h, store)

	_, ok := result.(*distributedRateLimiter)
	assert.True(t, ok, "should return distributedRateLimiter")
}

func TestNewDistributedRateLimiter_WithHTTPService_Error(t *testing.T) {
	config := RateLimiterConfig{
		Requests: 0, // Invalid
		Window:   time.Minute,
		Burst:    10,
	}
	h := newHTTPService(t)
	store := &dummyStore{}

	result := NewDistributedRateLimiter(config, h, store)

	assert.Same(t, h, result, "should return original HTTP on invalid config")
}
