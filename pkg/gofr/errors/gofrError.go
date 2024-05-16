package gofrerror

import (
	"fmt"
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
	if e.error != nil && e.message != "" {
		return fmt.Sprintf("%v", e.error)
	} else if e.error != nil {
		return e.error.Error()
	}

	return e.message
}

//nolint:revive // NewError creates a new GoFr error and wraps the error with the provided message.
func NewError(err error, message ...string) *errorGoFr {
	errMsg := strings.Join(message, " ")

	if errMsg != "" {
		return &errorGoFr{
			error:   errors.Wrap(err, errMsg),
			message: errMsg,
		}
	}

	return &errorGoFr{
		error: err,
	}
}

func (e *errorGoFr) StatusCode() int {
	return http.StatusInternalServerError
}
