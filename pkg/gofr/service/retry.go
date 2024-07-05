package service

import (
	"context"
	"net/http"
)

type RetryConfig struct {
	MaxRetries int
}

func (r *RetryConfig) AddOption(h HTTP) HTTP {
	return &retryProvider{
		maxRetries: r.MaxRetries,
		HTTP:       h,
	}
}

type retryProvider struct {
	maxRetries int

	HTTP
}

func (rp *retryProvider) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response,
	error) {
	return rp.doWithRetry(func() (*http.Response, error) {
		return rp.HTTP.Get(ctx, path, queryParams)
	})
}

func (rp *retryProvider) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	headers map[string]string) (*http.Response, error) {
	return rp.doWithRetry(func() (*http.Response, error) {
		return rp.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
	})
}

func (rp *retryProvider) Post(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return rp.doWithRetry(func() (*http.Response, error) {
		return rp.HTTP.Post(ctx, path, queryParams, body)
	})
}

func (rp *retryProvider) PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte,
	headers map[string]string) (*http.Response, error) {
	return rp.doWithRetry(func() (*http.Response, error) {
		return rp.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
	})
}

func (rp *retryProvider) Put(ctx context.Context, api string, queryParams map[string]interface{}, body []byte) (
	*http.Response, error) {
	return rp.doWithRetry(func() (*http.Response, error) {
		return rp.HTTP.Put(ctx, api, queryParams, body)
	})
}

func (rp *retryProvider) PutWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte,
	headers map[string]string) (*http.Response, error) {
	return rp.doWithRetry(func() (*http.Response, error) {
		return rp.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
	})
}

func (rp *retryProvider) Patch(ctx context.Context, path string, queryParams map[string]interface{}, body []byte) (
	*http.Response, error) {
	return rp.doWithRetry(func() (*http.Response, error) {
		return rp.HTTP.Patch(ctx, path, queryParams, body)
	})
}

func (rp *retryProvider) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]interface{}, body []byte,
	headers map[string]string) (*http.Response, error) {
	return rp.doWithRetry(func() (*http.Response, error) {
		return rp.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
	})
}

func (rp *retryProvider) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return rp.doWithRetry(func() (*http.Response, error) {
		return rp.HTTP.Delete(ctx, path, body)
	})
}

func (rp *retryProvider) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (
	*http.Response, error) {
	return rp.doWithRetry(func() (*http.Response, error) {
		return rp.HTTP.DeleteWithHeaders(ctx, path, body, headers)
	})
}

func (rp *retryProvider) doWithRetry(reqFunc func() (*http.Response, error)) (*http.Response, error) {
	var (
		resp *http.Response
		err  error
	)

	for i := 0; i < rp.maxRetries; i++ {
		resp, err = reqFunc()
		if err == nil && resp.StatusCode != http.StatusInternalServerError {
			return resp, nil
		}
	}

	return resp, err
}
