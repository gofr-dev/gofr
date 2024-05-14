package errors

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"
)

// GofrError represents a generic GoFr error.
type GofrError struct {
	error
	message string
}

// Error returns the formatted error message.
func (e *GofrError) Error() string {
	if e.error != nil && e.message != "" {
		return fmt.Sprintf("%v", e.error)
	} else if e.error != nil {
		return e.error.Error()
	}

	return e.message
}

// New creates a new GofrError and wraps the error with the provided message.
func New(err error, message ...string) *GofrError {
	return &GofrError{
		error:   errors.Wrap(err, message[0]),
		message: message[0],
	}
}

func (e *GofrError) StatusCode() int {
	return http.StatusInternalServerError
}
