package service

import (
	"context"
	"net/http"
)

type APIKeyConfig struct {
	APIKey string
}

func (a *APIKeyConfig) addOption(h HTTP) HTTP {
	return &APIKeyAuthProvider{
		apiKey: a.APIKey,
		HTTP:   h,
	}
}

type APIKeyAuthProvider struct {
	apiKey string

	HTTP
}

func (a *APIKeyAuthProvider) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return a.GetWithHeaders(ctx, path, queryParams, nil)
}

func (a *APIKeyAuthProvider) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	headers map[string]string) (*http.Response, error) {
	setXApiKey(headers, a.apiKey)

	return a.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

func (a *APIKeyAuthProvider) Post(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return a.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *APIKeyAuthProvider) PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte,
	headers map[string]string) (*http.Response, error) {
	setXApiKey(headers, a.apiKey)

	return a.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *APIKeyAuthProvider) Put(ctx context.Context, api string, queryParams map[string]interface{}, body []byte) (
	*http.Response, error) {
	return a.PutWithHeaders(ctx, api, queryParams, body, nil)
}

func (a *APIKeyAuthProvider) PutWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte,
	headers map[string]string) (*http.Response, error) {
	setXApiKey(headers, a.apiKey)

	return a.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *APIKeyAuthProvider) Patch(ctx context.Context, path string, queryParams map[string]interface{}, body []byte) (
	*http.Response, error) {
	return a.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *APIKeyAuthProvider) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte,
	headers map[string]string) (*http.Response, error) {
	setXApiKey(headers, a.apiKey)

	return a.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *APIKeyAuthProvider) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return a.DeleteWithHeaders(ctx, path, body, nil)
}

func (a *APIKeyAuthProvider) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (
	*http.Response, error) {
	setXApiKey(headers, a.apiKey)

	return a.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}

func setXApiKey(headers map[string]string, apiKey string) {
	if headers == nil {
		headers = make(map[string]string)
	}

	headers["X-API-KEY"] = apiKey
}
