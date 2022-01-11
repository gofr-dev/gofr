package service

import "net/http"

type Response struct {
	Body       []byte
	StatusCode int
	headers    http.Header
}

func (r *Response) GetHeader(key string) string {
	if r.headers != nil {
		return r.headers.Get(key)
	}

	return ""
}
