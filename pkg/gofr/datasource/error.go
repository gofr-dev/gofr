package datasource

import (
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
	return e.error.Error()
}

//nolint:revive // Error creates a new DB error with provided message.
func Error(message string) *errorDB {
	return &errorDB{
		error:   errors.New(message),
		message: message,
	}
}

//nolint:revive // ErrorWrapped creates a new database error with the provided error and  message.
func ErrorWrapped(err error, message ...string) *errorDB {
	errMsg := strings.Join(message, " ")

	if err != nil && errMsg != "" {
		return &errorDB{
			error:   errors.Wrap(err, errMsg),
			message: errMsg,
		}
	}

	if errMsg != "" {
		return Error(errMsg)
	}

	return &errorDB{
		error: err,
	}
}

// WithStack adds a stack trace to the Error.
func (e *errorDB) WithStack() *errorDB {
	e.error = errors.WithStack(e.error)
	return e
}

func (e *errorDB) StatusCode() int {
	return http.StatusInternalServerError
}
