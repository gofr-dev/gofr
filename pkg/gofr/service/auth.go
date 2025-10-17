package service

import (
	"context"
	"net/http"
)

const AuthHeader = "Authorization"

type authProvider struct {
	auth func(context.Context, map[string]string) (map[string]string, error)
	HTTP
}

type WithHeaderType func(ctx context.Context, path string, queryParams map[string]any, body []byte,
	headers map[string]string) (*http.Response, error)

func (a *authProvider) doAuthWithHeaders(
	ctx context.Context,
	withHeader WithHeaderType,
	path string,
	queryParams map[string]any,
	body []byte,
	headers map[string]string,
) (*http.Response, error) {
	headers, err := a.auth(ctx, headers)
	if err != nil {
		return nil, err
	}

	return withHeader(ctx, path, queryParams, body, headers)
}

func (a *authProvider) Get(ctx context.Context,
	path string, queryParams map[string]any) (*http.Response, error) {
	return a.GetWithHeaders(ctx, path, queryParams, nil)
}

func (a *authProvider) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	return a.doAuthWithHeaders(ctx,
		func(ctx context.Context, path string, queryParams map[string]any, _ []byte, headers map[string]string) (*http.Response, error) {
			return a.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
		}, path, queryParams, nil, headers)
}

func (a *authProvider) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return a.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *authProvider) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return a.doAuthWithHeaders(ctx, a.HTTP.PostWithHeaders, path, queryParams, body, headers)
}

func (a *authProvider) Patch(ctx context.Context, path string, queryParams map[string]any, body []byte) (*http.Response, error) {
	return a.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *authProvider) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return a.doAuthWithHeaders(ctx, a.HTTP.PatchWithHeaders, path, queryParams, body, headers)
}

func (a *authProvider) Put(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return a.PutWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *authProvider) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	return a.doAuthWithHeaders(ctx, a.HTTP.PutWithHeaders, path, queryParams, body, headers)
}

func (a *authProvider) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return a.DeleteWithHeaders(ctx, path, body, nil)
}

func (a *authProvider) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (*http.Response, error) {
	return a.doAuthWithHeaders(ctx,
		func(ctx context.Context, path string, _ map[string]any, body []byte, headers map[string]string) (*http.Response, error) {
			return a.HTTP.DeleteWithHeaders(ctx, path, body, headers)
		}, path, nil, body, headers)
}
