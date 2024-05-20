package error

import (
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

// gofrError represents a generic GoFr error.
type gofrError struct {
	error
	message string
}

// Error returns the formatted error message.
func (e *gofrError) Error() string {
	return e.error.Error()
}

//nolint:revive // New creates a new GoFr error with provided message.
func New(message string) *gofrError {
	return &gofrError{
		error:   errors.New(message),
		message: message,
	}
}

//nolint:revive // NewWrapped creates a new GoFr error and wraps the error with the provided message.
func NewWrapped(err error, message ...string) *gofrError {
	errMsg := strings.Join(message, " ")

	if err != nil && errMsg != "" {
		return &gofrError{
			error:   errors.Wrap(err, errMsg),
			message: errMsg,
		}
	}

	if errMsg != "" {
		return New(errMsg)
	}

	return &gofrError{
		error: err,
	}
}

func (e *gofrError) StatusCode() int {
	return http.StatusInternalServerError
}
