package errors

// DB standard error to encapsulate errors encountered when executing database operations
type DB struct {
	Err error
}

// Error returns the underlying database error message
func (e DB) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}

	return "DB Error"
}
