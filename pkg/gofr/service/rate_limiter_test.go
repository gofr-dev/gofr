package service

import (
	"context"
	"errors"

	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// --- Simple logger mock ---
type mockLogger struct {
	logs []string
}

func (l *mockLogger) Log(_ ...any)   { l.logs = append(l.logs, "Log") }
func (l *mockLogger) Debug(_ ...any) { l.logs = append(l.logs, "Debug") }

type mockStore struct {
	allowed    bool
	retryAfter time.Duration
	err        error
}

func (m *mockStore) Allow(_ context.Context, _ string, _ RateLimiterConfig) (bool, time.Duration, error) {
	return m.allowed, m.retryAfter, m.err
}
func (*mockStore) StartCleanup(_ context.Context, _ Logger) {}

func (*mockStore) StopCleanup() {}

func TestRateLimiter_buildFullURL(t *testing.T) {
	httpSvc := &httpService{url: "http://base.com/api"}
	rl := &rateLimiter{HTTP: httpSvc}

	assert.Equal(t, "http://foo.com/bar", rl.buildFullURL("http://foo.com/bar"))
	assert.Equal(t, "https://foo.com/bar", rl.buildFullURL("https://foo.com/bar"))
	assert.Equal(t, "http://base.com/api/foo", rl.buildFullURL("foo"))
	assert.Equal(t, "http://base.com/api/foo", rl.buildFullURL("/foo"))

	httpSvc.url = ""

	assert.Equal(t, "bar", rl.buildFullURL("bar"))

	rl.HTTP = &mockHTTP{}

	assert.Equal(t, "baz", rl.buildFullURL("baz"))
}

func TestRateLimiter_checkRateLimit_Error(t *testing.T) {
	store := &mockStore{allowed: true, err: errors.New("fail")}
	logger := &mockLogger{}

	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)

	metrics.EXPECT().IncrementCounter(gomock.Any(), "app_rate_limiter_requests_total", "service", "svc")
	metrics.EXPECT().IncrementCounter(gomock.Any(), "app_rate_limiter_errors_total", "service", "svc", "type", "store_error")

	rl := &rateLimiter{
		config: RateLimiterConfig{
			KeyFunc: func(*http.Request) string { return "svc" },
			Store:   store,
		},
		store:   store,
		logger:  logger,
		metrics: metrics,
	}

	req, _ := http.NewRequest("GET", "/", nil)

	err := rl.checkRateLimit(req)

	assert.NoError(t, err)
	assert.Contains(t, logger.logs, "Log")
}

func TestRateLimiter_checkRateLimit_Denied(t *testing.T) {
	store := &mockStore{allowed: false}
	logger := &mockLogger{}

	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)

	metrics.EXPECT().IncrementCounter(gomock.Any(), "app_rate_limiter_requests_total", "service", "svc")
	metrics.EXPECT().IncrementCounter(gomock.Any(), "app_rate_limiter_denied_total", "service", "svc")

	rl := &rateLimiter{
		config: RateLimiterConfig{
			KeyFunc: func(*http.Request) string { return "svc" },
			Store:   store,
		},
		store:   store,
		logger:  logger,
		metrics: metrics,
	}

	req, _ := http.NewRequest("GET", "/", nil)
	err := rl.checkRateLimit(req)

	assert.IsType(t, &RateLimitError{}, err)
	assert.Contains(t, logger.logs, "Debug")
}

func TestRateLimiter_checkRateLimit_Allowed(t *testing.T) {
	store := &mockStore{allowed: true}

	logger := &mockLogger{}

	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)

	metrics.EXPECT().IncrementCounter(gomock.Any(), "app_rate_limiter_requests_total", "service", "svc")

	rl := &rateLimiter{
		config: RateLimiterConfig{
			KeyFunc: func(*http.Request) string { return "svc" },
			Store:   store,
		},
		store:   store,
		logger:  logger,
		metrics: metrics,
	}

	req, _ := http.NewRequest("GET", "/", nil)

	err := rl.checkRateLimit(req)
	assert.NoError(t, err)
}

func TestRateLimiter_HTTPMethods(t *testing.T) {
	mock := &mockHTTP{}

	store := &mockStore{allowed: true}
	logger := &mockLogger{}

	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)

	metrics.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	rl := &rateLimiter{
		config: RateLimiterConfig{
			KeyFunc: func(*http.Request) string { return "svc" },
			Store:   store,
		},
		store:   store,
		logger:  logger,
		metrics: metrics,
		HTTP:    mock,
	}

	ctx := context.Background()
	resp, err := rl.Get(ctx, "foo", nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = rl.GetWithHeaders(ctx, "foo", nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = rl.Post(ctx, "foo", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, err = rl.PostWithHeaders(ctx, "foo", nil, nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	resp, err = rl.Put(ctx, "foo", nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = rl.PutWithHeaders(ctx, "foo", nil, nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = rl.Patch(ctx, "foo", nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = rl.PatchWithHeaders(ctx, "foo", nil, nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	resp, err = rl.Delete(ctx, "foo", nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	resp, err = rl.DeleteWithHeaders(ctx, "foo", nil, nil)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
