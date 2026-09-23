package service

import (
	"bytes"
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
	"go.opentelemetry.io/otel"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// The recovery interval these tests configure, and the wait a test does when it needs that
// interval to have elapsed. They are a pair: cbTestRecoveryWait must exceed
// cbTestInterval or a request that is supposed to find the circuit recoverable finds it
// still open, so neither can be tuned alone.
//
// The interval must also stay well above zero. CircuitBreakerConfig.Interval is a
// time.Duration, so the `Interval: 1` these tests used to carry meant one NANOSECOND, and
// NewCircuitBreaker feeds it straight to time.NewTicker in a background goroutine -- a
// ticker firing as fast as the runtime can deliver, for the life of the process, and going on
// firing after the test that created it has returned.
//
// Running the CBOpenRequests tests at -count=20 costs about 2s of CPU with these constants.
// The same run on the nanosecond interval costs tens of seconds to a few minutes -- one to two
// orders of magnitude more, and the figure swings by 5x between runs because it is a function
// of how much idle CPU the ticker finds. Only the small side of that comparison is stable, so
// it is the one quoted here.
const (
	cbTestInterval     = 50 * time.Millisecond
	cbTestRecoveryWait = 60 * time.Millisecond
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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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
		Interval:  cbTestInterval,
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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  cbTestInterval,
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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  cbTestInterval,
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
		if !tc.expectErr {
			time.Sleep(cbTestRecoveryWait)
		}

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
		if !tc.expectErr {
			time.Sleep(cbTestRecoveryWait)
		}

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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  cbTestInterval,
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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  cbTestInterval,
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
		if !tc.expectErr {
			time.Sleep(cbTestRecoveryWait)
		}

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
		if !tc.expectErr {
			time.Sleep(cbTestRecoveryWait)
		}

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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  cbTestInterval,
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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  cbTestInterval,
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
		if !tc.expectErr {
			time.Sleep(cbTestRecoveryWait)
		}

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
		if !tc.expectErr {
			time.Sleep(cbTestRecoveryWait)
		}

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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  cbTestInterval,
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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  cbTestInterval,
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
		if !tc.expectErr {
			time.Sleep(cbTestRecoveryWait)
		}

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
		if !tc.expectErr {
			time.Sleep(cbTestRecoveryWait)
		}

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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  cbTestInterval,
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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	service := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  cbTestInterval,
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
		if !tc.expectErr {
			time.Sleep(cbTestRecoveryWait)
		}

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
		if !tc.expectErr {
			time.Sleep(cbTestRecoveryWait)
		}

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
	// Exact count, not MinTimes: app_circuit_open_count must fire once for the
	// Closed -> Open transition, not once per failing request that observes it.
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), "app_circuit_open_count", "service", "test-service").Times(1)

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

// TestCircuitBreaker_OpenCircuit_CountsTransitionNotFailures pins the fix for
// an app_circuit_open_count over-count found in review of #3856: openCircuit
// is reached from handleFailure whenever failureCount > threshold, and every
// concurrent request that passed the isOpen check before the trip falls
// through to handleFailure and re-enters openCircuit. Without a
// wasOpen guard, a burst of N concurrent failures recorded N-threshold
// "openings" for what is a single Closed -> Open transition — exactly
// concurrency-threshold, deterministic, and invisible to a MinTimes(1)
// assertion (or to sequential traffic, which happens to trigger the trip
// only once and so looks correct either way).
func TestCircuitBreaker_OpenCircuit_CountsTransitionNotFailures(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), "app_circuit_open_count", "service", "test-service").Times(1)

	service := httpService{
		Client:  &http.Client{Transport: &customTransport{}},
		url:     "http://example.invalid",
		name:    "test-service",
		Tracer:  otel.Tracer("gofr-http-client"),
		Logger:  logging.NewMockLogger(logging.DEBUG),
		Metrics: mockMetric,
	}

	cbConfig := CircuitBreakerConfig{
		Threshold: 2,
		// Long enough that the async health-check recovery goroutine cannot
		// fire mid-test and interfere with the open-count assertion.
		Interval: time.Minute,
	}

	httpServiceWithCB := cbConfig.AddOption(&service)

	const concurrency = 30

	var wg sync.WaitGroup

	for range concurrency {
		wg.Add(1)

		go func() {
			defer wg.Done()

			resp, _ := httpServiceWithCB.Get(t.Context(), "invalid", nil)
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
		}()
	}

	wg.Wait()
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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	require.ErrorIs(t, err, ErrCircuitOpen)

	// Third request - circuit is still open, returns ErrCircuitOpen
	resp, err = httpSvc.Get(t.Context(), "fail", nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	require.ErrorIs(t, err, ErrCircuitOpen)

	// Wait for interval to pass so circuit can attempt recovery via /health endpoint
	time.Sleep(1 * time.Second)

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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	require.ErrorIs(t, err, ErrCircuitOpen)

	// Third request - circuit is still open
	resp, err = httpSvc.Get(t.Context(), "fail", nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	require.ErrorIs(t, err, ErrCircuitOpen)

	// Wait for interval to pass
	time.Sleep(500 * time.Millisecond)

	// Fourth request should also fail - circuit cannot recover because /.well-known/alive returns 404
	resp, err = httpSvc.Get(t.Context(), "success", nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	require.ErrorIs(t, err, ErrCircuitOpen)
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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

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
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	require.ErrorIs(t, err, ErrCircuitOpen)

	// Third request - circuit is still open
	resp, err = httpSvc.Get(t.Context(), "fail", nil)
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}

	require.ErrorIs(t, err, ErrCircuitOpen)

	// Wait for interval to pass so circuit can attempt recovery
	time.Sleep(500 * time.Millisecond)

	// Fourth request - circuit should recover using custom health endpoint
	resp, err = httpSvc.Get(t.Context(), "success", nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}

// barrierTimeout bounds how long a handler waits for its peers to show up. It is
// a LIVENESS bound, not a performance one: the barrier releases the instant the
// last participant arrives, so a healthy run never spends time here no matter
// how loaded the machine is. It exists only so that a genuinely serialized
// client fails as an assertion instead of hanging until go test's own deadline.
//
// Fifteen seconds is therefore enormous - what it has to cover is five
// goroutines each opening a localhost connection, which is sub-millisecond work
// - and the whole timeout is spent only on a run that was going to fail anyway.
const barrierTimeout = 15 * time.Second

// statusNotOverlapped is what a handler answers when it waited out the barrier
// instead of meeting its peers -- the failure these two tests exist to report.
//
// It is deliberately NOT a 5xx. The circuit breaker under test counts any status
// above 500 as a failure -- the result.StatusCode > 500 check in
// executeWithCircuitBreaker -- and opens once failureCount exceeds the threshold
// in handleFailure, so answering 503 here would feed the breaker the very signal
// this test emits when it fails. On a run where serialization pushed the count
// past the threshold, the remaining requests would come back as errors rather
// than statuses, require.NoError would fire first, and the operator would read a
// generic circuit-open error instead of "the requests were serialized, not
// parallel".
//
// Neither test can reach that today -- the thresholds are 10 and 5 against at
// most 5 requests -- but the diagnostic should not depend on that arithmetic
// staying true.
const statusNotOverlapped = http.StatusConflict

// concurrencyBarrier proves that n requests were in flight at the same instant,
// without measuring how long anything took.
//
// The two tests below used to assert a wall-clock bound - "all five finished in
// under 2s, therefore they ran in parallel". That conflates "concurrent" with
// "fast": the bound sat one second above a one-second floor, so a loaded runner
// failed a circuit breaker that was behaving perfectly. It also could not tell a
// serialized-but-quick implementation from a parallel one.
//
// A barrier asserts the property itself. Every handler blocks until all n
// handlers are inside it together, which is reachable only if the client
// dispatched them concurrently, and is unreachable if it did not - at any speed.
//
// container/health_concurrency_test.go carries the same idea as checkBarrier,
// and deliberately stays separate: that one is a plain release gate, while this
// one reports the timeout to its caller and broadcasts the give-up so a
// serialized run costs one timeout instead of n. Two uses do not justify a
// shared testutil package; a third would, and unifying them means keeping the
// reporting and the broadcast.
type concurrencyBarrier struct {
	n       int
	all     chan struct{} // closed once every participant has arrived
	givenUp chan struct{} // closed by the first participant to time out

	giveUp sync.Once

	mu      sync.Mutex
	arrived int
}

func newConcurrencyBarrier(n int) *concurrencyBarrier {
	return &concurrencyBarrier{n: n, all: make(chan struct{}), givenUp: make(chan struct{})}
}

// arrive records one participant and blocks until all n have arrived, reporting
// whether that happened within timeout. A false return means the requests were
// serialized.
//
// The first participant to time out releases every other waiter too. Without
// that, serialized requests each wait the full timeout in turn and a failing run
// costs n*timeout - long enough to hit the CI step budget instead of reporting.
func (b *concurrencyBarrier) arrive(timeout time.Duration) bool {
	b.mu.Lock()

	b.arrived++
	if b.arrived == b.n {
		close(b.all)
	}

	b.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-b.all:
		return true
	case <-b.givenUp:
		return false
	case <-timer.C:
		b.giveUp.Do(func() { close(b.givenUp) })

		return false
	}
}

// TestCircuitBreaker_ParallelExecution tests that requests execute in parallel.
func TestCircuitBreaker_ParallelExecution(t *testing.T) {
	const numRequests = 5

	requestCount := 0
	mu := sync.Mutex{}
	barrier := newConcurrencyBarrier(numRequests)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()

		requestCount++

		mu.Unlock()

		// Releases only when all five requests are inside the handler at once.
		// If they were serialized, the first one waits alone and answers
		// statusNotOverlapped, which the status assertion below turns into a
		// failure.
		if !barrier.arrive(barrierTimeout) {
			w.WriteHeader(statusNotOverlapped)

			return
		}

		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	httpSvc := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric,
		&CircuitBreakerConfig{
			Threshold: 10,
			Interval:  5 * time.Second,
		})

	var wg sync.WaitGroup

	reqErrs := make([]error, numRequests)
	statuses := make([]int, numRequests)

	// Launch 5 concurrent requests
	for i := 0; i < numRequests; i++ {
		wg.Add(1)

		go func(index int) {
			defer wg.Done()

			resp, err := httpSvc.Get(t.Context(), "test", nil)
			reqErrs[index] = err

			if err == nil && resp != nil {
				statuses[index] = resp.StatusCode

				_, _ = io.ReadAll(resp.Body)

				_ = resp.Body.Close()
			}
		}(i)
	}

	wg.Wait()

	// Verify all requests completed successfully. A statusNotOverlapped is the
	// barrier reporting that this request never overlapped the other four.
	for i := 0; i < numRequests; i++ {
		require.NoError(t, reqErrs[i], "Request %d should not error", i)
		assert.Equal(t, http.StatusOK, statuses[i],
			"Request %d did not overlap the others: requests were serialized, not parallel", i)
	}

	assert.Equal(t, numRequests, requestCount, "All requests should have been processed")
}

// TestCircuitBreaker_ConcurrentFailures tests thread safety during concurrent failures.
func TestCircuitBreaker_ConcurrentFailures(t *testing.T) {
	failCount := 0
	mu := sync.Mutex{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()

		failCount++

		current := failCount

		mu.Unlock()

		// First 3 requests fail, rest succeed
		if current <= 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
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
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	httpSvc := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric,
		&CircuitBreakerConfig{
			Threshold: 2,
			Interval:  1 * time.Second,
		})

	var wg sync.WaitGroup

	numRequests := 10

	for i := 0; i < numRequests; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			resp, _ := httpSvc.Get(t.Context(), "test", nil)
			if resp != nil {
				_ = resp.Body.Close()
			}
		}()
	}

	wg.Wait()
}

// TestCircuitBreaker_MixedHTTPMethods tests parallel requests with different HTTP methods.
func TestCircuitBreaker_MixedHTTPMethods(t *testing.T) {
	const numMethods = 5

	barrier := newConcurrencyBarrier(numMethods)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// See TestCircuitBreaker_ParallelExecution: statusNotOverlapped means this
		// request never overlapped its peers.
		if !barrier.arrive(barrierTimeout) {
			w.WriteHeader(statusNotOverlapped)

			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctrl := gomock.NewController(t)
	mockMetric := NewMockMetrics(ctrl)

	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	httpSvc := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), mockMetric,
		&CircuitBreakerConfig{
			Threshold: 5,
			Interval:  2 * time.Second,
		})

	var wg sync.WaitGroup

	// Test all HTTP methods in parallel
	methods := []func() (*http.Response, error){
		func() (*http.Response, error) { return httpSvc.Get(t.Context(), "test", nil) },
		func() (*http.Response, error) { return httpSvc.Post(t.Context(), "test", nil, []byte(`{}`)) },
		func() (*http.Response, error) { return httpSvc.Put(t.Context(), "test", nil, []byte(`{}`)) },
		func() (*http.Response, error) { return httpSvc.Patch(t.Context(), "test", nil, []byte(`{}`)) },
		func() (*http.Response, error) { return httpSvc.Delete(t.Context(), "test", []byte(`{}`)) },
	}

	// The barrier is sized for exactly this many participants, so the slice and
	// numMethods have to agree. This runs before any request is dispatched, so
	// either direction -- a method added, or one removed -- fails here in
	// milliseconds with a length mismatch rather than through a handler that
	// waits out barrierTimeout.
	//
	// It cannot move any earlier: every entry in methods closes over httpSvc,
	// which needs server.URL, so the server necessarily exists by this point.
	require.Len(t, methods, numMethods)

	errs := make([]error, numMethods)
	statuses := make([]int, numMethods)

	for i, method := range methods {
		wg.Add(1)

		go func(index int, fn func() (*http.Response, error)) {
			defer wg.Done()

			resp, err := fn()
			errs[index] = err

			if err == nil && resp != nil {
				statuses[index] = resp.StatusCode

				_ = resp.Body.Close()
			}
		}(i, method)
	}

	wg.Wait()

	for i := 0; i < numMethods; i++ {
		require.NoError(t, errs[i], "Method %d should not error", i)
		assert.Equal(t, http.StatusOK, statuses[i],
			"Method %d did not overlap the others: the methods were serialized, not parallel", i)
	}
}

// TestCircuitBreaker_SlowHealthCheckDoesNotBlock asserts that while tryCircuitRecovery's
// health probe is in-flight, other circuit-breaker operations are not blocked by the
// exclusive lock. Regression guard for the fix in tryCircuitRecovery that releases
// cb.mu before calling healthCheck.
//
// Built directly on the unexported circuitBreaker struct (without NewCircuitBreaker)
// so the background health-check ticker doesn't introduce timing races.
func TestCircuitBreaker_SlowHealthCheckDoesNotBlock(t *testing.T) {
	healthEntered := make(chan struct{}, 1)
	releaseHealth := make(chan struct{})

	ctrl := gomock.NewController(t)
	mockHTTP := NewMockHTTP(ctrl)

	mockHTTP.EXPECT().
		HealthCheck(gomock.Any()).
		DoAndReturn(func(_ context.Context) *Health {
			select {
			case healthEntered <- struct{}{}:
			default:
			}

			<-releaseHealth

			return &Health{Status: serviceUp}
		}).
		AnyTimes()

	cb := &circuitBreaker{
		state:       OpenState,
		threshold:   1,
		interval:    50 * time.Millisecond,
		lastChecked: time.Now().Add(-1 * time.Hour), // stale, so time.Since > interval
		HTTP:        mockHTTP,
	}

	// Goroutine A triggers recovery; its HealthCheck will block until releaseHealth is closed.
	aDone := make(chan bool, 1)

	go func() {
		aDone <- cb.tryCircuitRecovery()
	}()

	// Wait until the health check is actually in-flight (lock has been released by A).
	select {
	case <-healthEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("HealthCheck never invoked")
	}

	// Goroutine B issues independent operations. With the fix, none should be blocked
	// by A's in-flight health check. Without the fix, isOpen()/tryCircuitRecovery would
	// wait on cb.mu held by A across the slow HealthCheck call.
	type bResult struct {
		isOpen    bool
		recovered bool
	}

	bDone := make(chan bResult, 1)

	go func() {
		open := cb.isOpen()
		recovered := cb.tryCircuitRecovery()

		bDone <- bResult{isOpen: open, recovered: recovered}
	}()

	select {
	case r := <-bDone:
		assert.True(t, r.isOpen, "circuit should still report open while health check is in-flight")
		assert.False(t, r.recovered, "second tryCircuitRecovery must short-circuit, not block")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("concurrent operations were blocked by the in-flight health check")
	}

	// Unblock A and verify recovery completed successfully.
	close(releaseHealth)

	select {
	case ok := <-aDone:
		require.True(t, ok, "tryCircuitRecovery should succeed after healthy probe")
	case <-time.After(2 * time.Second):
		t.Fatal("tryCircuitRecovery never returned")
	}

	cb.mu.RLock()
	state := cb.state
	cb.mu.RUnlock()
	assert.Equal(t, ClosedState, state, "circuit should be closed after successful recovery")
}

func TestHttpService_QuerySuccessRequests(t *testing.T) {
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

	resp, err := service.Query(t.Context(), "test", nil, []byte(`{"q":"x"}`))

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

func TestHttpService_QueryWithHeaderSuccessRequests(t *testing.T) {
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

	resp, err := service.QueryWithHeaders(t.Context(), "test", nil, []byte(`{"q":"x"}`),
		map[string]string{"content-type": "application/json"})

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	_ = resp.Body.Close()
}

// TestCircuitBreaker_doRequest_UnsupportedMethod verifies the default case:
// an unknown method must return errUnsupportedMethod instead of a nil response.
func TestCircuitBreaker_doRequest_UnsupportedMethod(t *testing.T) {
	cb := &circuitBreaker{state: ClosedState, interval: time.Second}

	//nolint:bodyclose // the unsupported-method path returns a nil response, nothing to close
	resp, err := cb.doRequest(t.Context(), "TRACE", "test", nil, nil, nil)

	require.ErrorIs(t, err, errUnsupportedMethod)
	assert.Nil(t, resp)
}

// cbQueryServer is a controllable downstream for QUERY circuit-breaker integration
// tests. When down is true, QUERY /search returns 503 (>500, trips the breaker). When
// aliveMirrorsDown is true, the /.well-known/alive probe also fails while down, so the
// breaker stays open until the service actually heals.
func cbQueryServer(down *atomic.Bool, aliveMirrorsDown bool) *httptest.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != methodQuery {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if down.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		body, _ := io.ReadAll(r.Body)

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})

	mux.HandleFunc("/.well-known/alive", func(w http.ResponseWriter, _ *http.Request) {
		if aliveMirrorsDown && down.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
	})

	return httptest.NewServer(mux)
}

// cbQueryBreaker builds a circuitBreaker directly (not via NewCircuitBreaker) so it does NOT
// start the background health-check goroutine — keeping the test deterministic and leak-free.
// Its embedded HTTP is a plain httpService (no decorators) pointing at url.
func cbQueryBreaker(t *testing.T, url string, threshold int, interval time.Duration) *circuitBreaker {
	t.Helper()

	ctrl := gomock.NewController(t)
	m := NewMockMetrics(ctrl)
	m.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	m.EXPECT().NewCounter(gomock.Any(), gomock.Any()).AnyTimes()
	m.EXPECT().NewGauge(gomock.Any(), gomock.Any()).AnyTimes()
	m.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	base := NewHTTPService(url, logging.NewMockLogger(logging.ERROR), m)

	return &circuitBreaker{
		state:     ClosedState,
		threshold: threshold,
		interval:  interval,
		HTTP:      base,
	}
}

// TestCircuitBreaker_Query_Trips verifies an outbound QUERY participates in circuit-breaker
// failure accounting: repeated downstream 503s open the circuit and further QUERY calls are
// short-circuited with ErrCircuitOpen without hitting the downstream.
func TestCircuitBreaker_Query_Trips(t *testing.T) {
	var down atomic.Bool

	server := cbQueryServer(&down, false)
	defer server.Close()

	cb := cbQueryBreaker(t, server.URL, 2, time.Minute)

	// Healthy: QUERY succeeds and body is echoed.
	resp, err := cb.Query(t.Context(), "search", nil, []byte(`{"q":"ok"}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	// Failing downstream: drive QUERY calls until the circuit opens.
	down.Store(true)

	var opened bool

	for i := 0; i < 6; i++ {
		resp, err = cb.Query(t.Context(), "search", nil, []byte(`{"q":"x"}`))
		if resp != nil {
			_ = resp.Body.Close()
		}

		if err != nil && err.Error() == ErrCircuitOpen.Error() {
			opened = true
			break
		}
	}

	assert.True(t, opened, "QUERY calls should open the circuit after repeated downstream failures")
}

// TestCircuitBreaker_Query_Recovers verifies that when the circuit is open, an outbound QUERY
// triggers recovery: once the interval has elapsed and the downstream health probe reports UP,
// the breaker closes and the QUERY is served. Deterministic — no background goroutine, no sleeps.
func TestCircuitBreaker_Query_Recovers(t *testing.T) {
	var down atomic.Bool

	server := cbQueryServer(&down, false) // /.well-known/alive stays UP => healthy
	defer server.Close()

	cb := cbQueryBreaker(t, server.URL, 2, 50*time.Millisecond)

	// Force the breaker open with a stale lastChecked so the next QUERY attempts recovery.
	cb.state = OpenState
	cb.lastChecked = time.Now().Add(-time.Hour)

	resp, err := cb.Query(t.Context(), "search", nil, []byte(`{"q":"ok"}`))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	cb.mu.RLock()
	state := cb.state
	cb.mu.RUnlock()
	assert.Equal(t, ClosedState, state, "circuit should be closed after a successful QUERY recovery")
}

// TestCircuitBreaker_ConcurrentRecovery_OnlyOneResetsCircuit verifies that when N
// goroutines simultaneously discover the circuit is open and the recovery interval has
// elapsed, exactly ONE goroutine fires the health check and resets the circuit.
// The test is deterministic: the mock health check blocks until all N-1 "loser"
// goroutines have returned from tryCircuitRecovery, proving they were turned away
// without calling HealthCheck. If two goroutines won the race the othersDone channel
// would never close and the test would fail with a timeout error.
func TestCircuitBreaker_ConcurrentRecovery_OnlyOneResetsCircuit(t *testing.T) {
	const numGoroutines = 20

	var healthCallCount int64

	var returned int64

	othersDone := make(chan struct{})

	var once sync.Once

	ctrl := gomock.NewController(t)
	mockHTTP := NewMockHTTP(ctrl)

	mockHTTP.EXPECT().
		HealthCheck(gomock.Any()).
		DoAndReturn(func(_ context.Context) *Health {
			atomic.AddInt64(&healthCallCount, 1)

			select {
			case <-othersDone:
			case <-time.After(2 * time.Second): // Bounded wait so the test fails instead of hanging on mutants
				t.Error("test timed out waiting for losers to finish")
			}

			return &Health{Status: serviceUp}
		}).
		AnyTimes()

	cb := &circuitBreaker{
		state:       OpenState,
		threshold:   1,
		interval:    time.Hour,
		lastChecked: time.Now().Add(-2 * time.Hour), // Force interval to have elapsed
		HTTP:        mockHTTP,
	}

	startCh := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()

			<-startCh

			cb.tryCircuitRecovery()

			if atomic.AddInt64(&returned, 1) == numGoroutines-1 {
				once.Do(func() { close(othersDone) })
			}
		}()
	}

	close(startCh)

	wg.Wait()

	assert.Equal(t, int64(1), atomic.LoadInt64(&healthCallCount), "health check should be called exactly once")

	cb.mu.RLock()
	state := cb.state
	cb.mu.RUnlock()
	assert.Equal(t, ClosedState, state, "circuit should be closed after successful recovery")
}

// errClientWentAway is the cause a caller attaches with context.WithCancelCause. net/http
// returns context.Cause(ctx) rather than ctx.Err() when a request is canceled, so the error
// such a caller sees is this one and not context.Canceled.
var errClientWentAway = errors.New("client went away")

// cancelTestUpstream is a healthy upstream for the cancellation tests:
//   - /hang signals arrived (when anyone is listening) and then blocks until the request
//     context ends, so a test can cancel a request that is genuinely in flight;
//   - /unavailable answers 503, which the breaker counts as a failure;
//   - everything else answers 200.
func cancelTestUpstream(t *testing.T) (server *httptest.Server, arrived chan struct{}) {
	t.Helper()

	arrived = make(chan struct{})

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hang":
			select {
			case arrived <- struct{}{}:
			case <-r.Context().Done():
			}

			<-r.Context().Done()
		case "/unavailable":
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(server.Close)

	return server, arrived
}

func cancelTestMetrics(t *testing.T) *MockMetrics {
	t.Helper()

	mockMetric := NewMockMetrics(gomock.NewController(t))
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().SetGauge(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mockMetric.EXPECT().IncrementCounter(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	return mockMetric
}

func newCancelTestBreaker(t *testing.T, url string, threshold int) *circuitBreaker {
	t.Helper()

	svc := NewHTTPService(url, logging.NewMockLogger(logging.DEBUG), cancelTestMetrics(t),
		&CircuitBreakerConfig{Threshold: threshold, Interval: time.Hour})

	cb, ok := svc.(*circuitBreaker)
	require.True(t, ok, "CircuitBreakerConfig must be the outermost option here")

	return cb
}

func breakerSnapshot(cb *circuitBreaker) (state, failures int) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	return cb.state, cb.failureCount
}

// getCanceledInFlight sends GET /hang and cancels the caller's context once the upstream
// has received the request, so the cancellation happens mid-flight, as when a browser
// navigates away while the gateway is still waiting on the upstream.
func getCanceledInFlight(t *testing.T, svc HTTP, arrived <-chan struct{}) error {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go func() {
		<-arrived
		cancel()
	}()

	resp, err := svc.Get(ctx, "hang", nil)
	if resp != nil {
		_ = resp.Body.Close()
	}

	return err
}

func getStatus(ctx context.Context, svc HTTP, path string) (int, error) {
	resp, err := svc.Get(ctx, path, nil)
	if resp == nil {
		return 0, err
	}

	_ = resp.Body.Close()

	return resp.StatusCode, err
}

// A request its own caller abandons says nothing about the upstream, so eleven of them in a
// row against a Threshold of 10 must leave the breaker closed for everyone else. Before the
// fix the eleventh cancellation opened it.
func TestCircuitBreaker_CallerCancellationDoesNotOpenCircuit(t *testing.T) {
	const threshold = 10

	tests := []struct {
		desc    string
		call    func(t *testing.T, svc HTTP, arrived <-chan struct{}) error
		wantErr error
	}{
		{
			desc:    "canceled while in flight",
			call:    getCanceledInFlight,
			wantErr: context.Canceled,
		},
		{
			desc: "canceled before it was sent",
			call: func(t *testing.T, svc HTTP, _ <-chan struct{}) error {
				t.Helper()

				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				_, err := getStatus(ctx, svc, "hang")

				return err
			},
			wantErr: context.Canceled,
		},
		{
			desc: "canceled with a cause",
			call: func(t *testing.T, svc HTTP, arrived <-chan struct{}) error {
				t.Helper()

				ctx, cancel := context.WithCancelCause(t.Context())
				defer cancel(nil)

				go func() {
					<-arrived
					cancel(errClientWentAway)
				}()

				_, err := getStatus(ctx, svc, "hang")

				return err
			},
			wantErr: errClientWentAway,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			server, arrived := cancelTestUpstream(t)
			cb := newCancelTestBreaker(t, server.URL, threshold)

			for i := 0; i <= threshold; i++ {
				err := tc.call(t, cb, arrived)
				require.ErrorIs(t, err, tc.wantErr, "call %d", i+1)
				require.NotErrorIs(t, err, ErrCircuitOpen, "call %d", i+1)
			}

			state, failures := breakerSnapshot(cb)
			assert.Equal(t, ClosedState, state)
			assert.Zero(t, failures)

			code, err := getStatus(t.Context(), cb, "ok")
			require.NoError(t, err, "a healthy upstream must stay reachable for other callers")
			assert.Equal(t, http.StatusOK, code)
		})
	}
}

// The failures the breaker exists for still open it: a 503, a refused connection, and a
// request that ran out of time (context.DeadlineExceeded is evidence of a slow upstream,
// so it keeps counting).
func TestCircuitBreaker_UpstreamFailuresStillOpenCircuit(t *testing.T) {
	const threshold = 10

	refused := httptest.NewServer(http.NotFoundHandler())
	refusedURL := refused.URL
	refused.Close()

	tests := []struct {
		desc string
		url  func(server *httptest.Server) string
		call func(t *testing.T, svc HTTP) error
	}{
		{
			desc: "503 from the upstream",
			url:  func(s *httptest.Server) string { return s.URL },
			call: func(t *testing.T, svc HTTP) error {
				t.Helper()

				code, err := getStatus(t.Context(), svc, "unavailable")
				if err == nil {
					assert.Equal(t, http.StatusServiceUnavailable, code)
				}

				return err
			},
		},
		{
			desc: "connection refused",
			url:  func(*httptest.Server) string { return refusedURL },
			call: func(t *testing.T, svc HTTP) error {
				t.Helper()

				_, err := getStatus(t.Context(), svc, "ok")
				require.Error(t, err)

				return err
			},
		},
		{
			desc: "deadline exceeded",
			url:  func(s *httptest.Server) string { return s.URL },
			call: func(t *testing.T, svc HTTP) error {
				t.Helper()

				ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
				defer cancel()

				_, err := getStatus(ctx, svc, "hang")
				require.Error(t, err)

				if !errors.Is(err, ErrCircuitOpen) {
					require.ErrorIs(t, err, context.DeadlineExceeded)
				}

				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			server, _ := cancelTestUpstream(t)
			cb := newCancelTestBreaker(t, tc.url(server), threshold)

			for i := 0; i < threshold; i++ {
				err := tc.call(t, cb)
				require.NotErrorIs(t, err, ErrCircuitOpen, "call %d must not open the circuit yet", i+1)
			}

			require.ErrorIs(t, tc.call(t, cb), ErrCircuitOpen, "call %d must open the circuit", threshold+1)

			state, _ := breakerSnapshot(cb)
			assert.Equal(t, OpenState, state)
		})
	}
}

// A cancellation between two real failures is neither a failure nor a success: it must not
// add to the streak and it must not reset it.
func TestCircuitBreaker_CallerCancellationKeepsFailureStreak(t *testing.T) {
	server, arrived := cancelTestUpstream(t)
	cb := newCancelTestBreaker(t, server.URL, 2)

	code, err := getStatus(t.Context(), cb, "unavailable")
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, code)

	require.ErrorIs(t, getCanceledInFlight(t, cb, arrived), context.Canceled)

	code, err = getStatus(t.Context(), cb, "unavailable")
	require.NoError(t, err, "the cancellation must not have counted as a failure")
	require.Equal(t, http.StatusServiceUnavailable, code)

	state, failures := breakerSnapshot(cb)
	assert.Equal(t, ClosedState, state)
	assert.Equal(t, 2, failures, "the cancellation must not have reset the streak")

	require.ErrorIs(t, getCanceledInFlight(t, cb, arrived), context.Canceled)

	_, err = getStatus(t.Context(), cb, "unavailable")
	require.ErrorIs(t, err, ErrCircuitOpen, "the third real failure must open the circuit")
}

// With Retry as the outer layer (the order the docs recommend), every attempt reaches the
// breaker. A canceled request is retried against an already-canceled context, so before
// the fix one abandoned request counted MaxRetries+1 failures.
func TestCircuitBreaker_CallerCancellationThroughRetryDoesNotOpenCircuit(t *testing.T) {
	server, arrived := cancelTestUpstream(t)

	svc := NewHTTPService(server.URL, logging.NewMockLogger(logging.DEBUG), cancelTestMetrics(t),
		&CircuitBreakerConfig{Threshold: 2, Interval: time.Hour},
		&RetryConfig{MaxRetries: 3})

	rp, ok := svc.(*retryProvider)
	require.True(t, ok)

	cb, ok := rp.HTTP.(*circuitBreaker)
	require.True(t, ok)

	for i := 0; i < 3; i++ {
		require.ErrorIs(t, getCanceledInFlight(t, svc, arrived), context.Canceled, "request %d", i+1)
	}

	state, failures := breakerSnapshot(cb)
	assert.Equal(t, ClosedState, state)
	assert.Zero(t, failures)
}

// Concurrent cancellations share one breaker; run under -race.
func TestCircuitBreaker_ConcurrentCallerCancellations(t *testing.T) {
	const callers = 20

	server, _ := cancelTestUpstream(t)
	cb := newCancelTestBreaker(t, server.URL, 5)

	var wg sync.WaitGroup

	errs := make([]error, callers)

	for i := range callers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			stop := time.AfterFunc(10*time.Millisecond, cancel)
			defer stop.Stop()

			resp, err := cb.Get(ctx, "hang", nil)
			if resp != nil {
				_ = resp.Body.Close()
			}

			errs[i] = err
		}()
	}

	wg.Wait()

	for i, err := range errs {
		require.ErrorIs(t, err, context.Canceled, "caller %d", i)
	}

	state, failures := breakerSnapshot(cb)
	assert.Equal(t, ClosedState, state)
	assert.Zero(t, failures)
}
