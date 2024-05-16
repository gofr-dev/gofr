package datasource

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

// errorDB represents an error specific to database operations.
type errorDB struct {
	error
	message string
}

func (e *errorDB) Error() string {
	if e.error != nil && e.message != "" {
		return fmt.Sprintf("%v", e.error)
	} else if e.error != nil {
		return e.error.Error()
	}

	return e.message
}

//nolint:revive // NewError creates a new database error with the provided error and  message.
func NewError(err error, message ...string) *errorDB {
	errMsg := strings.Join(message, " ")

	if errMsg != "" {
		return &errorDB{
			error:   errors.Wrap(err, errMsg),
			message: errMsg,
		}
	}

	return &errorDB{
		error: err,
	}
}

// WithStack adds a stack trace to the NewError.
func (e *errorDB) WithStack() *errorDB {
	e.error = errors.WithStack(e.error)
	return e
}

func (e *errorDB) StatusCode() int {
	return http.StatusInternalServerError
}
