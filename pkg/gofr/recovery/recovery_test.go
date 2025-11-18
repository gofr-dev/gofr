package recovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func TestNew(t *testing.T) {
	logger := logging.NewLogger(logging.DEBUG)
	ctrl := gomock.NewController(t)
	metrics := container.NewMockMetrics(ctrl)

	h := New(logger, metrics)

	assert.NotNil(t, h)
	assert.Equal(t, logger, h.logger)
	assert.Equal(t, metrics, h.metrics)
}

func TestRecover_WithNilPanic(t *testing.T) {
	logger := logging.NewLogger(logging.DEBUG)
	ctrl := gomock.NewController(t)
	metrics := container.NewMockMetrics(ctrl)

	h := New(logger, metrics)

	// Should not panic or log anything
	h.Recover(context.Background(), nil)
}

func TestRecover_WithStringPanic(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		logger := logging.NewLogger(logging.DEBUG)
		ctrl := gomock.NewController(t)
		metrics := container.NewMockMetrics(ctrl)

		metrics.EXPECT().IncrementCounter(gomock.Any(), "panic_total", "type", "string").Times(1)

		h := New(logger, metrics)
		h.Recover(context.Background(), "test panic")
	})

	assert.Contains(t, logs, "test panic")
	assert.Contains(t, logs, "recovery_test.go")
}

func TestRecover_WithErrorPanic(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		logger := logging.NewLogger(logging.DEBUG)
		ctrl := gomock.NewController(t)
		metrics := container.NewMockMetrics(ctrl)

		metrics.EXPECT().IncrementCounter(gomock.Any(), "panic_total", "type", "error").Times(1)

		h := New(logger, metrics)
		h.Recover(context.Background(), assert.AnError)
	})

	assert.Contains(t, logs, "assert.AnError")
	assert.Contains(t, logs, "recovery_test.go")
}

func TestRecover_WithUnknownPanic(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		logger := logging.NewLogger(logging.DEBUG)
		ctrl := gomock.NewController(t)
		metrics := container.NewMockMetrics(ctrl)

		metrics.EXPECT().IncrementCounter(gomock.Any(), "panic_total", "type", "unknown").Times(1)

		h := New(logger, metrics)
		h.Recover(context.Background(), 42)
	})

	assert.Contains(t, logs, "42")
	assert.Contains(t, logs, "recovery_test.go")
}

func TestRecover_WithNilMetrics(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		logger := logging.NewLogger(logging.DEBUG)

		h := New(logger, nil)
		// Should not panic even with nil metrics
		h.Recover(context.Background(), "test panic")
	})

	assert.Contains(t, logs, "test panic")
}

func TestHTTPRecoverMiddleware_NoPanic(t *testing.T) {
	logger := logging.NewLogger(logging.DEBUG)
	ctrl := gomock.NewController(t)
	metrics := container.NewMockMetrics(ctrl)

	h := New(logger, metrics)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	middleware := h.HTTPRecoverMiddleware(nextHandler)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

	middleware.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

func TestHTTPRecoverMiddleware_WithPanic(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		logger := logging.NewLogger(logging.DEBUG)
		ctrl := gomock.NewController(t)
		metrics := container.NewMockMetrics(ctrl)

		metrics.EXPECT().IncrementCounter(gomock.Any(), "panic_total", "type", "string").Times(1)

		h := New(logger, metrics)

		nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			panic("test panic")
		})

		middleware := h.HTTPRecoverMiddleware(nextHandler)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/test", http.NoBody)

		middleware.ServeHTTP(w, r)

		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.Contains(t, w.Body.String(), "Some unexpected error has occurred")
	})

	assert.Contains(t, logs, "test panic")
}

func TestGoSafe_NoPanic(t *testing.T) {
	logger := logging.NewLogger(logging.DEBUG)
	ctrl := gomock.NewController(t)
	metrics := container.NewMockMetrics(ctrl)

	h := New(logger, metrics)

	executed := false
	h.GoSafe(context.Background(), func() {
		executed = true
	})

	// Give goroutine time to execute
	time.Sleep(10 * time.Millisecond)
	assert.True(t, executed, "goroutine should execute")
}

func TestGoSafe_WithPanic(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		logger := logging.NewLogger(logging.DEBUG)
		ctrl := gomock.NewController(t)
		metrics := container.NewMockMetrics(ctrl)

		metrics.EXPECT().IncrementCounter(gomock.Any(), "panic_total", "type", "string").Times(1)

		h := New(logger, metrics)

		h.GoSafe(context.Background(), func() {
			panic("goroutine panic")
		})

		// Give goroutine time to execute and panic
		time.Sleep(10 * time.Millisecond)
	})

	assert.Contains(t, logs, "goroutine panic")
}

func TestSafeCronFunc_NoPanic(t *testing.T) {
	logger := logging.NewLogger(logging.DEBUG)
	ctrl := gomock.NewController(t)
	metrics := container.NewMockMetrics(ctrl)

	h := New(logger, metrics)

	executed := false
	safeFn := h.SafeCronFunc(func() {
		executed = true
	})

	safeFn()

	assert.True(t, executed)
}

func TestSafeCronFunc_WithPanic(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		logger := logging.NewLogger(logging.DEBUG)
		ctrl := gomock.NewController(t)
		metrics := container.NewMockMetrics(ctrl)

		metrics.EXPECT().IncrementCounter(gomock.Any(), "panic_total", "type", "string").Times(1)

		h := New(logger, metrics)

		safeFn := h.SafeCronFunc(func() {
			panic("cron panic")
		})

		safeFn()
	})

	assert.Contains(t, logs, "cron panic")
}

func TestRunSafeCommand_NoError(t *testing.T) {
	logger := logging.NewLogger(logging.DEBUG)
	ctrl := gomock.NewController(t)
	metrics := container.NewMockMetrics(ctrl)

	h := New(logger, metrics)

	executed := false
	err := h.RunSafeCommand(context.Background(), func() error {
		executed = true
		return nil
	})

	assert.True(t, executed)
	assert.NoError(t, err)
}

func TestRunSafeCommand_WithError(t *testing.T) {
	logger := logging.NewLogger(logging.DEBUG)
	ctrl := gomock.NewController(t)
	metrics := container.NewMockMetrics(ctrl)

	h := New(logger, metrics)

	err := h.RunSafeCommand(context.Background(), func() error {
		return assert.AnError
	})

	assert.Equal(t, assert.AnError, err)
}

func TestRunSafeCommand_WithPanic(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		logger := logging.NewLogger(logging.DEBUG)
		ctrl := gomock.NewController(t)
		metrics := container.NewMockMetrics(ctrl)

		metrics.EXPECT().IncrementCounter(gomock.Any(), "panic_total", "type", "string").Times(1)

		h := New(logger, metrics)

		err := h.RunSafeCommand(context.Background(), func() error {
			panic("command panic")
			return nil
		})

		assert.NoError(t, err) // Panic is recovered, not returned as error
	})

	assert.Contains(t, logs, "command panic")
}

func TestGetPanicType_String(t *testing.T) {
	assert.Equal(t, "string", getPanicType("test"))
}

func TestGetPanicType_Error(t *testing.T) {
	assert.Equal(t, "error", getPanicType(assert.AnError))
}

func TestGetPanicType_Unknown(t *testing.T) {
	assert.Equal(t, "unknown", getPanicType(42))
	assert.Equal(t, "unknown", getPanicType(3.14))
	assert.Equal(t, "unknown", getPanicType([]int{1, 2, 3}))
}
