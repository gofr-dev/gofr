package error

import (
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

// errorGoFr represents a generic GoFr error.
type errorGoFr struct {
	error
	message string
}

// Error returns the formatted error message.
func (e *errorGoFr) Error() string {
	return e.error.Error()
}

//nolint:revive // New creates a new GoFr error with provided message.
func New(message string) *errorGoFr {
	return &errorGoFr{
		error:   errors.New(message),
		message: message,
	}
}

//nolint:revive // NewWrapped creates a new GoFr error and wraps the error with the provided message.
func NewWrapped(err error, message ...string) *errorGoFr {
	errMsg := strings.Join(message, " ")

	if err != nil && errMsg != "" {
		return &errorGoFr{
			error:   errors.Wrap(err, errMsg),
			message: errMsg,
		}
	}

	if errMsg != "" {
		return New(errMsg)
	}

	return &errorGoFr{
		error: err,
	}
}

func (e *errorGoFr) StatusCode() int {
	return http.StatusInternalServerError
}
