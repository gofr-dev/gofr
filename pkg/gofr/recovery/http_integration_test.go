package recovery

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// TestHTTPRecoverMiddleware_PanicLogging verifies that panics are properly logged.
func TestHTTPRecoverMiddleware_PanicLogging(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		logger := logging.NewLogger(logging.DEBUG)
		ctrl := gomock.NewController(t)
		metrics := container.NewMockMetrics(ctrl)

		metrics.EXPECT().IncrementCounter(gomock.Any(), "panic_total", gomock.Any(), gomock.Any()).Times(1)

		h := New(logger, metrics)

		handler := h.HTTPRecoverMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic in middleware")
		}))

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

		handler.ServeHTTP(w, r)
	})

	assert.Contains(t, logs, "test panic in middleware")
	assert.Contains(t, logs, "stack_trace")
}

// TestHTTPRecoverMiddleware_ResponseHeaders verifies that proper headers are set on panic response.
func TestHTTPRecoverMiddleware_ResponseHeaders(t *testing.T) {
	logger := logging.NewLogger(logging.DEBUG)
	ctrl := gomock.NewController(t)
	metrics := container.NewMockMetrics(ctrl)

	metrics.EXPECT().IncrementCounter(gomock.Any(), "panic_total", gomock.Any(), gomock.Any()).Times(1)

	h := New(logger, metrics)

	handler := h.HTTPRecoverMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

	handler.ServeHTTP(w, r)

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// TestHTTPRecoverMiddleware_NestedMiddleware verifies that recovery works with other middleware.
func TestHTTPRecoverMiddleware_NestedMiddleware(t *testing.T) {
	logger := logging.NewLogger(logging.DEBUG)
	ctrl := gomock.NewController(t)
	metrics := container.NewMockMetrics(ctrl)

	metrics.EXPECT().IncrementCounter(gomock.Any(), "panic_total", gomock.Any(), gomock.Any()).Times(1)

	h := New(logger, metrics)

	// Inner handler that panics
	innerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("inner panic")
	})

	// Wrap with recovery middleware
	recoveryHandler := h.HTTPRecoverMiddleware(innerHandler)

	// Wrap with another middleware that sets a header
	outerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Custom", "value")
		recoveryHandler.ServeHTTP(w, r)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

	outerHandler.ServeHTTP(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}
