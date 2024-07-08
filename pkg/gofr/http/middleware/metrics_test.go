package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/mock"
)

type mockMetrics struct {
	mock.Mock
}

func (m *mockMetrics) IncrementCounter(ctx context.Context, name string, labels ...string) {
	m.Called(ctx, name, labels)
}

func (m *mockMetrics) DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string) {
	m.Called(ctx, name, value, labels)
}

func (m *mockMetrics) RecordHistogram(ctx context.Context, name string, value float64, labels ...string) {
	m.Called(ctx, name, value, labels)
}

func (m *mockMetrics) SetGauge(name string, value float64, _ ...string) {
	m.Called(name, value)
}

func TestMetrics(t *testing.T) {
	mockMetrics := &mockMetrics{}

	mockMetrics.On("RecordHistogram", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)

	router := mux.NewRouter()
	router.HandleFunc("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet).Name("/test")

	route := router.NewRoute()
	route.Path("/test").Name("/test")

	req := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)
	rr := httptest.NewRecorder()

	router.Use(Metrics(mockMetrics))

	router.ServeHTTP(rr, req)

	mockMetrics.AssertCalled(t, "RecordHistogram", mock.Anything, "app_http_response", mock.Anything,
		[]string{"path", "/test", "method", "GET", "status", "200"})
}
