package service

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// OAuthConfig describes a 2-legged OAuth2 flow, with both the
// client application information and the server's endpoint URLs.
type OAuthConfig struct {
	// ClientID is the application's ID.
	ClientID string

	// ClientSecret is the application's secret.
	ClientSecret string

	// TokenURL is the resource server's token endpoint
	// URL. This is a constant specific to each server.
	TokenURL string

	// Scope specifies optional requested permissions.
	Scopes []string

	// EndpointParams specifies additional parameters for requests to the token endpoint.
	EndpointParams url.Values
}

func (h *OAuthConfig) addOption(svc HTTP) HTTP {
	return &oAuth{
		Config: clientcredentials.Config{
			ClientID:       h.ClientID,
			ClientSecret:   h.ClientSecret,
			TokenURL:       h.TokenURL,
			Scopes:         h.Scopes,
			EndpointParams: h.EndpointParams,
			AuthStyle:      oauth2.AuthStyleInHeader,
		},
		HTTP: svc,
	}
}

type oAuth struct {
	clientcredentials.Config

	HTTP
}

func (o *oAuth) addAuthorizationHeader(ctx context.Context, headers map[string]string) (map[string]string, error) {
	var err error

	if headers == nil {
		headers = make(map[string]string)
	}

	token, err := o.TokenSource(ctx).Token()
	if err != nil {
		return nil, err
	}

	headers["Authorization"] = fmt.Sprintf("%v %v", token.TokenType, token.AccessToken)

	return headers, nil
}

func (o *oAuth) GetWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	headers map[string]string) (*http.Response, error) {
	headers, err := o.addAuthorizationHeader(ctx, headers)
	if err != nil {
		return nil, err
	}

	return o.HTTP.GetWithHeaders(ctx, path, queryParams, headers)
}

// PostWithHeaders is a wrapper for doRequest with the POST method and headers.
func (o *oAuth) PostWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	headers, err := o.addAuthorizationHeader(ctx, headers)
	if err != nil {
		return nil, err
	}

	return o.HTTP.PostWithHeaders(ctx, path, queryParams, body, headers)
}

// PatchWithHeaders is a wrapper for doRequest with the PATCH method and headers.
func (o *oAuth) PatchWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	headers, err := o.addAuthorizationHeader(ctx, headers)
	if err != nil {
		return nil, err
	}

	return o.HTTP.PatchWithHeaders(ctx, path, queryParams, body, headers)
}

// PutWithHeaders is a wrapper for doRequest with the PUT method and headers.
func (o *oAuth) PutWithHeaders(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte, headers map[string]string) (*http.Response, error) {
	headers, err := o.addAuthorizationHeader(ctx, headers)
	if err != nil {
		return nil, err
	}

	return o.HTTP.PutWithHeaders(ctx, path, queryParams, body, headers)
}

// DeleteWithHeaders is a wrapper for doRequest with the DELETE method and headers.
func (o *oAuth) DeleteWithHeaders(ctx context.Context, path string, body []byte, headers map[string]string) (
	*http.Response, error) {
	headers, err := o.addAuthorizationHeader(ctx, headers)
	if err != nil {
		return nil, err
	}

	return o.HTTP.DeleteWithHeaders(ctx, path, body, headers)
}

func (o *oAuth) Get(ctx context.Context, path string, queryParams map[string]interface{}) (*http.Response, error) {
	return o.GetWithHeaders(ctx, path, queryParams, nil)
}

// Post is a wrapper for doRequest with the POST method and headers.
func (o *oAuth) Post(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return o.PostWithHeaders(ctx, path, queryParams, body, nil)
}

// Patch is a wrapper for doRequest with the PATCH method and headers.
func (o *oAuth) Patch(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return o.PatchWithHeaders(ctx, path, queryParams, body, nil)
}

// Put is a wrapper for doRequest with the PUT method and headers.
func (o *oAuth) Put(ctx context.Context, path string, queryParams map[string]interface{},
	body []byte) (*http.Response, error) {
	return o.PutWithHeaders(ctx, path, queryParams, body, nil)
}

// Delete is a wrapper for doRequest with the DELETE method and headers.
func (o *oAuth) Delete(ctx context.Context, path string, body []byte) (
	*http.Response, error) {
	return o.DeleteWithHeaders(ctx, path, body, nil)
}
