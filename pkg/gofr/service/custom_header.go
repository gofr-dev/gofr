package service

import (
	"context"
	"net/http"
)

type DefaultHeaders struct {
	Headers map[string]string
}

func (a *DefaultHeaders) AddOption(h HTTP) HTTP {
	return &customHeader{
		Headers: a.Headers,
		HTTP:    h,
	}
}

type customHeader struct {
	Headers map[string]string

	HTTP
}

func (a *customHeader) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response, error) {
	return a.GetWithHeaders(ctx, path, queryParams, nil)
}

func (a *customHeader) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	headers = setCustomHeader(headers, a.Headers)

	return a.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

func (a *customHeader) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return a.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *customHeader) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	headers = setCustomHeader(headers, a.Headers)

	return a.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *customHeader) Put(ctx context.Context, api string, queryParams map[string]any, body []byte) (
	*http.Response, error) {
	return a.PutWithHeaders(ctx, api, queryParams, body, nil)
}

func (a *customHeader) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	headers = setCustomHeader(headers, a.Headers)

	return a.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *customHeader) Patch(ctx context.Context, path string, queryParams map[string]any, body []byte) (
	*http.Response, error) {
	return a.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *customHeader) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	headers = setCustomHeader(headers, a.Headers)

	return a.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *customHeader) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return a.DeleteWithHeaders(ctx, path, body, nil)
}

func (a *customHeader) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (
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
