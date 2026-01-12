package service

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func testServer() *httptest.Server {
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return httptest.NewServer(h)
}

func setupHTTPServiceTestServerForCircuitBreaker(t *testing.T) (*httptest.Server, HTTP) {
	t.Helper()

	// Start a test HTTP server
	server := testServer()

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(),
		gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// Initialize HTTP service with custom transport, URL, tracer, logger, and metrics
	service := httpService{
		Client:  &http.Client{Transport: &customTransport{}},
		url:     server.URL,
		name:    "test-service",
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

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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
	server, service := setupHTTPServiceTestServerForCircuitBreaker(t)
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
	server, service := setupHTTPServiceTestServerForCircuitBreaker(t)
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

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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
	server, service := setupHTTPServiceTestServerForCircuitBreaker(t)
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
	server, service := setupHTTPServiceTestServerForCircuitBreaker(t)
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

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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
	server, service := setupHTTPServiceTestServerForCircuitBreaker(t)
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
	server, service := setupHTTPServiceTestServerForCircuitBreaker(t)
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

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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
	server, service := setupHTTPServiceTestServerForCircuitBreaker(t)
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
	server, service := setupHTTPServiceTestServerForCircuitBreaker(t)
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

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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
	server, service := setupHTTPServiceTestServerForCircuitBreaker(t)
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
	server, service := setupHTTPServiceTestServerForCircuitBreaker(t)
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

func TestCircuitBreaker_Metrics(t *testing.T) {
	server := testServer()
	defer server.Close()

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge("app_http_circuit_breaker_state", 1.0, "service", "test-service").MinTimes(1)
	mockMetric.EXPECT().SetGauge("app_http_circuit_breaker_state", 0.0, "service", "test-service").AnyTimes()

	service := httpService{
		Client:  &http.Client{Transport: &customTransport{}},
		url:     server.URL,
		name:    "test-service",
		Tracer:  otel.Tracer("gofr-http-client"),
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	cbConfig := CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1 * time.Second,
	}

	httpServiceWithCB := cbConfig.AddOption(&service)

	// Trigger failures to open circuit
	for i := 0; i < 3; i++ {
		resp, _ := httpServiceWithCB.Get(t.Context(), "invalid", nil)
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
	}
}

func TestCircuitBreaker_HTTP500_TripsCircuit(t *testing.T) {
	server := testServer()
	defer server.Close()

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	// Expect metrics to be recorded
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	service := httpService{
		Client:  &http.Client{Transport: &customTransport{}},
		url:     server.URL,
		name:    "test-service",
		Tracer:  otel.Tracer("gofr-http-client"),
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	// Threshold 1, Long Interval
	cbConfig := CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1 * time.Minute,
	}

	httpServiceWithCB := cbConfig.AddOption(&service)

	// 1. First call returns 500. Failure count becomes 1.
	resp, err := httpServiceWithCB.Get(t.Context(), "error-500", nil)
	require.NoError(t, err) // 500 is not an error returned by Get
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	resp.Body.Close()

	// 2. Second call returns 500. Failure count becomes 2. Threshold (1) exceeded. Circuit Opens immediately.
	// The request is executed, but the CB sees the failure count > threshold and returns ErrCircuitOpen.
	resp, err = httpServiceWithCB.Get(t.Context(), "error-500", nil)
	if resp != nil {
		resp.Body.Close()
	}

	require.ErrorIs(t, err, ErrCircuitOpen)
	assert.Nil(t, resp)

	// 3. Third call should also fail with ErrCircuitOpen (Circuit is Open)
	resp, err = httpServiceWithCB.Get(t.Context(), "error-500", nil)
	if resp != nil {
		resp.Body.Close()
	}

	require.ErrorIs(t, err, ErrCircuitOpen)
	assert.Nil(t, resp)
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

	if r.URL.Path == "/error-500" {
		return &http.Response{
			Body:       io.NopCloser(bytes.NewBufferString("Internal Server Error")),
			StatusCode: http.StatusServiceUnavailable,
			Request:    r,
		}, nil
	}

	return nil, testutil.CustomError{ErrorMessage: "cb error"}
}

func TestCircuitBreaker_CustomHealthEndpoint_Recovery(t *testing.T) {
	// Server that returns 502 for /fail (triggers circuit breaker), 200 for /health and /success
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/fail":
			w.WriteHeader(http.StatusBadGateway)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	httpSvc := NewHTTPService(server.URL,
		logging.NewMockLogger(logging.DEBUG),
		mockMetric,
		&CircuitBreakerConfig{Threshold: 1, Interval: 200 * time.Millisecond},
		&HealthConfig{HealthEndpoint: "health", Timeout: 5},
	)

	// First request returns 502 - failure count becomes 1, threshold not exceeded (1 > 1 is false)
	resp, err := httpSvc.Get(t.Context(), "fail", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	resp.Body.Close()

	// Second request returns 502 - failure count becomes 2, exceeds threshold (2 > 1)
	// Circuit opens and returns ErrCircuitOpen
	resp, err = httpSvc.Get(t.Context(), "fail", nil)
	require.ErrorIs(t, err, ErrCircuitOpen)
	require.NoError(t, resp.Body.Close())

	// Third request - circuit is still open, returns ErrCircuitOpen
	resp, err = httpSvc.Get(t.Context(), "fail", nil)
	require.ErrorIs(t, err, ErrCircuitOpen)
	require.NoError(t, resp.Body.Close())

	// Wait for interval to pass so circuit can attempt recovery via /health endpoint
	time.Sleep(500 * time.Millisecond)

	// Fourth request - circuit should recover via /health endpoint and succeed
	resp, err = httpSvc.Get(t.Context(), "success", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

func TestCircuitBreaker_DefaultHealthEndpoint_NoRecoveryWhenMissing(t *testing.T) {
	// Server that returns 502 for /fail (triggers circuit breaker),
	// 404 for /.well-known/alive (default health endpoint missing)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/alive":
			w.WriteHeader(http.StatusNotFound) // Default health endpoint not available
		case "/fail":
			w.WriteHeader(http.StatusBadGateway)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// No HealthConfig - will use default /.well-known/alive which returns 404
	httpSvc := NewHTTPService(server.URL,
		logging.NewMockLogger(logging.DEBUG),
		mockMetric,
		&CircuitBreakerConfig{Threshold: 1, Interval: 200 * time.Millisecond},
	)

	// First request returns 502 - failure count becomes 1, threshold not exceeded
	resp, err := httpSvc.Get(t.Context(), "fail", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	resp.Body.Close()

	// Second request returns 502 - failure count becomes 2, exceeds threshold
	// Circuit opens and returns ErrCircuitOpen
	resp, err = httpSvc.Get(t.Context(), "fail", nil)
	require.ErrorIs(t, err, ErrCircuitOpen)
	require.NoError(t, resp.Body.Close())

	// Third request - circuit is still open
	resp, err = httpSvc.Get(t.Context(), "fail", nil)
	require.ErrorIs(t, err, ErrCircuitOpen)
	require.NoError(t, resp.Body.Close())

	// Wait for interval to pass
	time.Sleep(500 * time.Millisecond)

	// Fourth request should also fail - circuit cannot recover because /.well-known/alive returns 404
	resp, err = httpSvc.Get(t.Context(), "success", nil)
	require.ErrorIs(t, err, ErrCircuitOpen)
	require.NoError(t, resp.Body.Close())
}

func TestCircuitBreaker_HealthEndpointWithTimeout(t *testing.T) {
	// Server that returns 502 for /fail, 200 for /health and other paths
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
		case "/fail":
			w.WriteHeader(http.StatusBadGateway)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	httpSvc := NewHTTPService(server.URL,
		logging.NewMockLogger(logging.DEBUG),
		mockMetric,
		&CircuitBreakerConfig{Threshold: 1, Interval: 200 * time.Millisecond},
		&HealthConfig{HealthEndpoint: "health", Timeout: 10},
	)

	// First request returns 502 - failure count becomes 1, threshold not exceeded
	resp, err := httpSvc.Get(t.Context(), "fail", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusBadGateway, resp.StatusCode)
	resp.Body.Close()

	// Second request returns 502 - failure count becomes 2, exceeds threshold
	// Circuit opens and returns ErrCircuitOpen
	resp, err = httpSvc.Get(t.Context(), "fail", nil)
	require.ErrorIs(t, err, ErrCircuitOpen)
	require.NoError(t, resp.Body.Close())

	// Third request - circuit is still open
	resp, err = httpSvc.Get(t.Context(), "fail", nil)
	require.ErrorIs(t, err, ErrCircuitOpen)
	require.NoError(t, resp.Body.Close())

	// Wait for interval to pass so circuit can attempt recovery
	time.Sleep(500 * time.Millisecond)

	// Fourth request - circuit should recover using custom health endpoint
	resp, err = httpSvc.Get(t.Context(), "success", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
