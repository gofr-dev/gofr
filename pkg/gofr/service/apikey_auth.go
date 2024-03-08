package service

import (
	"context"
	"net/http"
)

type ApiKeyAuth struct {
	ApiKey string
}

func (a *ApiKeyAuth) addOption(h HTTP) HTTP {
	return &ApiKeyAuthProvider{
		apiKey: a.ApiKey,
		HTTP:   h,
	}
}

type ApiKeyAuthProvider struct {
	apiKey string

	HTTP
}

func (a *ApiKeyAuthProvider) addAuthorizationHeader(headers map[string]string) error {
	headers["X-API-KEY"] = a.apiKey

	return nil
}

func (a *ApiKeyAuthProvider) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return a.GetWithHeaders(ctx, path, queryParams, nil)
}

func (a *ApiKeyAuthProvider) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, headers map[string]string) (*http.Response, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	err := a.addAuthorizationHeader(headers)
	if err != nil {
		return nil, err
	}

	return a.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

func (a *ApiKeyAuthProvider) Post(ctx context.Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return a.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *ApiKeyAuthProvider) PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	err := a.addAuthorizationHeader(headers)
	if err != nil {
		return nil, err
	}

	return a.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *ApiKeyAuthProvider) Put(ctx context.Context, api string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return a.PutWithHeaders(ctx, api, queryParams, body, nil)
}

func (a *ApiKeyAuthProvider) PutWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	err := a.addAuthorizationHeader(headers)
	if err != nil {
		return nil, err
	}

	return a.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *ApiKeyAuthProvider) Patch(ctx context.Context, path string, queryParams map[string]interface{}, body []byte) (*http.Response, error) {
	return a.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *ApiKeyAuthProvider) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte, headers map[string]string) (*http.Response, error) {
	err := a.addAuthorizationHeader(headers)
	if headers == nil {
		headers = make(map[string]string)
	}

	if err != nil {
		return nil, err
	}

	return a.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *ApiKeyAuthProvider) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return a.DeleteWithHeaders(ctx, path, body, nil)
}

func (a *ApiKeyAuthProvider) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (*http.Response, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	err := a.addAuthorizationHeader(headers)
	if err != nil {
		return nil, err
	}

	return a.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}
