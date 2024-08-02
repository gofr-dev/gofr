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

	resp, err := service.Get(context.Background(), "test", nil)

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

	resp, err := service.GetWithHeaders(context.Background(), "test", nil, nil)

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
		resp, err := service.Get(context.Background(), tc.path, nil)

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
		resp, err := service.GetWithHeaders(context.Background(), tc.path, nil, nil)

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

	resp, err := service.Put(context.Background(), "test", nil, nil)

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

	resp, err := service.PutWithHeaders(context.Background(), "test", nil, nil, nil)

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
		resp, err := service.Put(context.Background(), tc.path, nil, nil)

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
		resp, err := service.PutWithHeaders(context.Background(), tc.path, nil, nil, nil)

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

	resp, err := service.Get(context.Background(), "test", nil)

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

	resp, err := service.GetWithHeaders(context.Background(), "test", nil, nil)

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
		resp, err := service.Patch(context.Background(), tc.path, nil, nil)

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
		resp, err := service.PatchWithHeaders(context.Background(), tc.path, nil, nil, nil)

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

	resp, err := service.Post(context.Background(), "test", nil, nil)

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

	resp, err := service.PostWithHeaders(context.Background(), "test", nil, nil, nil)

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
		resp, err := service.Post(context.Background(), tc.path, nil, nil)

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
		resp, err := service.PostWithHeaders(context.Background(), tc.path, nil, nil, nil)

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

	resp, err := service.Delete(context.Background(), "test", nil)

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

	resp, err := service.DeleteWithHeaders(context.Background(), "test", nil, nil)

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
		resp, err := service.Delete(context.Background(), tc.path, nil)

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
		resp, err := service.DeleteWithHeaders(context.Background(), tc.path, nil, nil)

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
