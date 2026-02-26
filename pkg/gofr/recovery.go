package gofr

import (
	"errors"
	"fmt"
	"runtime/debug"

	"gofr.dev/pkg/gofr/logging"
)

// ErrPanic is the base error for panic recovery.
var ErrPanic = errors.New("panic in component")

// RecoveryLog represents the structure of a panic recovery log entry.
type RecoveryLog struct {
	Component  string `json:"component,omitempty"`
	Error      string `json:"error,omitempty"`
	StackTrace string `json:"stack_trace,omitempty"`
}

// RecoveryHandler is a centralized panic recovery mechanism that can be used
// across all components including cron jobs, command executions, and goroutines.
type RecoveryHandler struct {
	logger    logging.Logger
	component string
}

// NewRecoveryHandler creates a new RecoveryHandler with the specified logger and component name.
func NewRecoveryHandler(logger logging.Logger, component string) *RecoveryHandler {
	return &RecoveryHandler{
		logger:    logger,
		component: component,
	}
}

// Recover handles panic recovery and logs the error with stack trace.
// It should be called with defer at the beginning of any function that needs panic recovery.
func (r *RecoveryHandler) Recover() {
	if rec := recover(); rec != nil { //nolint:revive
		_ = r.handlePanic(rec)
	}
}

// RecoverWithCallback handles panic recovery and executes a callback function if a panic occurs.
func (r *RecoveryHandler) RecoverWithCallback(callback func(error)) {
	if rec := recover(); rec != nil { //nolint:revive
		err := r.handlePanic(rec)
		if callback != nil {
			callback(err)
		}
	}
}

// RecoverWithChannel handles panic recovery and sends a signal to a channel if a panic occurs.
func (r *RecoveryHandler) RecoverWithChannel(panicChan chan<- struct{}) {
	if rec := recover(); rec != nil { //nolint:revive
		_ = r.handlePanic(rec)
		if panicChan != nil {
			close(panicChan)
		}
	}
}

// handlePanic processes the panic value, logs it, and returns an error.
func (r *RecoveryHandler) handlePanic(rec any) error {
	var errMsg string

	switch t := rec.(type) {
	case string:
		errMsg = t
	case error:
		errMsg = t.Error()
	default:
		errMsg = fmt.Sprintf("%v", rec)
	}

	err := fmt.Errorf("%w: %s - %s", ErrPanic, r.component, errMsg)

	r.logger.Error(RecoveryLog{
		Component:  r.component,
		Error:      errMsg,
		StackTrace: string(debug.Stack()),
	})

	return err
}

// SafeGo wraps a goroutine with panic recovery.
func SafeGo(logger logging.Logger, component string, fn func()) {
	go func() {
		defer NewRecoveryHandler(logger, component).Recover()
		fn()
	}()
}

// SafeGoWithCallback wraps a goroutine with panic recovery and executes a callback on panic.
func SafeGoWithCallback(logger logging.Logger, component string, fn func(), callback func(error)) {
	go func() {
		defer NewRecoveryHandler(logger, component).RecoverWithCallback(callback)
		fn()
	}()
}
