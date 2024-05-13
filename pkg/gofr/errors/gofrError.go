package errors

import (
	"fmt"
	"github.com/pkg/errors"
)

// GofrError represents a generic GoFr error
type GofrError struct {
	error
	message string
}

// Error returns the formatted error message.
func (e *GofrError) Error() string {
	if e.error != nil {
		return fmt.Sprintf("%s: %v", e.message, e.error)
	}
	return e.message
}

// NewGofrError creates a new GofrError and wraps the error with the provided message.
func NewGofrError(wrapErr error, message string) *GofrError {
	return &GofrError{
		error:   errors.Wrap(wrapErr, message),
		message: message,
	}
}
