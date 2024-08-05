package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
)

type mockHTTP struct{}

func (*mockHTTP) HealthCheck(_ context.Context) *Health {
	return &Health{
		Status:  "UP",
		Details: map[string]interface{}{"host": "http://test.com"},
	}
}

func (*mockHTTP) getHealthResponseForEndpoint(_ context.Context, _ string, _ int) *Health {
	return &Health{
		Status:  "UP",
		Details: map[string]interface{}{"host": "http://test.com"},
	}
}

func (*mockHTTP) Get(_ context.Context, _ string, _ map[string]interface{}) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func (*mockHTTP) GetWithHeaders(_ context.Context, _ string, _ map[string]interface{}, _ map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func (*mockHTTP) Post(_ context.Context, _ string, _ map[string]interface{}, _ []byte) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusCreated, Body: http.NoBody}, nil
}

func (*mockHTTP) PostWithHeaders(_ context.Context, _ string, _ map[string]interface{}, _ []byte,
	_ map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusCreated, Body: http.NoBody}, nil
}

func (*mockHTTP) Put(_ context.Context, _ string, _ map[string]interface{}, _ []byte) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func (*mockHTTP) PutWithHeaders(_ context.Context, _ string, _ map[string]interface{}, _ []byte,
	_ map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func (*mockHTTP) Patch(_ context.Context, _ string, _ map[string]interface{}, _ []byte) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func (*mockHTTP) PatchWithHeaders(_ context.Context, _ string, _ map[string]interface{}, _ []byte,
	_ map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil
}

func (*mockHTTP) Delete(_ context.Context, _ string, _ []byte) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
}

func (*mockHTTP) DeleteWithHeaders(_ context.Context, _ string, _ []byte, _ map[string]string) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody}, nil
}

func TestRetryProvider_Get(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the GET request
	resp, err := retryHTTP.Get(context.Background(), "/test", nil)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryProvider_GetWithHeaders(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the GET request with headers
	resp, err := retryHTTP.GetWithHeaders(context.Background(), "/test", nil,
		map[string]string{"Content-Type": "application/json"})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryProvider_Post(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the POST request
	resp, err := retryHTTP.Post(context.Background(), "/test", nil, []byte("body"))
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestRetryProvider_PostWithHeaders(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the POST request with headers
	resp, err := retryHTTP.PostWithHeaders(context.Background(), "/test", nil, []byte("body"),
		map[string]string{"Content-Type": "application/json"})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
}

func TestRetryProvider_Put(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the PUT request
	resp, err := retryHTTP.Put(context.Background(), "/test", nil, []byte("body"))
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryProvider_PutWithHeaders(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the PUT request with headers
	resp, err := retryHTTP.PutWithHeaders(context.Background(), "/test", nil, []byte("body"),
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

		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	// Create a new HTTP service instance with basic auth
	httpService := NewHTTPService(server.URL, logging.NewMockLogger(logging.INFO), nil,
		&RetryConfig{MaxRetries: 5})

	// Make the PATCH request
	resp, err := httpService.Patch(context.Background(), "/test", nil, []byte("body"))
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestRetryProvider_PatchWithHeaders(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the PATCH request with headers
	resp, err := retryHTTP.PatchWithHeaders(context.Background(), "/test", nil, []byte("body"),
		map[string]string{"Content-Type": "application/json"})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRetryProvider_Delete(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the DELETE request
	resp, err := retryHTTP.Delete(context.Background(), "/test", nil)
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
func TestRetryProvider_DeleteWithHeaders(t *testing.T) {
	mockHTTP := &mockHTTP{}
	retryConfig := &RetryConfig{MaxRetries: 3}
	retryHTTP := retryConfig.AddOption(mockHTTP)

	// Make the DELETE request with headers
	resp, err := retryHTTP.DeleteWithHeaders(context.Background(), "/test", []byte("body"),
		map[string]string{"Content-Type": "application/json"})
	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}
