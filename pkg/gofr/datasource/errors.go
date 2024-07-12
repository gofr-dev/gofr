package datasource

import (
	"net/http"

	"github.com/pkg/errors"
)

// ErrorDB represents an error specific to database operations.
type ErrorDB struct {
	Err     error
	Message string
}

func (e ErrorDB) Error() string {
	switch {
	case e.Message == "":
		return e.Err.Error()
	case e.Err == nil:
		return e.Message
	default:
		return errors.Wrap(e.Err, e.Message).Error()
	}
}

// WithStack adds a stack trace to the Error.
func (e ErrorDB) WithStack() ErrorDB {
	e.Err = errors.WithStack(e.Err)
	return e
}

func (ErrorDB) StatusCode() int {
	return http.StatusInternalServerError
}
