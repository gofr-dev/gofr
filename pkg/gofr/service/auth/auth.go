package auth

import (
	"context"
	"fmt"
	"net/http"

	"gofr.dev/pkg/gofr/service"
)

// Provider provides authentication credentials for outgoing HTTP requests.
// Implementations return a static header key and a dynamic header value.
type Provider interface {
	GetHeaderKey() string
	GetHeaderValue(ctx context.Context) (string, error)
}

// TokenSource provides raw token values for bearer-style authentication.
// Implementations are responsible only for obtaining the token string;
// the bearer header format ("Bearer <token>") is handled by NewBearerAuthOption.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// NewAuthOption wraps any Provider into a service.Options for use with AddHTTPService.
func NewAuthOption(p Provider) service.Options {
	return &authOptionAdapter{provider: p}
}

// NewBearerAuthOption creates a service.Options that injects "Authorization: Bearer <token>"
// using the provided TokenSource.
func NewBearerAuthOption(src TokenSource) service.Options {
	return NewAuthOption(&bearerAuthProvider{source: src})
}

type bearerAuthProvider struct {
	source TokenSource
}

func (b *bearerAuthProvider) UseLogger(logger service.Logger) {
	if obs, ok := b.source.(service.Observable); ok {
		obs.UseLogger(logger)
	}
}

func (b *bearerAuthProvider) UseMetrics(metrics service.Metrics) {
	if obs, ok := b.source.(service.Observable); ok {
		obs.UseMetrics(metrics)
	}
}

func (*bearerAuthProvider) GetHeaderKey() string {
	return service.AuthHeader
}

func (b *bearerAuthProvider) GetHeaderValue(ctx context.Context) (string, error) {
	token, err := b.source.Token(ctx)
	if err != nil {
		return "", err
	}

	return "Bearer " + token, nil
}

type authOptionAdapter struct {
	provider Provider
}

func (a *authOptionAdapter) UseLogger(logger service.Logger) {
	if obs, ok := a.provider.(service.Observable); ok {
		obs.UseLogger(logger)
	}
}

func (a *authOptionAdapter) UseMetrics(metrics service.Metrics) {
	if obs, ok := a.provider.(service.Observable); ok {
		obs.UseMetrics(metrics)
	}
}

func (a *authOptionAdapter) AddOption(h service.HTTP) service.HTTP {
	return &authProvider{
		auth: a.addHeader,
		HTTP: h,
	}
}

// addHeader handles nil init, collision detection, and header injection.
func (a *authOptionAdapter) addHeader(ctx context.Context, headers map[string]string) (map[string]string, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	key := a.provider.GetHeaderKey()

	if existing, exists := headers[key]; exists {
		return headers, Err{Message: fmt.Sprintf("value %v already exists for header %v", existing, key)}
	}

	value, err := a.provider.GetHeaderValue(ctx)
	if err != nil {
		return headers, err
	}

	headers[key] = value

	return headers, nil
}

type authProvider struct {
	auth func(context.Context, map[string]string) (map[string]string, error)
	service.HTTP
}

func (a *authProvider) Get(ctx context.Context, path string, queryParams map[string]any) (*http.Response, error) {
	return a.GetWithHeaders(ctx, path, queryParams, nil)
}

func (a *authProvider) GetWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	headers map[string]string) (*http.Response, error) {
	headers, err := a.auth(ctx, headers)
	if err != nil {
		return nil, err
	}

	return a.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

func (a *authProvider) Post(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return a.PostWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *authProvider) PostWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	headers, err := a.auth(ctx, headers)
	if err != nil {
		return nil, err
	}

	return a.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *authProvider) Patch(ctx context.Context, path string, queryParams map[string]any, body []byte) (*http.Response, error) {
	return a.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *authProvider) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	headers, err := a.auth(ctx, headers)
	if err != nil {
		return nil, err
	}

	return a.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *authProvider) Put(ctx context.Context, path string, queryParams map[string]any,
	body []byte) (*http.Response, error) {
	return a.PutWithHeaders(ctx, path, queryParams, body, nil)
}

func (a *authProvider) PutWithHeaders(ctx context.Context, path string, queryParams map[string]any,
	body []byte, headers map[string]string) (*http.Response, error) {
	headers, err := a.auth(ctx, headers)
	if err != nil {
		return nil, err
	}

	return a.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

func (a *authProvider) Delete(ctx context.Context, path string, body []byte) (*http.Response, error) {
	return a.DeleteWithHeaders(ctx, path, body, nil)
}

func (a *authProvider) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (*http.Response, error) {
	headers, err := a.auth(ctx, headers)
	if err != nil {
		return nil, err
	}

	return a.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}
