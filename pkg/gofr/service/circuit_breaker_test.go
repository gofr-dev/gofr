package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

func testServer() *httptest.Server {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return httptest.NewServer(h)
}

func TestHttpService_GetSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), nil, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Get(context.Background(), "test", nil)

	defer resp.Body.Close().Error()

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestHttpService_GetWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), nil, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.GetWithHeaders(context.Background(), "test", nil, nil)

	defer resp.Body.Close().Error()

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestHttpService_PutSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), nil, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Put(context.Background(), "test", nil, nil)

	defer resp.Body.Close().Error()

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestHttpService_PutWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), nil, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.PutWithHeaders(context.Background(), "test", nil, nil, nil)

	defer resp.Body.Close().Error()

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestHttpService_PatchSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), nil, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Get(context.Background(), "test", nil)

	defer resp.Body.Close().Error()

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestHttpService_PatchWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), nil, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.GetWithHeaders(context.Background(), "test", nil, nil)

	defer resp.Body.Close().Error()

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestHttpService_PostSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), nil, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Post(context.Background(), "test", nil, nil)

	defer resp.Body.Close().Error()

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestHttpService_PostWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), nil, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.PostWithHeaders(context.Background(), "test", nil, nil, nil)

	defer resp.Body.Close().Error()

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestHttpService_DeleteSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), nil, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Delete(context.Background(), "test", nil)

	defer resp.Body.Close().Error()

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}

func TestHttpService_DeleteWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), nil, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.DeleteWithHeaders(context.Background(), "test", nil, nil)

	defer resp.Body.Close().Error()

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)
}
