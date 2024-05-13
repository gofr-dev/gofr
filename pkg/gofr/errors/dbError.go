package errors

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"
)

// DBError represents an error specific to database operations.
type DBError struct {
	*GofrError
}

// NewDBError creates a new DBError with the provided error, message, and optional arguments for formatting.
func NewDBError(err error, message string, a ...interface{}) *DBError {
	return &DBError{
		GofrError: NewGofrError(err, fmt.Sprintf(message, a...)),
	}
}

// WithStack adds a stack trace to the DBError.
func (e *DBError) WithStack() *DBError {
	e.error = errors.WithStack(e.error)
	return e
}

func (e *DBError) StatusCode() int {
	return http.StatusInternalServerError
}
