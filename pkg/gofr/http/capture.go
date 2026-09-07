package http

import (
	"bytes"
	"net/http"
)

// ResponseCapture is an http.ResponseWriter that records the status, headers and a size-capped body
// of a response produced in-process — for example when a request is dispatched through the router
// without a network client. Bytes past the cap are dropped but reported as written so the handler
// is not disrupted.
type ResponseCapture struct {
	header http.Header
	body   bytes.Buffer
	status int
	max    int
	wrote  bool
}

// NewResponseCapture returns a ResponseCapture that retains at most maxBytes of the response body.
func NewResponseCapture(maxBytes int) *ResponseCapture {
	return &ResponseCapture{header: make(http.Header), status: http.StatusOK, max: maxBytes}
}

func (c *ResponseCapture) Header() http.Header { return c.header }

func (c *ResponseCapture) WriteHeader(status int) {
	if !c.wrote {
		c.status = status
		c.wrote = true
	}
}

func (c *ResponseCapture) Write(b []byte) (int, error) {
	c.wrote = true

	room := c.max - c.body.Len()
	if room <= 0 {
		return len(b), nil
	}

	if len(b) > room {
		c.body.Write(b[:room])

		return len(b), nil
	}

	return c.body.Write(b)
}

// Status returns the recorded status code (200 if the handler never set one).
func (c *ResponseCapture) Status() int { return c.status }

// Body returns the recorded, possibly truncated, response body.
func (c *ResponseCapture) Body() []byte { return c.body.Bytes() }
