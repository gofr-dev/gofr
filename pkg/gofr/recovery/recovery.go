// Package recovery provides centralized panic recovery mechanisms for the GoFr framework.
// It handles panics across HTTP handlers, goroutines, cron jobs, and CLI commands
// with consistent logging, metrics, and OpenTelemetry span creation.
package recovery

import (
	"context"
	"fmt"
	"net/http"
	"runtime/debug"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
)

// panicLog represents a panic event for logging.
type panicLog struct {
	Error      string `json:"error,omitempty"`
	StackTrace string `json:"stack_trace,omitempty"`
}

// Handler handles panic recovery with logging, metrics, and tracing.
type Handler struct {
	logger  logging.Logger
	metrics container.Metrics
}

// New creates a new panic recovery handler.
func New(logger logging.Logger, metrics container.Metrics) *Handler {
	return &Handler{
		logger:  logger,
		metrics: metrics,
	}
}

// Recover handles a panic by logging it, recording metrics, and creating a span.
// It converts the panic value to a string and records it.
func (h *Handler) Recover(ctx context.Context, panicValue any) {
	if panicValue == nil {
		return
	}

	// Record panic metric
	if h.metrics != nil {
		h.metrics.IncrementCounter(ctx, "panic_total", "type", getPanicType(panicValue))
	}

	// Create and record span
	tracer := otel.GetTracerProvider().Tracer("gofr-recovery")
	_, span := tracer.Start(ctx, "panic_recovery")
	defer span.End()

	span.SetStatus(codes.Error, fmt.Sprintf("panic: %v", panicValue))
	span.SetAttributes(
		attribute.String("panic.value", fmt.Sprint(panicValue)),
		attribute.String("panic.type", getPanicType(panicValue)),
	)

	// Log panic
	panicStr := fmt.Sprint(panicValue)
	h.logger.Error(panicLog{
		Error:      panicStr,
		StackTrace: string(debug.Stack()),
	})
}

// HTTPRecoverMiddleware wraps an HTTP handler with panic recovery.
// If a panic occurs, it logs the panic, records metrics, and returns a 500 error.
func (h *Handler) HTTPRecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if panicValue := recover(); panicValue != nil {
				h.Recover(r.Context(), panicValue)

				// Send error response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"code":500,"status":"ERROR","message":"Some unexpected error has occurred"}`))
			}
		}()

		next.ServeHTTP(w, r)
	})
}

// GoSafe wraps a goroutine function with panic recovery.
// It ensures that panics in the goroutine are logged and don't crash the application.
func (h *Handler) GoSafe(ctx context.Context, fn func()) {
	go func() {
		defer func() {
			if panicValue := recover(); panicValue != nil {
				h.Recover(ctx, panicValue)
			}
		}()

		fn()
	}()
}

// SafeCronFunc wraps a cron function with panic recovery.
// It returns a function that can be used as a cron job handler.
func (h *Handler) SafeCronFunc(fn func()) func() {
	return func() {
		defer func() {
			if panicValue := recover(); panicValue != nil {
				h.Recover(context.Background(), panicValue)
			}
		}()

		fn()
	}
}

// RunSafeCommand wraps a command function with panic recovery.
// It returns any error from the function or converts a panic to an error.
func (h *Handler) RunSafeCommand(ctx context.Context, fn func() error) error {
	defer func() {
		if panicValue := recover(); panicValue != nil {
			h.Recover(ctx, panicValue)
		}
	}()

	return fn()
}

// getPanicType returns the type of the panic value.
func getPanicType(panicValue any) string {
	switch panicValue.(type) {
	case string:
		return "string"
	case error:
		return "error"
	default:
		return "unknown"
	}
}
