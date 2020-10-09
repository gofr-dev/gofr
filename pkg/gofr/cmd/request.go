package cmd

import (
	"context"
)

// Request is an abstraction over the actual command with flags. This abstraction is useful because it allows us
// to create cmd applications in same way we would create a HTTP server application.
// Gofr's http.Request is another such abstraction.
type Request struct {
	flags  map[string]bool
	params map[string]string
}

// TODO - use statement to parse the request to populate the flags and params.

// NewRequest creates a Request from a statement. This way we can simulate running a command without actually
// doing it. It makes the code more testable this way.
func NewRequest(statement string) *Request {
	r := Request{
		flags:  make(map[string]bool),
		params: make(map[string]string),
	}

	return &r
}

// Param returns the value of the parameter for key.
func (r *Request) Param(key string) string {
	return r.params[key]
}

func (r *Request) Context() context.Context {
	return context.Background()
}
