package service

import "net/http"

// Response represents an HTTP response with body, status code, and headers.
type Response struct {
	Body       []byte
	StatusCode int
	headers    http.Header
}

// GetHeader fetches the value of a specified HTTP header,
func (r *Response) GetHeader(key string) string {
	if r.headers != nil {
		return r.headers.Get(key)
	}

	return ""
}
