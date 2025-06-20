package service

import (
	"fmt"
)

type OAuthErr struct {
	Err     error
	Message string
}

func (o OAuthErr) Error() string {
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
