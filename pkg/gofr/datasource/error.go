package datasource

import (
	"net/http"

	"github.com/pkg/errors"
)

// ErrDB represents an error specific to database operations.
type ErrDB struct {
	Err     error
	Message string
}

func (e ErrDB) Error() string {
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
func (e ErrDB) WithStack() ErrDB {
	e.Err = errors.WithStack(e.Err)
	return e
}

func (e ErrDB) StatusCode() int {
	return http.StatusInternalServerError
}
