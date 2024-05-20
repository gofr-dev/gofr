package datasource

import (
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

// dbError represents an error specific to database operations.
type dbError struct {
	error
	message string
}

func (e *dbError) Error() string {
	return e.error.Error()
}

//nolint:revive // Error creates a new DB error with provided message.
func Error(message string) *dbError {
	return &dbError{
		error:   errors.New(message),
		message: message,
	}
}

//nolint:revive // ErrorWrapped creates a new database error with the provided error and  message.
func ErrorWrapped(err error, message ...string) *dbError {
	errMsg := strings.Join(message, " ")

	if err != nil && errMsg != "" {
		return &dbError{
			error:   errors.Wrap(err, errMsg),
			message: errMsg,
		}
	}

	if errMsg != "" {
		return Error(errMsg)
	}

	return &dbError{
		error: err,
	}
}

// WithStack adds a stack trace to the Error.
func (e *dbError) WithStack() *dbError {
	e.error = errors.WithStack(e.error)
	return e
}

func (e *dbError) StatusCode() int {
	return http.StatusInternalServerError
}
