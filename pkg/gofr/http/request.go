package http

import (
	"context"
	"net/http"
)

// Request is an abstraction over the underlying http.Request. This abstraction is useful because it allows us
// to create applications without being aware of the transport. cmd.Request is another such abstraction.
type Request struct {
	req *http.Request
}

func NewRequest(r *http.Request) *Request {
	return &Request{
		req: r,
	}
}

func (r *Request) Param(key string) string {
	return r.req.URL.Query().Get(key)
}

func (r *Request) Context() context.Context {
	return r.req.Context()
}
