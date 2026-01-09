package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func testServer() *httptest.Server {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return httptest.NewServer(h)
}

func setupHTTPServiceTestServerForCircuitBreaker() (*httptest.Server, HTTP) {
	// Start a test HTTP server
	server := testServer()

	mockMetric := &mockMetrics{}
	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	// Initialize HTTP service with custom transport, URL, tracer, logger, and metrics
	service := httpService{
		Client:  &http.Client{Transport: &customTransport{}},
		url:     server.URL,
		Tracer:  otel.Tracer("gofr-http-client"),
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	// Circuit breaker configuration
	cbConfig := CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	}

	// Apply circuit breaker option to the HTTP service
	httpservice := cbConfig.AddOption(&service)

	return server, httpservice
}

func TestHttpService_GetSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Get(t.Context(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func TestHttpService_GetWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.GetWithHeaders(t.Context(), "test", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func TestHttpService_GetCBOpenRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForCircuitBreaker()
	defer server.Close()

	// Test cases
	testCases := []struct {
		name       string
		path       string
		expectErr  bool
		expectResp *http.Response
	}{
		{"Request will Fail", "invalid", true, nil},
		{"Request will Fail", "invalid", true, nil},
		{"Request will pass", "success", false, &http.Response{}},
	}

	// Perform test cases
	for _, tc := range testCases {
		resp, err := service.Get(t.Context(), tc.path, nil)

		if tc.expectErr {
			require.Error(t, err)
			assert.Nil(t, resp)
		} else {
			require.NoError(t, err)
			assert.NotNil(t, resp)
			_ = resp.Body.Close()
		}
	}
}

func TestHttpService_GetWithHeaderCBOpenRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForCircuitBreaker()
	defer server.Close()

	// Test cases
	testCases := []struct {
		name       string
		path       string
		expectErr  bool
		expectResp *http.Response
	}{
		{"Request will Fail", "invalid", true, nil},
		{"Request will Fail", "invalid", true, nil},
		{"Request will pass", "success", false, &http.Response{}},
	}

	// Perform test cases
	for _, tc := range testCases {
		resp, err := service.GetWithHeaders(t.Context(), tc.path, nil, nil)

		if tc.expectErr {
			require.Error(t, err)
			assert.Nil(t, resp)
		} else {
			require.NoError(t, err)
			assert.NotNil(t, resp)
			_ = resp.Body.Close()
		}
	}
}

func TestHttpService_PutSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Put(t.Context(), "test", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func TestHttpService_PutWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.PutWithHeaders(t.Context(), "test", nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func TestHttpService_PutCBOpenRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForCircuitBreaker()
	defer server.Close()

	// Test cases
	testCases := []struct {
		name       string
		path       string
		expectErr  bool
		expectResp *http.Response
	}{
		{"Request will Fail", "invalid", true, nil},
		{"Request will Fail", "invalid", true, nil},
		{"Request will pass", "success", false, &http.Response{}},
	}

	// Perform test cases
	for _, tc := range testCases {
		resp, err := service.Put(t.Context(), tc.path, nil, nil)

		if tc.expectErr {
			require.Error(t, err)
			assert.Nil(t, resp)
		} else {
			require.NoError(t, err)
			assert.NotNil(t, resp)
			_ = resp.Body.Close()
		}
	}
}

func TestHttpService_PutWithHeaderCBOpenRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForCircuitBreaker()
	defer server.Close()

	// Test cases
	testCases := []struct {
		name       string
		path       string
		expectErr  bool
		expectResp *http.Response
	}{
		{"Request will Fail", "invalid", true, nil},
		{"Request will Fail", "invalid", true, nil},
		{"Request will pass", "success", false, &http.Response{}},
	}

	// Perform test cases
	for _, tc := range testCases {
		resp, err := service.PutWithHeaders(t.Context(), tc.path, nil, nil, nil)

		if tc.expectErr {
			require.Error(t, err)
			assert.Nil(t, resp)
		} else {
			require.NoError(t, err)
			assert.NotNil(t, resp)
			_ = resp.Body.Close()
		}
	}
}

func TestHttpService_PatchSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Get(t.Context(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func TestHttpService_PatchWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.GetWithHeaders(t.Context(), "test", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func TestHttpService_PatchCBOpenRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForCircuitBreaker()
	defer server.Close()

	// Test cases
	testCases := []struct {
		name       string
		path       string
		expectErr  bool
		expectResp *http.Response
	}{
		{"Request will Fail", "invalid", true, nil},
		{"Request will Fail", "invalid", true, nil},
		{"Request will pass", "success", false, &http.Response{}},
	}

	// Perform test cases
	for _, tc := range testCases {
		resp, err := service.Patch(t.Context(), tc.path, nil, nil)

		if tc.expectErr {
			require.Error(t, err)
			assert.Nil(t, resp)
		} else {
			require.NoError(t, err)
			assert.NotNil(t, resp)
			_ = resp.Body.Close()
		}
	}
}

func TestHttpService_PatchWithHeaderCBOpenRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForCircuitBreaker()
	defer server.Close()

	// Test cases
	testCases := []struct {
		name       string
		path       string
		expectErr  bool
		expectResp *http.Response
	}{
		{"Request will Fail", "invalid", true, nil},
		{"Request will Fail", "invalid", true, nil},
		{"Request will pass", "success", false, &http.Response{}},
	}

	// Perform test cases
	for _, tc := range testCases {
		resp, err := service.PatchWithHeaders(t.Context(), tc.path, nil, nil, nil)

		if tc.expectErr {
			require.Error(t, err)
			assert.Nil(t, resp)
		} else {
			require.NoError(t, err)
			assert.NotNil(t, resp)
			_ = resp.Body.Close()
		}
	}
}

func TestHttpService_PostSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Post(t.Context(), "test", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func TestHttpService_PostWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.PostWithHeaders(t.Context(), "test", nil, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func TestHttpService_PostCBOpenRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForCircuitBreaker()
	defer server.Close()

	// Test cases
	testCases := []struct {
		name       string
		path       string
		expectErr  bool
		expectResp *http.Response
	}{
		{"Request will Fail", "invalid", true, nil},
		{"Request will Fail", "invalid", true, nil},
		{"Request will pass", "success", false, &http.Response{}},
	}

	// Perform test cases
	for _, tc := range testCases {
		resp, err := service.Post(t.Context(), tc.path, nil, nil)

		if tc.expectErr {
			require.Error(t, err)
			assert.Nil(t, resp)
		} else {
			require.NoError(t, err)
			assert.NotNil(t, resp)
			_ = resp.Body.Close()
		}
	}
}

func TestHttpService_PostWithHeaderCBOpenRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForCircuitBreaker()
	defer server.Close()

	// Test cases
	testCases := []struct {
		name       string
		path       string
		expectErr  bool
		expectResp *http.Response
	}{
		{"Request will Fail", "invalid", true, nil},
		{"Request will Fail", "invalid", true, nil},
		{"Request will pass", "success", false, &http.Response{}},
	}

	// Perform test cases
	for _, tc := range testCases {
		resp, err := service.PostWithHeaders(t.Context(), tc.path, nil, nil, nil)

		if tc.expectErr {
			require.Error(t, err)
			assert.Nil(t, resp)
		} else {
			require.NoError(t, err)
			assert.NotNil(t, resp)
			_ = resp.Body.Close()
		}
	}
}

func TestHttpService_DeleteSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Delete(t.Context(), "test", nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func TestHttpService_DeleteWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.DeleteWithHeaders(t.Context(), "test", nil, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func TestHttpService_DeleteCBOpenRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForCircuitBreaker()
	defer server.Close()

	// Test cases
	testCases := []struct {
		name       string
		path       string
		expectErr  bool
		expectResp *http.Response
	}{
		{"Request will Fail", "invalid", true, nil},
		{"Request will Fail", "invalid", true, nil},
		{"Request will pass", "success", false, &http.Response{}},
	}

	// Perform test cases
	for _, tc := range testCases {
		resp, err := service.Delete(t.Context(), tc.path, nil)

		if tc.expectErr {
			require.Error(t, err)
			assert.Nil(t, resp)
		} else {
			require.NoError(t, err)
			assert.NotNil(t, resp)
			_ = resp.Body.Close()
		}
	}
}

func TestHttpService_DeleteWithHeaderCBOpenRequests(t *testing.T) {
	server, service := setupHTTPServiceTestServerForCircuitBreaker()
	defer server.Close()

	// Test cases
	testCases := []struct {
		name       string
		path       string
		expectErr  bool
		expectResp *http.Response
	}{
		{"Request will Fail", "invalid", true, nil},
		{"Request will Fail", "invalid", true, nil},
		{"Request will pass", "success", false, &http.Response{}},
	}

	// Perform test cases
	for _, tc := range testCases {
		resp, err := service.DeleteWithHeaders(t.Context(), tc.path, nil, nil)

		if tc.expectErr {
			require.Error(t, err)
			assert.Nil(t, resp)
		} else {
			require.NoError(t, err)
			assert.NotNil(t, resp)
			_ = resp.Body.Close()
		}
	}
}

type mockMetrics struct {
	mock.Mock
}

func (m *mockMetrics) RecordHistogram(ctx context.Context, name string, value float64, labels ...string) {
	m.Called(ctx, name, value, labels)
}

type customTransport struct {
}

func (*customTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Path == "/.well-known/alive" || r.URL.Path == "/success" {
		return &http.Response{
			Body:       io.NopCloser(bytes.NewBufferString("Hello World")),
			StatusCode: http.StatusOK,
			Request:    r,
		}, nil
	}

	return nil, testutil.CustomError{ErrorMessage: "cb error"}
}

// customHealthEndpointTransport simulates a service that doesn't have /.well-known/alive
// but has a custom health endpoint like /health or /breeds.
type customHealthEndpointTransport struct {
	healthEndpoint string
}

func (t *customHealthEndpointTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// Custom health endpoint returns OK
	if r.URL.Path == "/"+t.healthEndpoint || r.URL.Path == "/success" {
		return &http.Response{
			Body:       io.NopCloser(bytes.NewBufferString("OK")),
			StatusCode: http.StatusOK,
			Request:    r,
		}, nil
	}

	// Default /.well-known/alive returns 404 (simulating service without GoFr default endpoint)
	if r.URL.Path == "/.well-known/alive" {
		return &http.Response{
			Body:       io.NopCloser(bytes.NewBufferString("Not Found")),
			StatusCode: http.StatusNotFound,
			Request:    r,
		}, nil
	}

	return nil, testutil.CustomError{ErrorMessage: "cb error"}
}

func TestCircuitBreaker_CustomHealthEndpoint_Recovery(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}
	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	// Initialize HTTP service with custom transport that only responds to custom health endpoint
	service := httpService{
		Client:  &http.Client{Transport: &customHealthEndpointTransport{healthEndpoint: "breeds"}},
		url:     server.URL,
		Tracer:  otel.Tracer("gofr-http-client"),
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	// Circuit breaker configuration with custom health endpoint
	cbConfig := CircuitBreakerConfig{
		Threshold:      1,
		Interval:       1,
		HealthEndpoint: "breeds", // Custom endpoint instead of /.well-known/alive
	}

	httpService := cbConfig.AddOption(&service)

	// First request fails - circuit opens
	_, err := httpService.Get(t.Context(), "invalid", nil)
	require.Error(t, err)

	// Second request fails - circuit is now open
	_, err = httpService.Get(t.Context(), "invalid", nil)
	require.Error(t, err)
	assert.Equal(t, ErrCircuitOpen, err)

	// Third request should succeed as circuit recovers using custom health endpoint
	resp, err := httpService.Get(t.Context(), "success", nil)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	_ = resp.Body.Close()
}

func TestCircuitBreaker_DefaultHealthEndpoint_NoRecoveryWhenMissing(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}
	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	// Initialize HTTP service with custom transport that doesn't have /.well-known/alive
	service := httpService{
		Client:  &http.Client{Transport: &customHealthEndpointTransport{healthEndpoint: "breeds"}},
		url:     server.URL,
		Tracer:  otel.Tracer("gofr-http-client"),
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	// Circuit breaker configuration WITHOUT custom health endpoint
	cbConfig := CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
		// HealthEndpoint not set - will use default /.well-known/alive which returns 404
	}

	httpService := cbConfig.AddOption(&service)

	// First request fails - circuit opens
	_, err := httpService.Get(t.Context(), "invalid", nil)
	require.Error(t, err)

	// Second request fails - circuit is now open
	_, err = httpService.Get(t.Context(), "invalid", nil)
	require.Error(t, err)
	assert.Equal(t, ErrCircuitOpen, err)

	// Third request should also fail - circuit cannot recover because /.well-known/alive returns 404
	_, err = httpService.Get(t.Context(), "success", nil)
	require.Error(t, err)
	assert.Equal(t, ErrCircuitOpen, err)
}

func TestCircuitBreaker_HealthEndpointWithTimeout(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}
	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := httpService{
		Client:  &http.Client{Transport: &customHealthEndpointTransport{healthEndpoint: "health"}},
		url:     server.URL,
		Tracer:  otel.Tracer("gofr-http-client"),
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	// Circuit breaker configuration with custom health endpoint and timeout
	cbConfig := CircuitBreakerConfig{
		Threshold:      1,
		Interval:       1,
		HealthEndpoint: "health",
		HealthTimeout:  10, // 10 seconds timeout
	}

	httpService := cbConfig.AddOption(&service)

	// First request fails - circuit opens
	_, err := httpService.Get(t.Context(), "invalid", nil)
	require.Error(t, err)

	// Second request fails - circuit is now open
	_, err = httpService.Get(t.Context(), "invalid", nil)
	require.Error(t, err)

	// Circuit should recover using custom health endpoint
	resp, err := httpService.Get(t.Context(), "success", nil)
	require.NoError(t, err)
	assert.NotNil(t, resp)
	_ = resp.Body.Close()
}

