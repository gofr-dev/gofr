package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/logging"
)

func newBaseHTTPService(t *testing.T, hitCounter *atomic.Int64) *httpService {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hitCounter.Add(1)
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

func assertAllowed(t *testing.T, resp *http.Response, err error) {
	t.Helper()
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func assertRateLimited(t *testing.T, err error, key ...string) {
	t.Helper()
	require.Error(t, err)

	var rlErr *RateLimitError

	require.ErrorAs(t, err, &rlErr)

	if len(key) > 0 {
		assert.Equal(t, key[0], rlErr.ServiceKey)
	}

	assert.GreaterOrEqual(t, rlErr.RetryAfter, time.Second)
}

func wait(d time.Duration) { time.Sleep(d) }

func TestNewLocalRateLimiter_Basic(t *testing.T) {
	var hits atomic.Int64

	base := newBaseHTTPService(t, &hits)

	rl := NewLocalRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 5,
		Burst:             5,
		KeyFunc:           func(*http.Request) string { return "svc-basic" },
	}, base)

	resp, err := rl.Get(t.Context(), "/ok", nil)
	assertAllowed(t, resp, err)

	if resp != nil {
		_ = resp.Body.Close()
	}

	assert.Equal(t, int64(1), hits.Load())
}

// Burst=1 then immediate second call denied; after refill allowed again.
func TestLocalRateLimiter_EnforceLimit(t *testing.T) {
	var hits atomic.Int64

	base := newBaseHTTPService(t, &hits)

	rl := NewLocalRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 1,
		Burst:             1,
		KeyFunc:           func(*http.Request) string { return "svc-limit" },
	}, base)

	resp, err := rl.Get(t.Context(), "/r1", nil)
	assertAllowed(t, resp, err)

	if resp != nil {
		_ = resp.Body.Close()
	}

	resp, err = rl.Get(t.Context(), "/r2", nil)
	require.Nil(t, resp)
	assertRateLimited(t, err)

	if resp != nil {
		_ = resp.Body.Close()
	}

	wait(1100 * time.Millisecond)

	resp, err = rl.Get(t.Context(), "/r3", nil)
	assertAllowed(t, resp, err)

	if resp != nil {
		_ = resp.Body.Close()
	}

	assert.Equal(t, int64(2), hits.Load())
}

// Fractional RPS (0.5 -> 1 token every 2s).
func TestLocalRateLimiter_FractionalRPS(t *testing.T) {
	var hits atomic.Int64

	base := newBaseHTTPService(t, &hits)

	rl := NewLocalRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 0.5,
		Burst:             1,
		KeyFunc:           func(*http.Request) string { return "svc-frac" },
	}, base)

	resp, err := rl.Get(t.Context(), "/a", nil)
	assertAllowed(t, resp, err)

	if resp != nil {
		_ = resp.Body.Close()
	}

	resp, err = rl.Get(t.Context(), "/b", nil)
	require.Nil(t, resp)
	assertRateLimited(t, err)

	if resp != nil {
		_ = resp.Body.Close()
	}

	wait(2100 * time.Millisecond)

	resp, err = rl.Get(t.Context(), "/c", nil)
	assertAllowed(t, resp, err)

	if resp != nil {
		_ = resp.Body.Close()
	}

	assert.Equal(t, int64(2), hits.Load())
}

// Different paths share same bucket via custom KeyFunc.
func TestLocalRateLimiter_CustomKey_SharedBucket(t *testing.T) {
	var hits atomic.Int64

	base := newBaseHTTPService(t, &hits)

	rl := NewLocalRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 1,
		Burst:             1,
		KeyFunc:           func(*http.Request) string { return "shared-key" },
	}, base)

	resp, err := rl.Get(t.Context(), "/p1", nil)
	assertAllowed(t, resp, err)

	if resp != nil {
		_ = resp.Body.Close()
	}

	if resp != nil {
		_ = resp.Body.Close()
	}

	resp, err = rl.Get(t.Context(), "/p2", nil)
	require.Nil(t, resp)
	assertRateLimited(t, err)

	if resp != nil {
		_ = resp.Body.Close()
	}

	wait(1100 * time.Millisecond)

	resp, err = rl.Get(t.Context(), "/p3", nil)
	assertAllowed(t, resp, err)

	if resp != nil {
		_ = resp.Body.Close()
	}

	assert.Equal(t, int64(2), hits.Load())
}

// Concurrency: Burst=1 & RPS=1 => only one succeeds immediately.
func TestLocalRateLimiter_Concurrency(t *testing.T) {
	var hits atomic.Int64
	base := newBaseHTTPService(t, &hits)

	rl := NewLocalRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 1,
		Burst:             1,
		KeyFunc:           func(*http.Request) string { return "svc-conc" },
	}, base)

	const workers = 12
	results := make([]error, workers)

	var wg sync.WaitGroup

	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer wg.Done()

			resp, err := rl.Get(context.Background(), "/c", nil)

			if resp != nil {
				_ = resp.Body.Close()
			}

			results[i] = err
		}(i)
	}

	wg.Wait()

	var allowed, denied int

	for _, e := range results {
		if e == nil {
			allowed++
			continue
		}

		var rlErr *RateLimitError

		if errors.As(e, &rlErr) {
			denied++
			continue
		}

		t.Fatalf("unexpected error type: %v", e)
	}

	assert.Equal(t, 1, allowed)
	assert.Equal(t, workers-1, denied)
	assert.Equal(t, int64(1), hits.Load())
}

// buildFullURL behavior for relative and absolute forms.
func TestBuildFullURL(t *testing.T) {
	var hits atomic.Int64

	base := newBaseHTTPService(t, &hits)

	assert.Contains(t, buildFullURL("/x", base), "/x")
	assert.Equal(t, "http://example.com/z", buildFullURL("http://example.com/z", base))
	assert.Contains(t, buildFullURL("rel", base), "/rel")
}

// Ensures metrics calls do not panic when metrics nil (guard path).
func TestLocalRateLimiter_NoMetrics(t *testing.T) {
	var hits atomic.Int64

	base := newBaseHTTPService(t, &hits)

	rl := NewLocalRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 2,
		Burst:             2,
		KeyFunc:           func(*http.Request) string { return "svc-nometrics" },
	}, base)

	resp, err := rl.Get(t.Context(), "/m", nil)
	assertAllowed(t, resp, err)

	if resp != nil {
		_ = resp.Body.Close()
	}
}

// Denial path exposes RateLimitError fields.
func TestLocalRateLimiter_RateLimitErrorFields(t *testing.T) {
	var hits atomic.Int64

	base := newBaseHTTPService(t, &hits)

	rl := NewLocalRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 0, // Always zero refill
		Burst:             1,
		KeyFunc:           func(*http.Request) string { return "svc-zero" },
	}, base)

	resp, err := rl.Get(t.Context(), "/z1", nil)

	assertAllowed(t, resp, err)

	if resp != nil {
		_ = resp.Body.Close()
	}

	resp, err = rl.Get(t.Context(), "/z2", nil)
	require.Nil(t, resp)

	if resp != nil {
		_ = resp.Body.Close()
	}

	var rlErr *RateLimitError

	require.ErrorAs(t, err, &rlErr)

	assert.Equal(t, "svc-zero", rlErr.ServiceKey)
	assert.GreaterOrEqual(t, rlErr.RetryAfter, time.Second)
}

func TestLocalRateLimiter_WrapperMethods_SuccessAndLimited(t *testing.T) {
	var hits atomic.Int64

	base := newBaseHTTPService(t, &hits)

	// Success limiter: plenty of capacity
	successRL := NewLocalRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 100,
		Burst:             100,
		KeyFunc:           func(*http.Request) string { return "wrapper-allow" },
	}, base)

	// Deny limiter: zero capacity (covers error branch)
	denyRL := NewLocalRateLimiter(RateLimiterConfig{
		RequestsPerSecond: 0,
		Burst:             0,
		KeyFunc:           func(*http.Request) string { return "wrapper-deny" },
	}, base)

	tests := []struct {
		name string
		call func(h HTTP) (*http.Response, error)
	}{
		{"Get", func(h HTTP) (*http.Response, error) { return h.Get(t.Context(), "/g", nil) }},
		{"GetWithHeaders", func(h HTTP) (*http.Response, error) {
			return h.GetWithHeaders(t.Context(), "/gh", nil, map[string]string{"X": "1"})
		}},
		{"Post", func(h HTTP) (*http.Response, error) { return h.Post(t.Context(), "/p", nil, []byte("x")) }},
		{"PostWithHeaders", func(h HTTP) (*http.Response, error) {
			return h.PostWithHeaders(t.Context(), "/ph", nil, []byte("x"), map[string]string{"X": "1"})
		}},
		{"Patch", func(h HTTP) (*http.Response, error) { return h.Patch(t.Context(), "/pa", nil, []byte("x")) }},
		{"PatchWithHeaders", func(h HTTP) (*http.Response, error) {
			return h.PatchWithHeaders(t.Context(), "/pah", nil, []byte("x"), map[string]string{"X": "1"})
		}},
		{"Put", func(h HTTP) (*http.Response, error) { return h.Put(t.Context(), "/put", nil, []byte("x")) }},
		{"PutWithHeaders", func(h HTTP) (*http.Response, error) {
			return h.PutWithHeaders(t.Context(), "/puth", nil, []byte("x"), map[string]string{"X": "1"})
		}},
		{"Delete", func(h HTTP) (*http.Response, error) { return h.Delete(t.Context(), "/d", []byte("x")) }},
		{"DeleteWithHeaders", func(h HTTP) (*http.Response, error) {
			return h.DeleteWithHeaders(t.Context(), "/dh", []byte("x"), map[string]string{"X": "1"})
		}},
	}

	// Success path
	for _, tc := range tests {
		t.Run(tc.name+"_Allowed", func(t *testing.T) {
			resp, err := tc.call(successRL)

			assertAllowed(t, resp, err)

			if resp != nil {
				_ = resp.Body.Close()
			}
		})
	}

	// Denied path (each should hit rate limit before underlying service)
	for _, tc := range tests {
		t.Run(tc.name+"_RateLimited", func(t *testing.T) {
			resp, err := tc.call(denyRL)

			require.Error(t, err)
			assert.Nil(t, resp)

			if resp != nil {
				_ = resp.Body.Close()
			}

			var rlErr *RateLimitError

			assert.ErrorAs(t, err, &rlErr)
		})
	}

	// At least all success invocations should have reached downstream.
	assert.Equal(t, int64(len(tests)), hits.Load())
}
