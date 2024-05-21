package error

import (
	"net/http"

	"github.com/pkg/errors"
)

// ErrGoFr represents a generic GoFr error.
type ErrGoFr struct {
	Err     error
	Message string
}

// Error returns the formatted error message.
func (e ErrGoFr) Error() string {
	switch {
	case e.Message == "":
		return e.Err.Error()
	case e.Err == nil:
		return e.Message
	default:
		return errors.Wrap(e.Err, e.Message).Error()
	}
}

func (e ErrGoFr) WithStack() ErrGoFr {
	e.Err = errors.WithStack(e.Err)
	return e
}

func (e ErrGoFr) StatusCode() int {
	return http.StatusInternalServerError
}
