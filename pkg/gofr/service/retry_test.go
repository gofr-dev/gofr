package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
)

type mockHTTP struct{}

func (*mockHTTP) HealthCheck(_ context.Context) *Health {
	return &Health{
		Status:  "UP",
		Details: map[string]any{"host": "http://test.com"},
	}
}

func (*mockHTTP) getHealthResponseForEndpoint(_ context.Context, _ string, _ int) *Health {
	return &Health{
		Status:  "UP",
		Details: map[string]any{"host": "http://test.com"},
	}
}

func (*mockHTTP) Get(_ context.Context, _ string, _ map[string]any) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func (*mockHTTP) GetWithHeaders(_ context.Context, _ string, _ map[string]any, _ map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func (*mockHTTP) Post(_ context.Context, _ string, _ map[string]any, _ []byte) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusCreated, Body: http.NoBody}, nil
}

func (*mockHTTP) PostWithHeaders(_ context.Context, _ string, _ map[string]any, _ []byte,
	_ map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusCreated, Body: http.NoBody}, nil
}

func (*mockHTTP) Put(_ context.Context, _ string, _ map[string]any, _ []byte) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func (*mockHTTP) PutWithHeaders(_ context.Context, _ string, _ map[string]any, _ []byte,
	_ map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func (*mockHTTP) Patch(_ context.Context, _ string, _ map[string]any, _ []byte) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func (*mockHTTP) PatchWithHeaders(_ context.Context, _ string, _ map[string]any, _ []byte,
	_ map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func (*mockHTTP) Delete(_ context.Context, _ string, _ []byte) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
}

func (*mockHTTP) DeleteWithHeaders(_ context.Context, _ string, _ []byte, _ map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
}

// Query/QueryWithHeaders stamp X-Mock-Method so a test can prove the decorator
// delegated to Query and not to some other verb whose mock also returns 200.
func (*mockHTTP) Query(_ context.Context, _ string, _ map[string]any, _ []byte) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody,
		Header: http.Header{"X-Mock-Method": []string{methodQuery}}}, nil
}

func (*mockHTTP) QueryWithHeaders(_ context.Context, _ string, _ map[string]any, _ []byte,
	_ map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody,
		Header: http.Header{"X-Mock-Method": []string{methodQuery}}}, nil
}

// Helper to create a retry HTTP instance.
func newRetryHTTP() HTTP {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}

	return retryConfig.AddOption(mockHTTP)
}

func TestRetryProvider_Get(t *testing.T) {
	retryHTTP := newRetryHTTP()

	// Make the GET request
	resp, err := retryHTTP.Get(t.Context(), "/test", nil)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryProvider_GetWithHeaders(t *testing.T) {
	retryHTTP := newRetryHTTP()

	// Make the GET request with headers
	resp, err := retryHTTP.GetWithHeaders(t.Context(), "/test", nil,
		map[string]string{"Content-Type": "application/json"})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryProvider_Post(t *testing.T) {
	retryHTTP := newRetryHTTP()

	// Make the POST request
	resp, err := retryHTTP.Post(t.Context(), "/test", nil, []byte("body"))
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestRetryProvider_PostWithHeaders(t *testing.T) {
	retryHTTP := newRetryHTTP()

	// Make the POST request with headers
	resp, err := retryHTTP.PostWithHeaders(t.Context(), "/test", nil, []byte("body"),
		map[string]string{"Content-Type": "application/json"})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestRetryProvider_Query(t *testing.T) {
	retryHTTP := newRetryHTTP()

	resp, err := retryHTTP.Query(t.Context(), "/test", nil, []byte("body"))
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, methodQuery, resp.Header.Get("X-Mock-Method"), "retry must delegate to Query, not another verb")
}

func TestRetryProvider_QueryWithHeaders(t *testing.T) {
	retryHTTP := newRetryHTTP()

	resp, err := retryHTTP.QueryWithHeaders(t.Context(), "/test", nil, []byte("body"),
		map[string]string{"Content-Type": "application/json"})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, methodQuery, resp.Header.Get("X-Mock-Method"), "retry must delegate to QueryWithHeaders, not another verb")
}

func TestRetryProvider_Put(t *testing.T) {
	retryHTTP := newRetryHTTP()

	// Make the PUT request
	resp, err := retryHTTP.Put(t.Context(), "/test", nil, []byte("body"))
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryProvider_PutWithHeaders(t *testing.T) {
	retryHTTP := newRetryHTTP()

	// Make the PUT request with headers
	resp, err := retryHTTP.PutWithHeaders(t.Context(), "/test", nil, []byte("body"),
		map[string]string{"Content-Type": "application/json"})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryProvider_Patch_WithError(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkAuthHeaders(t, r)
		assert.Equal(t, http.MethodPatch, r.Method)

		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	// Create a new HTTP service instance with basic auth
	httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&RetryConfig{MaxRetries: 5})

	// Make the PATCH request
	resp, err := httpService.Patch(t.Context(), "/test", nil, []byte("body"))
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestRetryProvider_PatchWithHeaders(t *testing.T) {
	retryHTTP := newRetryHTTP()

	// Make the PATCH request with headers
	resp, err := retryHTTP.PatchWithHeaders(t.Context(), "/test", nil, []byte("body"),
		map[string]string{"Content-Type": "application/json"})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryProvider_Delete(t *testing.T) {
	retryHTTP := newRetryHTTP()

	// Make the DELETE request
	resp, err := retryHTTP.Delete(t.Context(), "/test", nil)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
func TestRetryProvider_DeleteWithHeaders(t *testing.T) {
	retryHTTP := newRetryHTTP()

	// Make the DELETE request with headers
	resp, err := retryHTTP.DeleteWithHeaders(t.Context(), "/test", []byte("body"),
		map[string]string{"Content-Type": "application/json"})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestRetryProvider_Metrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)

	// Expect NewCounter and IncrementCounter to be called with labels
	metrics.EXPECT().NewCounter("app_http_retry_count", gomock.Any()).AnyTimes()
	metrics.EXPECT().IncrementCounter(gomock.Any(), "app_http_retry_count", "service", "test-service").MinTimes(1)
	metrics.EXPECT().RecordHistogram(gomock.Any(), "app_http_service_response", gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any()).AnyTimes()

	// Create a mock HTTP server that fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	// Create a new HTTP service instance with retry config, metrics and name
	httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), metrics,
		WithAttributes(map[string]string{"name": "test-service"}), &RetryConfig{MaxRetries: 2})

	// Make the request
	resp, err := httpService.Get(t.Context(), "/test", nil)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

var errProbeConnRefused = errors.New("dial tcp: connection refused")

// attemptProbe sits between the retry layer and the HTTP service. It counts the attempts retry makes
// and the response bodies retry closes, can replace an attempt's outcome with an error, and runs a
// hook once an attempt has returned.
type attemptProbe struct {
	attempts atomic.Int32
	closed   atomic.Int32

	fail  func(attempt int32) error
	after func(attempt int32)
}

func (p *attemptProbe) AddOption(h HTTP) HTTP {
	return &probedHTTP{HTTP: h, probe: p}
}

type probedHTTP struct {
	HTTP
	probe *attemptProbe
}

func (p *probedHTTP) Unwrap() HTTP { return p.HTTP }

func (p *probedHTTP) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response, error) {
	attempt := p.probe.attempts.Add(1)

	defer func() {
		if p.probe.after != nil {
			p.probe.after(attempt)
		}
	}()

	if p.probe.fail != nil {
		if err := p.probe.fail(attempt); err != nil {
			return nil, err
		}
	}

	resp, err := p.HTTP.Get(ctx, path, queryParams)
	if resp != nil && resp.Body != nil {
		resp.Body = &closeCountingBody{ReadCloser: resp.Body, closed: &p.probe.closed}
	}

	return resp, err
}

type closeCountingBody struct {
	io.ReadCloser
	closed *atomic.Int32
}

func (b *closeCountingBody) Close() error {
	b.closed.Add(1)

	return b.ReadCloser.Close()
}

// countingServer answers every request with 503 Service Unavailable and counts the requests that reached it.
func countingServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var hits atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	return server, &hits
}

// newProbedRetryService builds retry(probe(httpService)) against url, allowing 3 retries: 4 attempts in all.
func newProbedRetryService(url string, probe *attemptProbe) HTTP {
	return NewHTTPService(url, logging.NewMockLogger(logging.INFO), nil, probe, &RetryConfig{MaxRetries: 3})
}

// requireNoResponse fails the test if a response came back, closing it first.
func requireNoResponse(t *testing.T, resp *http.Response) {
	t.Helper()

	if resp != nil {
		resp.Body.Close()
	}

	require.Nil(t, resp)
}

// singleAttemptErr is the error one un-retried call returns for ctx: the result every extra
// attempt made with a done context repeats.
func singleAttemptErr(ctx context.Context, t *testing.T, url string) error {
	t.Helper()

	resp, err := NewHTTPService(url, logging.NewMockLogger(logging.INFO), nil).Get(ctx, "test", nil)
	requireNoResponse(t, resp)
	require.Error(t, err)

	return err
}

func TestRetryProvider_ContextDoneBeforeCall_MakesOneAttempt(t *testing.T) {
	server, hits := countingServer(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	probe := &attemptProbe{}

	resp, err := newProbedRetryService(server.URL, probe).Get(ctx, "test", nil)

	requireNoResponse(t, resp)
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, singleAttemptErr(ctx, t, server.URL).Error(), err.Error(), "caller must see the same error as today")
	assert.Equal(t, int32(1), probe.attempts.Load(), "a done context must not be retried")
	assert.Equal(t, int32(0), hits.Load())
}

func TestRetryProvider_ContextDoneInFlight_StopsAfterNextAttempt(t *testing.T) {
	tests := []struct {
		desc     string
		deadline bool
		wantErr  error
	}{
		{desc: "canceled while attempt 1 is on the wire", wantErr: context.Canceled},
		{desc: "deadline passes while attempt 1 is on the wire", deadline: true, wantErr: context.DeadlineExceeded},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			if tc.deadline {
				ctx, cancel = context.WithTimeout(t.Context(), 100*time.Millisecond)
			}

			defer cancel()

			var hits atomic.Int32

			// The handler never answers: attempt 1 ends only when the client's context does.
			server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				hits.Add(1)

				if !tc.deadline {
					cancel()
				}

				<-r.Context().Done()
			}))
			defer server.Close()

			probe := &attemptProbe{}

			resp, err := newProbedRetryService(server.URL, probe).Get(ctx, "test", nil)

			requireNoResponse(t, resp)
			require.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, singleAttemptErr(ctx, t, server.URL).Error(), err.Error(), "caller must see the same error as today")
			assert.Equal(t, int32(2), probe.attempts.Load(), "the attempt made after the context ended must be the last")
			assert.Equal(t, int32(1), hits.Load())
		})
	}
}

func TestRetryProvider_ContextDoneBetweenAttempts_StopsAfterNextAttempt(t *testing.T) {
	tests := []struct {
		desc       string
		deadline   bool
		failFirst  bool
		wantErr    error
		wantHits   int32
		wantClosed int32
	}{
		{desc: "503 then cancel", wantErr: context.Canceled, wantHits: 1, wantClosed: 1},
		{desc: "503 then deadline", deadline: true, wantErr: context.DeadlineExceeded, wantHits: 1, wantClosed: 1},
		{desc: "connection error then cancel", failFirst: true, wantErr: context.Canceled},
		{desc: "connection error then deadline", deadline: true, failFirst: true, wantErr: context.DeadlineExceeded},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			server, hits := countingServer(t)

			ctx, cancel := context.WithCancel(t.Context())
			if tc.deadline {
				ctx, cancel = context.WithTimeout(t.Context(), 500*time.Millisecond)
			}

			defer cancel()

			probe := &attemptProbe{
				after: func(attempt int32) {
					if attempt != 1 {
						return
					}

					if !tc.deadline {
						cancel()
					}

					<-ctx.Done()
				},
			}

			if tc.failFirst {
				probe.fail = func(attempt int32) error {
					if attempt == 1 {
						return errProbeConnRefused
					}

					return nil
				}
			}

			resp, err := newProbedRetryService(server.URL, probe).Get(ctx, "test", nil)

			requireNoResponse(t, resp)
			require.ErrorIs(t, err, tc.wantErr)
			assert.Equal(t, singleAttemptErr(ctx, t, server.URL).Error(), err.Error(), "caller must see the same error as today")
			assert.Equal(t, int32(2), probe.attempts.Load(), "the attempt made after the context ended must be the last")
			assert.Equal(t, tc.wantHits, hits.Load())
			assert.Equal(t, tc.wantClosed, probe.closed.Load(), "a discarded response must be closed")
		})
	}
}

func TestRetryProvider_LiveContext_RetriesEveryAttemptAndClosesDiscardedBodies(t *testing.T) {
	server, hits := countingServer(t)

	probe := &attemptProbe{}

	resp, err := newProbedRetryService(server.URL, probe).Get(t.Context(), "test", nil)
	require.NoError(t, err)

	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, int32(4), probe.attempts.Load())
	assert.Equal(t, int32(4), hits.Load())
	assert.Equal(t, int32(3), probe.closed.Load(), "the three discarded responses must be closed, the returned one left open")

	require.NoError(t, resp.Body.Close())
	assert.Equal(t, int32(4), probe.closed.Load())
}

func TestRetryProvider_ConcurrentCallers_StopPerCallerContext(t *testing.T) {
	const callers = 16

	server, hits := countingServer(t)

	probe := &attemptProbe{}
	svc := newProbedRetryService(server.URL, probe)

	canceled, cancel := context.WithCancel(t.Context())
	cancel()

	var wg sync.WaitGroup

	for i := range callers {
		ctx := t.Context()
		if i%2 == 0 {
			ctx = canceled
		}

		wg.Go(func() {
			resp, err := svc.Get(ctx, "test", nil)
			if err != nil {
				assert.ErrorIs(t, err, context.Canceled)

				return
			}

			assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
			assert.NoError(t, resp.Body.Close())
		})
	}

	wg.Wait()

	// 8 canceled callers make 1 attempt each; 8 live callers make all 4 and close 3 discarded bodies each,
	// then close the returned one.
	assert.Equal(t, int32(callers/2*1+callers/2*4), probe.attempts.Load())
	assert.Equal(t, int32(callers/2*4), hits.Load())
	assert.Equal(t, int32(callers/2*4), probe.closed.Load())
}
