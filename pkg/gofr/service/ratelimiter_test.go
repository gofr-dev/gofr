package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/logging"
)

func testServerRL() *httptest.Server {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return httptest.NewServer(h)
}

func setupHTTPServiceTestServerForRateLimiter() (*httptest.Server, HTTP) {
	server := testServerRL()

	service := httpService{
		Client: &http.Client{Transport: &customTransportRL{}},
		url:    server.URL,
		Tracer: otel.Tracer("gofr-http-client"),
		Logger: logging.NewMockLogger(logging.DEBUG),
	}

	rlConfig := RateLimiterConfig{
		Limit:    2,
		Duration: time.Second,
		MaxQueue: 5,
	}

	httpservice := rlConfig.AddOption(&service)

	return server, httpservice
}

func TestRateLimiter_GetSuccessRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForRateLimiter()
	defer server.Close()

	resp, err := service.Get(context.Background(), "test", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestRateLimiter_GetWithHeadersSuccessRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForRateLimiter()
	defer server.Close()

	headers := map[string]string{"Content-Type": "application/json"}
	resp, err := service.GetWithHeaders(context.Background(), "test", nil, headers)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestRateLimiter_RateLimitingBehavior(t *testing.T) {
	server, service := setupHTTPServiceTestServerForRateLimiter()
	defer server.Close()

	start := time.Now()

	for i := 0; i < 3; i++ {
		resp, err := service.Get(context.Background(), "test", nil)
		require.NoError(t, err)
		require.NotNil(t, resp)
		_ = resp.Body.Close()
	}

	duration := time.Since(start)

	assert.GreaterOrEqual(t, duration.Milliseconds(), int64(450),
		"Expected third request to be delayed by rate limiting, but completed too quickly: %v", duration)

	assert.LessOrEqual(t, duration.Milliseconds(), int64(750),
		"Requests took longer than expected: %v", duration)
}

func TestRateLimiter_PostSuccessRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForRateLimiter()
	defer server.Close()

	resp, err := service.Post(context.Background(), "test", nil, []byte("test"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestRateLimiter_PostWithHeadersSuccessRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForRateLimiter()
	defer server.Close()

	headers := map[string]string{"Content-Type": "application/json"}
	resp, err := service.PostWithHeaders(context.Background(), "test", nil, []byte("test"), headers)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestRateLimiter_PutSuccessRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForRateLimiter()
	defer server.Close()

	resp, err := service.Put(context.Background(), "test", nil, []byte("test"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestRateLimiter_PutWithHeadersSuccessRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForRateLimiter()
	defer server.Close()

	headers := map[string]string{"Content-Type": "application/json"}
	resp, err := service.PutWithHeaders(context.Background(), "test", nil, []byte("test"), headers)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestRateLimiter_PatchSuccessRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForRateLimiter()
	defer server.Close()

	resp, err := service.Patch(context.Background(), "test", nil, []byte("test"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestRateLimiter_PatchWithHeadersSuccessRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForRateLimiter()
	defer server.Close()

	headers := map[string]string{"Content-Type": "application/json"}
	resp, err := service.PatchWithHeaders(context.Background(), "test", nil, []byte("test"), headers)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestRateLimiter_DeleteSuccessRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForRateLimiter()
	defer server.Close()

	resp, err := service.Delete(context.Background(), "test", []byte("test"))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestRateLimiter_DeleteWithHeadersSuccessRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForRateLimiter()
	defer server.Close()

	headers := map[string]string{"Content-Type": "application/json"}
	resp, err := service.DeleteWithHeaders(context.Background(), "test", []byte("test"), headers)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()
}

func TestRateLimiter_ConcurrentRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForRateLimiter()
	defer server.Close()

	numRequests := 5
	results := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			resp, err := service.Get(context.Background(), "test", nil)
			if err == nil {
				_ = resp.Body.Close()
			}
			results <- err
		}()
	}

	var errors, successes int

	for i := 0; i < numRequests; i++ {
		if err := <-results; err != nil {
			errors++
		} else {
			successes++
		}
	}

	assert.Positive(t, successes, 0, "Expected some requests to succeed")
}

type customTransportRL struct{}

func (*customTransportRL) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Path == "/success" || r.URL.Path == "/test" {
		return &http.Response{
			Body:       io.NopCloser(bytes.NewBufferString("Success")),
			StatusCode: http.StatusOK,
			Request:    r,
		}, nil
	}

	return &http.Response{
		Body:       io.NopCloser(bytes.NewBufferString("Error")),
		StatusCode: http.StatusInternalServerError,
		Request:    r,
	}, nil
}
