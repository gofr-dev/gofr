package service

import (
	"context"
	"net/http"
)

type Headers struct {
	Headers map[string]string
}

func (a *Headers) AddOption(h HTTP) HTTP {
	return &CustomHeader{
		Headers: a.Headers,
		HTTP:    h,
	}
}

type CustomHeader struct {
	Headers map[string]string

	HTTP
}

func (a *CustomHeader) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return a.GetWithHeaders(ctx, path, queryParams, nil)
}

func (a *CustomHeader) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	headers map[string]string) (*http.Response, error) {
	headers = setCustomHeader(headers, a.Headers)

	return a.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

func (a *CustomHeader) Post(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return a.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *CustomHeader) PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte,
	headers map[string]string) (*http.Response, error) {
	headers = setCustomHeader(headers, a.Headers)

	return a.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *CustomHeader) Put(ctx context.Context, api string, queryParams map[string]interface{}, body []byte) (
	*http.Response, error) {
	return a.PutWithHeaders(ctx, api, queryParams, body, nil)
}

func (a *CustomHeader) PutWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte,
	headers map[string]string) (*http.Response, error) {
	headers = setCustomHeader(headers, a.Headers)

	return a.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *CustomHeader) Patch(ctx context.Context, path string, queryParams map[string]interface{}, body []byte) (
	*http.Response, error) {
	return a.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *CustomHeader) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte,
	headers map[string]string) (*http.Response, error) {
	headers = setCustomHeader(headers, a.Headers)

	return a.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *CustomHeader) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return a.DeleteWithHeaders(ctx, path, body, nil)
}

func (a *CustomHeader) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (
	*http.Response, error) {
	headers = setCustomHeader(headers, a.Headers)

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
