package service

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"gofr.dev/pkg/gofr/logging"

	"github.com/stretchr/testify/assert"
)

type mockHTTP struct{}

func (m *mockHTTP) HealthCheck(ctx context.Context) *Health {
	return &Health{
		Status:  "UP",
		Details: map[string]interface{}{"host": "http://test.com"},
	}
}

func (m *mockHTTP) getHealthResponseForEndpoint(ctx context.Context, endpoint string) *Health {
	return &Health{
		Status:  "UP",
		Details: map[string]interface{}{"host": "http://test.com"},
	}
}

func (m *mockHTTP) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(nil)}, nil
}

func (m *mockHTTP) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, headers map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(nil)}, nil
}

func (m *mockHTTP) Post(ctx context.Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusCreated, Body: ioutil.NopCloser(nil)}, nil
}

func (m *mockHTTP) PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusCreated, Body: ioutil.NopCloser(nil)}, nil
}

func (m *mockHTTP) Put(ctx context.Context, api string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(nil)}, nil
}

func (m *mockHTTP) PutWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(nil)}, nil
}

func (m *mockHTTP) Patch(ctx context.Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(nil)}, nil
}

func (m *mockHTTP) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: ioutil.NopCloser(nil)}, nil
}

func (m *mockHTTP) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusNoContent, Body: ioutil.NopCloser(nil)}, nil
}

func (m *mockHTTP) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusNoContent, Body: ioutil.NopCloser(nil)}, nil
}

func TestRetryProvider_Get(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the GET request
	resp, err := retryHTTP.Get(context.Background(), "/test", nil)

	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryProvider_GetWithHeaders(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the GET request with headers
	resp, err := retryHTTP.GetWithHeaders(context.Background(), "/test", nil,
		map[string]string{"Content-Type": "application/json"})

	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryProvider_Post(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the POST request
	resp, err := retryHTTP.Post(context.Background(), "/test", nil, []byte("body"))

	assert.Nil(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestRetryProvider_PostWithHeaders(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the POST request with headers
	resp, err := retryHTTP.PostWithHeaders(context.Background(), "/test", nil, []byte("body"),
		map[string]string{"Content-Type": "application/json"})

	assert.Nil(t, err)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestRetryProvider_Put(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the PUT request
	resp, err := retryHTTP.Put(context.Background(), "/test", nil, []byte("body"))

	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryProvider_PutWithHeaders(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the PUT request with headers
	resp, err := retryHTTP.PutWithHeaders(context.Background(), "/test", nil, []byte("body"),
		map[string]string{"Content-Type": "application/json"})

	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryProvider_Patch_WithError(t *testing.T) {
	// Create a mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkAuthHeaders(r, t)
		assert.Equal(t, http.MethodPatch, r.Method)

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create a new HTTP service instance with basic auth
	httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&RetryConfig{MaxRetries: 5})

	// Make the GET request
	resp, err := httpService.Patch(context.Background(), "/test", nil, []byte("body"))
	assert.Nil(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestRetryProvider_PatchWithHeaders(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the PUT request with headers
	resp, err := retryHTTP.PatchWithHeaders(context.Background(), "/test", nil, []byte("body"),
		map[string]string{"Content-Type": "application/json"})

	assert.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryProvider_Delete(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the PUT request with headers
	resp, err := retryHTTP.Delete(context.Background(), "/test", nil)

	assert.Nil(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
func TestRetryProvider_DeleteWithHeaders(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the PUT request with headers
	resp, err := retryHTTP.DeleteWithHeaders(context.Background(), "/test", []byte("body"),
		map[string]string{"Content-Type": "application/json"})

	assert.Nil(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
