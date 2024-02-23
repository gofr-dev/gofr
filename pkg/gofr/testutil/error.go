package testutil

type CustomError struct {
	ErrorMessage string
}

func (c CustomError) Error() string {
	return c.ErrorMessage
}
