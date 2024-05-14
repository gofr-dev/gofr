// Package service provides an HTTP client with features for logging, metrics, and resilience.It supports various
// functionalities like health checks, circuit-breaker and various authentication.
package service

import (
	"context"
	"net/http"
)

type CustomHeader struct {
	Header map[string]string
}

func (a *CustomHeader) AddOption(h HTTP) HTTP {
	return &Header{Header: a.Header,
		HTTP: h,
	}
}

type Header struct {
	Header map[string]string

	HTTP
}

func (a *Header) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return a.GetWithHeaders(ctx, path, queryParams, nil)
}

func (a *Header) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	headers map[string]string) (*http.Response, error) {
	headers = setCustomHeader(headers, a.Header)

	return a.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

func (a *Header) Post(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return a.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *Header) PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte,
	headers map[string]string) (*http.Response, error) {
	headers = setCustomHeader(headers, a.Header)

	return a.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *Header) Put(ctx context.Context, api string, queryParams map[string]interface{}, body []byte) (
	*http.Response, error) {
	return a.PutWithHeaders(ctx, api, queryParams, body, nil)
}

func (a *Header) PutWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte,
	headers map[string]string) (*http.Response, error) {
	headers = setCustomHeader(headers, a.Header)

	return a.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *Header) Patch(ctx context.Context, path string, queryParams map[string]interface{}, body []byte) (
	*http.Response, error) {
	return a.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *Header) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte,
	headers map[string]string) (*http.Response, error) {
	headers = setCustomHeader(headers, a.Header)

	return a.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *Header) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return a.DeleteWithHeaders(ctx, path, body, nil)
}

func (a *Header) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (
	*http.Response, error) {
	headers = setCustomHeader(headers, a.Header)

	return a.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}

func setCustomHeader(headers, customHeader map[string]string) map[string]string {
	if headers == nil {
		headers = make(map[string]string)
	}

	for key, value := range customHeader {
		headers[key] = value
	}

	return headers
}
