package auth

import (
	"errors"
	"fmt"
)

var errEmptyTokenFile = errors.New("token file is empty")

// Err represents an authentication configuration error.
type Err struct {
	Err     error
	Message string
}

func (o Err) Error() string {
	switch {
	case o.Message == "" && o.Err == nil:
		return "unknown error"
	case o.Message == "":
		return o.Err.Error()
	case o.Err == nil:
		return o.Message
	default:
		return fmt.Sprintf("%v: %v", o.Message, o.Err)
	}
}

func (o Err) Unwrap() error {
	return o.Err
}
