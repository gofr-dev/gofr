package datasource

import (
	"database/sql"
	"github.com/gocql/gocql"
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

func (e ErrorDB) StatusCode() int {
	if errors.Is(e.Err, sql.ErrNoRows) || errors.Is(e.Err, gocql.ErrNotFound) {
		return http.StatusNotFound
	}

	return http.StatusInternalServerError
}
