package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

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

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Get(context.Background(), "test", nil)

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	_ = resp.Body.Close()
}

func TestHttpService_GetWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.GetWithHeaders(context.Background(), "test", nil, nil)

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	_ = resp.Body.Close()
}

func TestHttpService_PutSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Put(context.Background(), "test", nil, nil)

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	_ = resp.Body.Close()
}

func TestHttpService_PutWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.PutWithHeaders(context.Background(), "test", nil, nil, nil)

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	_ = resp.Body.Close()
}

func TestHttpService_PatchSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Get(context.Background(), "test", nil)

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	_ = resp.Body.Close()
}

func TestHttpService_PatchWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.GetWithHeaders(context.Background(), "test", nil, nil)

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	_ = resp.Body.Close()
}

func TestHttpService_PostSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Post(context.Background(), "test", nil, nil)

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	_ = resp.Body.Close()
}

func TestHttpService_PostWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.PostWithHeaders(context.Background(), "test", nil, nil, nil)

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	_ = resp.Body.Close()
}

func TestHttpService_DeleteSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.Delete(context.Background(), "test", nil)

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	_ = resp.Body.Close()
}

func TestHttpService_DeleteWithHeaderSuccessRequests(t *testing.T) {
	server := testServer()
	defer server.Close()

	mockMetric := &mockMetrics{}

	mockMetric.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	service := NewHTTPService(server.URL, testutil.NewMockLogger(testutil.DEBUGLOG), mockMetric, &CircuitBreakerConfig{
		Threshold: 1,
		Interval:  1,
	})

	resp, err := service.DeleteWithHeaders(context.Background(), "test", nil, nil)

	assert.Nil(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusOK)

	_ = resp.Body.Close()
}

type mockMetrics struct {
	mock.Mock
}

func (m *mockMetrics) RecordHistogram(ctx context.Context, name string, value float64, labels ...string) {
	m.Called(ctx, name, value, labels)
}
