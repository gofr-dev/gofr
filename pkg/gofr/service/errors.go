package service

import "github.com/pkg/errors"

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
		return errors.Wrap(o.Err, o.Message).Error()
	}
}
