package service

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStore struct {
	allowed    bool
	retryAfter time.Duration
	err        error
}

func (m *mockStore) Allow(_ context.Context, _ string, _ RateLimiterConfig) (bool, time.Duration, error) {
	return m.allowed, m.retryAfter, m.err
}
func (*mockStore) StartCleanup(_ context.Context) {}

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
	store := &mockStore{allowed: true, err: errTest}

	rl := &rateLimiter{
		config: RateLimiterConfig{
			KeyFunc: func(*http.Request) string { return "svc" },
			Store:   store,
		},
		store: store,
	}

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

	err := rl.checkRateLimit(req)

	require.NoError(t, err)
}

func TestRateLimiter_checkRateLimit_Denied(t *testing.T) {
	store := &mockStore{allowed: false}

	rl := &rateLimiter{
		config: RateLimiterConfig{
			KeyFunc: func(*http.Request) string { return "svc" },
			Store:   store,
		},
		store: store,
	}

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
	err := rl.checkRateLimit(req)

	assert.IsType(t, &RateLimitError{}, err)
}

func TestRateLimiter_checkRateLimit_Allowed(t *testing.T) {
	store := &mockStore{allowed: true}

	rl := &rateLimiter{
		config: RateLimiterConfig{
			KeyFunc: func(*http.Request) string { return "svc" },
			Store:   store,
		},
		store: store,
	}

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)

	err := rl.checkRateLimit(req)
	assert.NoError(t, err)
}

func TestRateLimiter_HTTPMethods(t *testing.T) {
	mock := &mockHTTP{}

	store := &mockStore{allowed: true}

	rl := &rateLimiter{
		config: RateLimiterConfig{
			KeyFunc: func(*http.Request) string { return "svc" },
			Store:   store,
		},
		store: store,
		HTTP:  mock,
	}

	ctx := context.Background()
	resp, err := rl.Get(ctx, "foo", nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()

	resp, err = rl.GetWithHeaders(ctx, "foo", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()

	resp, err = rl.Post(ctx, "foo", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	defer resp.Body.Close()

	resp, err = rl.PostWithHeaders(ctx, "foo", nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	defer resp.Body.Close()

	resp, err = rl.Put(ctx, "foo", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()

	resp, err = rl.PutWithHeaders(ctx, "foo", nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()

	resp, err = rl.Patch(ctx, "foo", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()

	resp, err = rl.PatchWithHeaders(ctx, "foo", nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	defer resp.Body.Close()

	resp, err = rl.Delete(ctx, "foo", nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	defer resp.Body.Close()

	resp, err = rl.DeleteWithHeaders(ctx, "foo", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	_ = resp.Body.Close()
}
