package gofr

import (
	"context"
)

// Request is an interface which is written because it allows us
// to create applications without being aware of the transport.
// In both cmd or server application, this abstraction can be used.
type Request interface {
	Context() context.Context
	Param(string) string
	PathParam(string) string
	Bind(interface{}) error
	HostName() string
	Params(string) []string
}
