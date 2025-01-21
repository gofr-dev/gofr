// Package service provides an HTTP client with features for logging, metrics, and resilience.It supports various
// functionalities like health checks, circuit-breaker and various authentication.
package service

import (
	"context"
	"net/http"
)

type APIKeyConfig struct {
	APIKey string
}

func (a *APIKeyConfig) AddOption(h HTTP) HTTP {
	return &apiKeyAuthProvider{
		apiKey: a.APIKey,
		HTTP:   h,
	}
}

type apiKeyAuthProvider struct {
	apiKey string

	HTTP
}

func (a *apiKeyAuthProvider) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response, error) {
	return a.GetWithHeaders(ctx, path, queryParams, nil)
}

func (a *apiKeyAuthProvider) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	headers = setXApiKey(headers, a.apiKey)

	return a.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

func (a *apiKeyAuthProvider) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return a.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *apiKeyAuthProvider) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	headers = setXApiKey(headers, a.apiKey)

	return a.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *apiKeyAuthProvider) Put(ctx context.Context, api string, queryParams map[string]any, body []byte) (
	*http.Response, error) {
	return a.PutWithHeaders(ctx, api, queryParams, body, nil)
}

func (a *apiKeyAuthProvider) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	headers = setXApiKey(headers, a.apiKey)

	return a.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *apiKeyAuthProvider) Patch(ctx context.Context, path string, queryParams map[string]any, body []byte) (
	*http.Response, error) {
	return a.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *apiKeyAuthProvider) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error) {
	headers = setXApiKey(headers, a.apiKey)

	return a.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *apiKeyAuthProvider) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return a.DeleteWithHeaders(ctx, path, body, nil)
}

func (a *apiKeyAuthProvider) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (
	*http.Response, error) {
	headers = setXApiKey(headers, a.apiKey)

	return a.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}

func setXApiKey(headers map[string]string, apiKey string) map[string]string {
	if headers == nil {
		headers = make(map[string]string)
	}

	headers["X-API-KEY"] = apiKey

	return headers
}
