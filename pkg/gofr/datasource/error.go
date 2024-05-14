package datasource

import (
	"fmt"
	"net/http"

	"github.com/pkg/errors"
)

// DBErr represents an error specific to database operations.
type DBErr struct {
	error
	message string
}

func (e *DBErr) Error() string {
	if e.error != nil && e.message != "" {
		return fmt.Sprintf("%v", e.error)
	} else if e.error != nil {
		return e.error.Error()
	}

	return e.message
}

// DBError creates a new DBError with the provided error and  message.
func DBError(err error, message ...string) *DBErr {
	return &DBErr{
		error:   errors.Wrap(err, message[0]),
		message: message[0],
	}
}

// WithStack adds a stack trace to the DBError.
func (e *DBErr) WithStack() *DBErr {
	e.error = errors.WithStack(e.error)
	return e
}

func (e *DBErr) StatusCode() int {
	return http.StatusInternalServerError
}
