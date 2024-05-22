package errors

import (
	"github.com/pkg/errors"
)

// Response represents a generic GoFr error response struct which can be populated with any underlying error, custom message.
// and status code.
type Response struct {
	Err          error
	Message      string
	ResponseCode int
}

// Error returns the formatted error message.
func (e Response) Error() string {
	switch {
	case e.Message == "":
		return e.Err.Error()
	case e.Err == nil:
		return e.Message
	default:
		return errors.Wrap(e.Err, e.Message).Error()
	}
}

func (e Response) WithStack() Response {
	e.Err = errors.WithStack(e.Err)
	return e
}

func (e Response) StatusCode() int {
	return e.ResponseCode
}
