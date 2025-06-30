package service

import (
	"context"
	"net/http"
)

const AuthHeader = "Authorization"

type authProvider struct {
	HTTP
}

func (a *authProvider) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response, error) {
	return a.GetWithHeaders(ctx, path, queryParams, nil)
}

func (a *authProvider) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	return a.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

func (a *authProvider) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return a.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *authProvider) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return a.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *authProvider) Patch(ctx context.Context, path string, queryParams map[string]any, body []byte) (*http.Response, error) {
	return a.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *authProvider) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return a.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *authProvider) Put(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return a.PutWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *authProvider) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return a.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *authProvider) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return a.DeleteWithHeaders(ctx, path, body, nil)
}

func (a *authProvider) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (*http.Response, error) {
	return a.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}

func (a *authProvider) addAuthorizationHeader(ctx context.Context, headers map[string]string) (map[string]string, error) {
	return headers, nil
}
