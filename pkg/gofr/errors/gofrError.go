package errors

import (
	"github.com/pkg/errors"
)

// ErrorResponse represents a generic GoFr error.
type ErrorResponse struct {
	Err          error
	Message      string
	ResponseCode int
}

// Error returns the formatted error message.
func (e ErrorResponse) Error() string {
	switch {
	case e.Message == "":
		return e.Err.Error()
	case e.Err == nil:
		return e.Message
	default:
		return errors.Wrap(e.Err, e.Message).Error()
	}
}

func (e ErrorResponse) WithStack() ErrorResponse {
	e.Err = errors.WithStack(e.Err)
	return e
}

func (e ErrorResponse) StatusCode() int {
	return e.ResponseCode
}
