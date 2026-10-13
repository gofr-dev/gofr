package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

var errPolicyConnRefused = errors.New("dial tcp: connection refused")

func TestStandardRetryable(t *testing.T) {
	tests := []struct {
		desc   string
		status int
		err    error
		want   bool
	}{
		{desc: "200", status: http.StatusOK, want: false},
		{desc: "400", status: http.StatusBadRequest, want: false},
		{desc: "404", status: http.StatusNotFound, want: false},
		{desc: "408", status: http.StatusRequestTimeout, want: true},
		{desc: "429", status: http.StatusTooManyRequests, want: true},
		{desc: "500", status: http.StatusInternalServerError, want: false},
		{desc: "501", status: http.StatusNotImplemented, want: false},
		{desc: "502", status: http.StatusBadGateway, want: true},
		{desc: "503", status: http.StatusServiceUnavailable, want: true},
		{desc: "504", status: http.StatusGatewayTimeout, want: true},
		{desc: "505", status: http.StatusHTTPVersionNotSupported, want: false},
		{desc: "network error", err: errPolicyConnRefused, want: true},
		{desc: "rate limit error", err: &RateLimitError{ServiceKey: "svc", RetryAfter: time.Second}, want: true},
		{desc: "wrapped rate limit error", err: fmt.Errorf("outer: %w", &RateLimitError{}), want: true},
		{desc: "circuit open", err: ErrCircuitOpen, want: false},
		{desc: "canceled by an inner layer", err: fmt.Errorf("get: %w", context.Canceled), want: true},
		{desc: "client timeout", err: fmt.Errorf("get: %w", context.DeadlineExceeded), want: true},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var resp *http.Response
			if tc.err == nil {
				resp = &http.Response{StatusCode: tc.status, Body: http.NoBody}
			}

			assert.Equal(t, tc.want, StandardRetryable(resp, tc.err))
		})
	}
}

func TestBackoff_Next(t *testing.T) {
	const samples = 500

	tests := []struct {
		desc    string
		backoff Backoff
		attempt int
		ceiling time.Duration
		jitter  bool
	}{
		{desc: "constant", backoff: ConstantBackoff{Delay: 50 * time.Millisecond}, attempt: 3, ceiling: 50 * time.Millisecond},
		{desc: "constant zero", backoff: ConstantBackoff{}, attempt: 1, ceiling: 0},
		{desc: "exponential first retry", backoff: ExponentialBackoff{Base: 100 * time.Millisecond, Max: time.Second},
			attempt: 1, ceiling: 100 * time.Millisecond, jitter: true},
		{desc: "exponential third retry doubles twice", backoff: ExponentialBackoff{Base: 100 * time.Millisecond, Max: time.Second},
			attempt: 3, ceiling: 400 * time.Millisecond, jitter: true},
		{desc: "exponential capped at Max", backoff: ExponentialBackoff{Base: 100 * time.Millisecond, Max: time.Second},
			attempt: 10, ceiling: time.Second, jitter: true},
		{desc: "exponential custom multiplier", backoff: ExponentialBackoff{Base: 100 * time.Millisecond, Multiplier: 3},
			attempt: 3, ceiling: 900 * time.Millisecond, jitter: true},
		{desc: "exponential huge attempt does not overflow", backoff: ExponentialBackoff{Base: time.Second, Max: time.Minute},
			attempt: 500, ceiling: time.Minute, jitter: true},
		{desc: "exponential zero base", backoff: ExponentialBackoff{Max: time.Second}, attempt: 2, ceiling: 0},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var maxSeen time.Duration

			for range samples {
				d := tc.backoff.Next(tc.attempt)

				require.GreaterOrEqual(t, d, time.Duration(0))
				require.LessOrEqual(t, d, tc.ceiling)

				maxSeen = max(maxSeen, d)
			}

			if !tc.jitter {
				assert.Equal(t, tc.ceiling, maxSeen)
				return
			}

			// Full jitter is uniform over [0, ceiling]; 500 draws that all land in the lower half would
			// mean the ceiling is not the one documented.
			assert.Greater(t, maxSeen, tc.ceiling/2)
		})
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		desc   string
		header string
		want   time.Duration
		wantOK bool
	}{
		{desc: "seconds", header: "1", want: time.Second, wantOK: true},
		{desc: "large seconds", header: "120", want: 2 * time.Minute, wantOK: true},
		{desc: "zero", header: "0", want: 0, wantOK: true},
		{desc: "http date", header: now.Add(30 * time.Second).Format(http.TimeFormat), want: 30 * time.Second, wantOK: true},
		{desc: "past http date", header: now.Add(-time.Minute).Format(http.TimeFormat), want: 0, wantOK: true},
		{desc: "negative seconds", header: "-5"},
		{desc: "garbage", header: "soon"},
		{desc: "empty", header: ""},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			got, ok := parseRetryAfter(tc.header, now)

			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.want, got)
		})
	}
}

// waitRecorder replaces the provider's wait so no test sleeps. onWait runs inside the wait.
type waitRecorder struct {
	mu     sync.Mutex
	waits  []time.Duration
	onWait func()
}

func (w *waitRecorder) wait(ctx context.Context, d time.Duration) error {
	w.mu.Lock()
	w.waits = append(w.waits, d)
	w.mu.Unlock()

	if w.onWait != nil {
		w.onWait()
	}

	return ctx.Err()
}

func (w *waitRecorder) recorded() []time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()

	return append([]time.Duration(nil), w.waits...)
}

// scriptedServer answers hit n (1-based) with script(n, w, r) and counts the hits.
func scriptedServer(t *testing.T, script func(n int32, w http.ResponseWriter, r *http.Request)) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var hits atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		script(hits.Add(1), w, r)
	}))
	t.Cleanup(server.Close)

	return server, &hits
}

// statusSequence answers each hit with the next status of the list, repeating the last one.
func statusSequence(t *testing.T, header http.Header, statuses ...int) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	return scriptedServer(t, func(n int32, w http.ResponseWriter, _ *http.Request) {
		maps.Copy(w.Header(), header)

		status := statuses[min(int(n), len(statuses))-1]
		w.WriteHeader(status)
		_, _ = fmt.Fprintf(w, "hit %d", n)
	})
}

func newPolicyService(t *testing.T, serverURL string, metrics Metrics, policy *RetryPolicy) (HTTP, *waitRecorder) {
	t.Helper()

	svc := NewHTTPService(serverURL, logging.NewMockLogger(logging.DEBUG), metrics,
		WithAttributes(map[string]string{"name": "svc"}), policy)

	rp, ok := svc.(*retryProvider)
	require.True(t, ok, "RetryPolicy must wrap the service in a retryProvider")

	rec := &waitRecorder{}
	rp.wait = rec.wait

	return svc, rec
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()

	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return string(b)
}

func TestRetryPolicy_RetryDecision(t *testing.T) {
	retryAfter1 := http.Header{"Retry-After": {"1"}}
	retryAfter120 := http.Header{"Retry-After": {"120"}}

	tests := []struct {
		desc      string
		policy    RetryPolicy
		header    http.Header
		statuses  []int
		wantCode  int
		wantHits  int32
		wantWaits []time.Duration
	}{
		{desc: "503 then 200", policy: RetryPolicy{MaxAttempts: 3},
			statuses: []int{503, 200}, wantCode: 200, wantHits: 2, wantWaits: []time.Duration{0}},
		{desc: "backoff used for the wait", policy: RetryPolicy{MaxAttempts: 3, Backoff: ConstantBackoff{Delay: 20 * time.Millisecond}},
			statuses: []int{502, 504, 200}, wantCode: 200, wantHits: 3,
			wantWaits: []time.Duration{20 * time.Millisecond, 20 * time.Millisecond}},
		{desc: "MaxAttempts reached returns last response", policy: RetryPolicy{MaxAttempts: 3},
			statuses: []int{503}, wantCode: 503, wantHits: 3, wantWaits: []time.Duration{0, 0}},
		{desc: "MaxAttempts 1 never retries", policy: RetryPolicy{MaxAttempts: 1},
			statuses: []int{503}, wantCode: 503, wantHits: 1},
		{desc: "500 is not retried by the standard rule", policy: RetryPolicy{MaxAttempts: 3},
			statuses: []int{500, 200}, wantCode: 500, wantHits: 1},
		{desc: "429 Retry-After honored over backoff",
			policy: RetryPolicy{MaxAttempts: 2, RespectRetryAfter: true, Backoff: ConstantBackoff{Delay: time.Millisecond}},
			header: retryAfter1, statuses: []int{429, 200}, wantCode: 200, wantHits: 2, wantWaits: []time.Duration{time.Second}},
		{desc: "Retry-After ignored unless RespectRetryAfter",
			policy: RetryPolicy{MaxAttempts: 2, Backoff: ConstantBackoff{Delay: time.Millisecond}},
			header: retryAfter1, statuses: []int{429, 200}, wantCode: 200, wantHits: 2, wantWaits: []time.Duration{time.Millisecond}},
		{desc: "Retry-After capped at the 30s default", policy: RetryPolicy{MaxAttempts: 2, RespectRetryAfter: true},
			header: retryAfter120, statuses: []int{503, 200}, wantCode: 200, wantHits: 2, wantWaits: []time.Duration{30 * time.Second}},
		{desc: "Retry-After capped at MaxRetryAfter",
			policy: RetryPolicy{MaxAttempts: 2, RespectRetryAfter: true, MaxRetryAfter: 5 * time.Second},
			header: retryAfter120, statuses: []int{503, 200}, wantCode: 200, wantHits: 2, wantWaits: []time.Duration{5 * time.Second}},
		{desc: "custom RetryOn overrides the standard rule",
			policy: RetryPolicy{MaxAttempts: 3, RetryOn: func(r *http.Response, _ error) bool {
				return r != nil && r.StatusCode == http.StatusInternalServerError
			}},
			statuses: []int{500, 503, 200}, wantCode: 503, wantHits: 2, wantWaits: []time.Duration{0}},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			server, hits := statusSequence(t, tc.header, tc.statuses...)
			svc, rec := newPolicyService(t, server.URL, nil, &tc.policy)

			resp, err := svc.Get(t.Context(), "test", nil)
			require.NoError(t, err)

			assert.Equal(t, tc.wantCode, resp.StatusCode)
			assert.Equal(t, fmt.Sprintf("hit %d", tc.wantHits), readBody(t, resp), "the returned body must be the last attempt's, unread")
			assert.Equal(t, tc.wantHits, hits.Load())
			assert.Equal(t, tc.wantWaits, rec.recorded())
		})
	}
}

func TestRetryPolicy_Methods(t *testing.T) {
	type call func(ctx context.Context, svc HTTP) (*http.Response, error)

	tests := []struct {
		desc     string
		methods  []string
		keyName  string
		call     call
		wantHits int32
	}{
		{desc: "GET retried", wantHits: 2, call: func(ctx context.Context, s HTTP) (*http.Response, error) {
			return s.Get(ctx, "x", nil)
		}},
		{desc: "PUT retried", wantHits: 2, call: func(ctx context.Context, s HTTP) (*http.Response, error) {
			return s.Put(ctx, "x", nil, []byte(`{}`))
		}},
		{desc: "DELETE retried", wantHits: 2, call: func(ctx context.Context, s HTTP) (*http.Response, error) {
			return s.Delete(ctx, "x", nil)
		}},
		{desc: "QUERY retried", wantHits: 2, call: func(ctx context.Context, s HTTP) (*http.Response, error) {
			return s.Query(ctx, "x", nil, []byte(`{}`))
		}},
		{desc: "POST without key sent once", wantHits: 1, call: func(ctx context.Context, s HTTP) (*http.Response, error) {
			return s.Post(ctx, "x", nil, []byte(`{}`))
		}},
		{desc: "PATCH without key sent once", wantHits: 1, call: func(ctx context.Context, s HTTP) (*http.Response, error) {
			return s.PatchWithHeaders(ctx, "x", nil, []byte(`{}`), map[string]string{"X-Other": "1"})
		}},
		{desc: "POST with key retried", wantHits: 2, call: func(ctx context.Context, s HTTP) (*http.Response, error) {
			return s.PostWithHeaders(ctx, "x", nil, []byte(`{}`), map[string]string{"Idempotency-Key": "k1"})
		}},
		{desc: "POST key matched case-insensitively", wantHits: 2, call: func(ctx context.Context, s HTTP) (*http.Response, error) {
			return s.PostWithHeaders(ctx, "x", nil, []byte(`{}`), map[string]string{"idempotency-key": "k1"})
		}},
		{desc: "POST with empty key sent once", wantHits: 1, call: func(ctx context.Context, s HTTP) (*http.Response, error) {
			return s.PostWithHeaders(ctx, "x", nil, []byte(`{}`), map[string]string{"Idempotency-Key": ""})
		}},
		{desc: "PATCH with custom key header retried", keyName: "X-Request-Id", wantHits: 2,
			call: func(ctx context.Context, s HTTP) (*http.Response, error) {
				return s.PatchWithHeaders(ctx, "x", nil, []byte(`{}`), map[string]string{"x-request-id": "k1"})
			}},
		{desc: "custom Methods excludes GET", methods: []string{http.MethodPut}, wantHits: 1,
			call: func(ctx context.Context, s HTTP) (*http.Response, error) {
				return s.GetWithHeaders(ctx, "x", nil, nil)
			}},
		{desc: "custom Methods includes POST", methods: []string{"post"}, wantHits: 2,
			call: func(ctx context.Context, s HTTP) (*http.Response, error) {
				return s.Post(ctx, "x", nil, []byte(`{}`))
			}},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			server, hits := statusSequence(t, nil, http.StatusServiceUnavailable, http.StatusOK)
			svc, _ := newPolicyService(t, server.URL, nil,
				&RetryPolicy{MaxAttempts: 2, Methods: tc.methods, IdempotencyKeyHeader: tc.keyName})

			resp, err := tc.call(t.Context(), svc)
			require.NoError(t, err)

			_ = readBody(t, resp)

			assert.Equal(t, tc.wantHits, hits.Load())
		})
	}
}

func TestRetryPolicy_PerAttemptTimeout(t *testing.T) {
	const body = "slow but complete body"

	server, hits := scriptedServer(t, func(n int32, w http.ResponseWriter, r *http.Request) {
		if n == 1 { // hangs until the attempt is abandoned
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}

			return
		}

		// Headers arrive at once; the body takes longer than PerAttemptTimeout, which must not cut it off.
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(300 * time.Millisecond)

		_, _ = io.WriteString(w, body)
	})

	svc, rec := newPolicyService(t, server.URL, nil,
		&RetryPolicy{MaxAttempts: 2, PerAttemptTimeout: 100 * time.Millisecond})

	start := time.Now()
	resp, err := svc.Get(t.Context(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, body, readBody(t, resp))
	assert.Equal(t, int32(2), hits.Load())
	assert.Len(t, rec.recorded(), 1)
	assert.Less(t, time.Since(start), 3*time.Second, "the hung attempt must be abandoned, not waited out")
}

func TestRetryPolicy_PerAttemptTimeout_LastAttemptError(t *testing.T) {
	server, _ := scriptedServer(t, func(_ int32, _ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})

	// The timed-out attempt could be retried, so it is bounded; RetryOn then declines the retry.
	svc, _ := newPolicyService(t, server.URL, nil, &RetryPolicy{MaxAttempts: 2, PerAttemptTimeout: 50 * time.Millisecond,
		RetryOn: func(*http.Response, error) bool { return false }})

	resp, err := svc.Get(t.Context(), "test", nil)

	requireNoResponse(t, resp)
	require.ErrorIs(t, err, errAttemptTimeout)
	require.NotErrorIs(t, err, context.Canceled, "the caller's context was never canceled")

	var urlErr *url.Error
	require.ErrorAs(t, err, &urlErr, "the transport error naming the request must be kept")
}

func TestRetryPolicy_PerAttemptTimeout_NotOnTerminalAttempt(t *testing.T) {
	const slow = 200 * time.Millisecond

	get := func(ctx context.Context, svc HTTP) (*http.Response, error) { return svc.Get(ctx, "test", nil) }
	post := func(ctx context.Context, svc HTTP) (*http.Response, error) {
		return svc.Post(ctx, "test", nil, []byte("{}"))
	}

	tests := []struct {
		desc     string
		policy   RetryPolicy
		call     func(context.Context, HTTP) (*http.Response, error)
		wantHits int32
	}{
		{desc: "a POST without an idempotency key is never retried, so it is not cut off",
			policy: RetryPolicy{MaxAttempts: 3, PerAttemptTimeout: 50 * time.Millisecond}, call: post, wantHits: 1},
		{desc: "a single-attempt policy is not cut off",
			policy: RetryPolicy{MaxAttempts: 1, PerAttemptTimeout: 50 * time.Millisecond}, call: get, wantHits: 1},
		{desc: "the last attempt is not cut off",
			policy: RetryPolicy{MaxAttempts: 2, PerAttemptTimeout: 50 * time.Millisecond}, call: get, wantHits: 2},
	}

	for i, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			// Every attempt answers after slow, well past PerAttemptTimeout; only a bounded attempt is cut off.
			server, hits := scriptedServer(t, func(_ int32, w http.ResponseWriter, r *http.Request) {
				select {
				case <-r.Context().Done():
					return
				case <-time.After(slow):
				}

				_, _ = io.WriteString(w, "done")
			})

			svc, _ := newPolicyService(t, server.URL, nil, &tc.policy)

			resp, err := tc.call(t.Context(), svc)

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, "done", readBody(t, resp), "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.wantHits, hits.Load(), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestRetryPolicy_PerAttemptTimeout_CountedByInnerCircuitBreaker(t *testing.T) {
	server, hits := scriptedServer(t, func(_ int32, _ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	})

	// Breaker registered first, so it sits inside the retries, as the docs recommend.
	svc := NewHTTPService(server.URL, logging.NewMockLogger(logging.ERROR), nil,
		&CircuitBreakerConfig{Threshold: 1, Interval: time.Hour},
		&RetryPolicy{MaxAttempts: 5, PerAttemptTimeout: 50 * time.Millisecond})
	svc.(*retryProvider).wait = (&waitRecorder{}).wait

	resp, err := svc.Get(t.Context(), "test", nil)

	requireNoResponse(t, resp)
	require.ErrorIs(t, err, ErrCircuitOpen, "timed-out attempts must count as failures and open the breaker")
	assert.Equal(t, int32(2), hits.Load(), "threshold 1 opens on the second failure; no attempt follows")
}

func TestRetryPolicy_CanceledWaitSpendsNoBudget(t *testing.T) {
	server, hits := statusSequence(t, nil, http.StatusServiceUnavailable)

	// The floor allows exactly one retry in the window.
	svc, rec := newPolicyService(t, server.URL, nil,
		&RetryPolicy{MaxAttempts: 2, Budget: &RetryBudget{MinPerSecond: 1, TTL: time.Second}})

	// First call: canceled during the wait, so no retry is made.
	ctx, cancel := context.WithCancel(t.Context())
	rec.onWait = cancel

	resp, err := svc.Get(ctx, "test", nil)
	require.NoError(t, err)

	_ = readBody(t, resp)

	require.Equal(t, int32(1), hits.Load())

	// Second call: the one retry must still be available.
	rec.onWait = nil

	resp, err = svc.Get(t.Context(), "test", nil)
	require.NoError(t, err)

	_ = readBody(t, resp)

	assert.Equal(t, int32(3), hits.Load(), "a canceled wait must not spend the budget")
}

func TestDetachBody(t *testing.T) {
	small := "busy"
	large := strings.Repeat("x", discardedBodyLimit*3)

	tests := []struct {
		desc string
		body string
	}{
		{desc: "small body buffered", body: small},
		{desc: "empty body", body: ""},
		{desc: "large body kept whole", body: large},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			closed := &closeCounter{Reader: strings.NewReader(tc.body)}
			resp := &http.Response{StatusCode: http.StatusServiceUnavailable, Body: closed}

			detachBody(resp)

			if len(tc.body) <= discardedBodyLimit {
				assert.Equal(t, int32(1), closed.closes.Load(), "a small body's connection must be released")
			}

			assert.Equal(t, tc.body, readBody(t, resp))
		})
	}
}

type closeCounter struct {
	io.Reader
	closes atomic.Int32
}

func (c *closeCounter) Close() error {
	c.closes.Add(1)
	return nil
}

func TestRetryPolicy_ContextEnds(t *testing.T) {
	tests := []struct {
		desc     string
		timeout  time.Duration
		cancelIn bool
		wantWait int
	}{
		{desc: "deadline before the wait ends", timeout: 2 * time.Second, wantWait: 0},
		{desc: "context canceled during the wait", cancelIn: true, wantWait: 1},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			server, hits := statusSequence(t, http.Header{"Retry-After": {"10"}}, http.StatusServiceUnavailable, http.StatusOK)
			svc, rec := newPolicyService(t, server.URL, nil, &RetryPolicy{MaxAttempts: 3, RespectRetryAfter: true})

			ctx, cancel := context.WithCancel(t.Context())
			if tc.timeout > 0 {
				ctx, cancel = context.WithTimeout(t.Context(), tc.timeout)
			}

			defer cancel()

			if tc.cancelIn {
				rec.onWait = cancel
			}

			resp, err := svc.Get(ctx, "test", nil)
			require.NoError(t, err)

			assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
			assert.Equal(t, "hit 1", readBody(t, resp), "the last response must be returned undrained")
			assert.Equal(t, int32(1), hits.Load())
			assert.Len(t, rec.recorded(), tc.wantWait)
		})
	}
}

func TestRetryPolicy_NoRetryWithoutTimeForAnAttempt(t *testing.T) {
	tests := []struct {
		desc       string
		deadline   time.Duration
		attemptMax time.Duration
		wantCode   int
		wantHits   int32
	}{
		{desc: "less than PerAttemptTimeout left: last response returned",
			deadline: 500 * time.Millisecond, attemptMax: 2 * time.Second, wantCode: http.StatusServiceUnavailable, wantHits: 1},
		{desc: "a full attempt still fits: retried",
			deadline: 2 * time.Second, attemptMax: 100 * time.Millisecond, wantCode: http.StatusOK, wantHits: 2},
		{desc: "no PerAttemptTimeout: only the wait is checked",
			deadline: 500 * time.Millisecond, wantCode: http.StatusOK, wantHits: 2},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			server, hits := statusSequence(t, nil, http.StatusServiceUnavailable, http.StatusOK)
			svc, _ := newPolicyService(t, server.URL, nil,
				&RetryPolicy{MaxAttempts: 3, PerAttemptTimeout: tc.attemptMax})

			ctx, cancel := context.WithTimeout(t.Context(), tc.deadline)
			defer cancel()

			resp, err := svc.Get(ctx, "test", nil)
			require.NoError(t, err)

			assert.Equal(t, tc.wantCode, resp.StatusCode)
			assert.Equal(t, fmt.Sprintf("hit %d", tc.wantHits), readBody(t, resp))
			assert.Equal(t, tc.wantHits, hits.Load())
		})
	}
}

func TestWaitWithContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	start := time.Now()

	require.ErrorIs(t, waitWithContext(ctx, time.Hour), context.Canceled)
	assert.Less(t, time.Since(start), time.Second, "a canceled context must not wait out the timer")

	require.NoError(t, waitWithContext(t.Context(), time.Millisecond))
	require.NoError(t, waitWithContext(t.Context(), 0))
}

// fixedErrHTTP answers every GET with err and counts the calls.
type fixedErrHTTP struct {
	mockHTTP
	calls atomic.Int32
	err   error
}

func (f *fixedErrHTTP) Get(_ context.Context, _ string, _ map[string]any) (*http.Response, error) {
	f.calls.Add(1)
	return nil, f.err
}

func TestRetryPolicy_NeverRetried(t *testing.T) {
	always := func(*http.Response, error) bool { return true }

	tests := []struct {
		desc      string
		err       error
		wantCalls int32
	}{
		{desc: "circuit open, even with RetryOn always true", err: ErrCircuitOpen, wantCalls: 1},
		{desc: "wrapped circuit open", err: fmt.Errorf("cb: %w", ErrCircuitOpen), wantCalls: 1},
		{desc: "network error is retried", err: errPolicyConnRefused, wantCalls: 3},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			inner := &fixedErrHTTP{err: tc.err}
			svc := (&RetryPolicy{MaxAttempts: 3, RetryOn: always}).AddOption(inner)
			svc.(*retryProvider).wait = (&waitRecorder{}).wait

			resp, err := svc.Get(t.Context(), "x", nil)

			requireNoResponse(t, resp)
			require.ErrorIs(t, err, tc.err)
			assert.Equal(t, tc.wantCalls, inner.calls.Load())
		})
	}
}

func TestRetryPolicy_RateLimitErrorRetryAfter(t *testing.T) {
	inner := &fixedErrHTTP{err: &RateLimitError{ServiceKey: "svc", RetryAfter: 2 * time.Second}}
	svc := (&RetryPolicy{MaxAttempts: 2, RespectRetryAfter: true}).AddOption(inner)
	rec := &waitRecorder{}
	svc.(*retryProvider).wait = rec.wait

	resp, err := svc.Get(t.Context(), "x", nil)

	requireNoResponse(t, resp)

	var rle *RateLimitError
	require.ErrorAs(t, err, &rle)
	assert.Equal(t, int32(2), inner.calls.Load())
	assert.Equal(t, []time.Duration{2 * time.Second}, rec.recorded())
}

func TestRetryPolicy_Budget(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)

	metrics.EXPECT().RecordHistogram(gomock.Any(), "app_http_service_response", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any()).AnyTimes()
	metrics.EXPECT().IncrementCounter(gomock.Any(), "app_http_retry_budget_exhausted_total", "service", "svc").Times(1)

	server, hits := statusSequence(t, nil, http.StatusServiceUnavailable)
	svc, rec := newPolicyService(t, server.URL, metrics,
		&RetryPolicy{MaxAttempts: 3, Budget: &RetryBudget{Ratio: 0.5}}) // 1 retry needs 2 first attempts

	resp, err := svc.Get(t.Context(), "test", nil)
	require.NoError(t, err)

	assert.Equal(t, "hit 1", readBody(t, resp))
	assert.Equal(t, int32(1), hits.Load(), "an exhausted budget must stop the retry")
	assert.Empty(t, rec.recorded())
}

func TestRetryPolicy_Observability(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)

	metrics.EXPECT().RecordHistogram(gomock.Any(), "app_http_service_response", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any()).AnyTimes()
	metrics.EXPECT().IncrementCounter(gomock.Any(), "app_http_retry_count", "service", "svc", "reason", "status_5xx").Times(1)
	metrics.EXPECT().IncrementCounter(gomock.Any(), "app_http_retry_count", "service", "svc", "reason", "status_429").Times(1)
	metrics.EXPECT().RecordHistogram(gomock.Any(), "app_http_retry_wait_seconds", 0.01, "service", "svc").Times(2)

	server, _ := statusSequence(t, nil, http.StatusBadGateway, http.StatusTooManyRequests, http.StatusOK)

	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	ctx, span := tp.Tracer("test").Start(t.Context(), "caller")

	var resp *http.Response

	// The mock logger writes to the stdout it saw when it was built, so the service is built inside.
	logs := testutil.StdoutOutputForFunc(func() {
		svc, _ := newPolicyService(t, server.URL, metrics,
			&RetryPolicy{MaxAttempts: 3, Backoff: ConstantBackoff{Delay: 10 * time.Millisecond}})

		var err error

		resp, err = svc.Get(ctx, "test", nil) //nolint:bodyclose // Closed by readBody below.
		require.NoError(t, err)
	})

	span.End()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = readBody(t, resp)

	var events []string

	for _, s := range recorder.Ended() {
		if s.Name() != "caller" {
			continue
		}

		for _, e := range s.Events() {
			if e.Name != "http.retry" {
				continue
			}

			attrs := map[string]string{}
			for _, kv := range e.Attributes {
				attrs[string(kv.Key)] = kv.Value.String()
			}

			events = append(events, attrs["http.request.resend_count"]+" "+attrs["retry.reason"]+" "+attrs["retry.wait_seconds"])
		}
	}

	assert.Equal(t, []string{"1 status_5xx 0.01", "2 status_429 0.01"}, events)
	assert.Equal(t, 2, strings.Count(logs, "retrying"), "each retry is logged at debug level: %s", logs)
}
