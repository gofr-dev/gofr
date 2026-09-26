package gofr

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"

	"gofr.dev/pkg/gofr/container"
	grpcMiddleware "gofr.dev/pkg/gofr/grpc/middleware"
	"gofr.dev/pkg/gofr/http/middleware"
)

// authDisabledMsg is logged by the deprecated Enable* methods when their input is unusable and the
// app starts without that auth middleware. Args: the error, the mechanism, the WithError method.
const authDisabledMsg = "%v. %s is DISABLED: all routes are served without it. Use %s to stop startup instead"

var (
	errNoBasicAuthCredentials  = errors.New("no credentials provided for basic auth")
	errOddBasicAuthCredentials = errors.New("invalid number of arguments for basic auth: expected username/password pairs")
	errInvalidJWKSEndpoint     = errors.New("invalid JWKS endpoint URL")
)

// EnableBasicAuthWithError enables basic authentication for the application.
//
// It takes a variable number of credentials as alternating username and password strings.
// It returns an error, and installs no middleware, if no credentials or an odd number of
// arguments are provided.
func (a *App) EnableBasicAuthWithError(credentials ...string) error {
	if len(credentials) == 0 {
		return errNoBasicAuthCredentials
	}

	if len(credentials)%2 != 0 {
		return errOddBasicAuthCredentials
	}

	users := make(map[string]string)
	for i := 0; i < len(credentials); i += 2 {
		users[credentials[i]] = credentials[i+1]
	}

	provider := grpcMiddleware.BasicAuthProvider{Users: users}

	a.addAuthMiddleware(middleware.BasicAuthMiddleware(middleware.BasicAuthProvider{Users: users}),
		grpcMiddleware.BasicAuthUnaryInterceptor(provider), grpcMiddleware.BasicAuthStreamInterceptor(provider))

	return nil
}

// EnableBasicAuth enables basic authentication for the application.
//
// Deprecated: use [App.EnableBasicAuthWithError], which returns the error instead of starting with
// authentication disabled. EnableBasicAuth will be removed in the next major release.
func (a *App) EnableBasicAuth(credentials ...string) {
	if err := a.EnableBasicAuthWithError(credentials...); err != nil {
		a.container.Errorf(authDisabledMsg, err, "Basic authentication", "EnableBasicAuthWithError")
	}
}

// EnableBasicAuthWithFunc enables basic authentication for the HTTP server with a custom validation function.
//
// Deprecated: This method is deprecated and will be removed in future releases, users must use
// [App.EnableBasicAuthWithValidator] as it has access to application datasources.
func (a *App) EnableBasicAuthWithFunc(validateFunc func(username, password string) bool) {
	provider := grpcMiddleware.BasicAuthProvider{ValidateFunc: validateFunc, Container: a.container}

	a.addAuthMiddleware(middleware.BasicAuthMiddleware(middleware.BasicAuthProvider{ValidateFunc: validateFunc, Container: a.container}),
		grpcMiddleware.BasicAuthUnaryInterceptor(provider), grpcMiddleware.BasicAuthStreamInterceptor(provider))
}

// EnableBasicAuthWithValidator enables basic authentication for the HTTP server with a custom validator.
//
// The provided `validateFunc` is invoked for each authentication attempt. It receives a container instance,
// username, and password. The function should return `true` if the credentials are valid, `false` otherwise.
func (a *App) EnableBasicAuthWithValidator(validateFunc func(c *container.Container, username, password string) bool) {
	provider := grpcMiddleware.BasicAuthProvider{ValidateFuncWithDatasources: validateFunc, Container: a.container}

	a.addAuthMiddleware(middleware.BasicAuthMiddleware(middleware.BasicAuthProvider{
		ValidateFuncWithDatasources: validateFunc, Container: a.container}),
		grpcMiddleware.BasicAuthUnaryInterceptor(provider), grpcMiddleware.BasicAuthStreamInterceptor(provider))
}

// EnableAPIKeyAuth enables API key authentication for the application.
//
// It requires at least one API key to be provided. The provided API keys will be used to authenticate requests.
func (a *App) EnableAPIKeyAuth(apiKeys ...string) {
	provider := grpcMiddleware.APIKeyAuthProvider{APIKeys: apiKeys}

	a.addAuthMiddleware(middleware.APIKeyAuthMiddleware(middleware.APIKeyAuthProvider{}, apiKeys...),
		grpcMiddleware.APIKeyAuthUnaryInterceptor(provider), grpcMiddleware.APIKeyAuthStreamInterceptor(provider))
}

// EnableAPIKeyAuthWithFunc enables API key authentication for the application with a custom validation function.
//
// Deprecated: This method is deprecated and will be removed in future releases, users must use
// [App.EnableAPIKeyAuthWithValidator] as it has access to application datasources.
func (a *App) EnableAPIKeyAuthWithFunc(validateFunc func(apiKey string) bool) {
	provider := grpcMiddleware.APIKeyAuthProvider{ValidateFunc: validateFunc, Container: a.container}

	a.addAuthMiddleware(middleware.APIKeyAuthMiddleware(middleware.APIKeyAuthProvider{
		ValidateFunc: validateFunc,
		Container:    a.container,
	}), grpcMiddleware.APIKeyAuthUnaryInterceptor(provider), grpcMiddleware.APIKeyAuthStreamInterceptor(provider))
}

// EnableAPIKeyAuthWithValidator enables API key authentication for the application with a custom validation function.
//
// The provided `validateFunc` is used to determine the validity of an API key. It receives the request container
// and the API key as arguments and should return `true` if the key is valid, `false` otherwise.
func (a *App) EnableAPIKeyAuthWithValidator(validateFunc func(c *container.Container, apiKey string) bool) {
	provider := grpcMiddleware.APIKeyAuthProvider{ValidateFuncWithDatasources: validateFunc, Container: a.container}

	a.addAuthMiddleware(middleware.APIKeyAuthMiddleware(middleware.APIKeyAuthProvider{
		ValidateFuncWithDatasources: validateFunc,
		Container:                   a.container,
	}), grpcMiddleware.APIKeyAuthUnaryInterceptor(provider), grpcMiddleware.APIKeyAuthStreamInterceptor(provider))
}

// EnableOAuthWithError configures OAuth middleware for the application.
//
// It registers a new HTTP service for fetching JWKS and sets up OAuth middleware
// with the given JWKS endpoint and refresh interval.
//
// The JWKS endpoint is used to retrieve JSON Web Key Sets for verifying tokens.
// The refresh interval specifies how often to refresh the token cache.
// We can define optional JWT claim validation settings, including issuer, audience, and expiration checks.
// Accepts jwt.ParserOption for additional parsing options:
// https://pkg.go.dev/github.com/golang-jwt/jwt/v4#ParserOption
//
// It returns an error, and installs no middleware, if jwksEndpoint is not an http or https URL
// with a host.
func (a *App) EnableOAuthWithError(jwksEndpoint string,
	refreshInterval int,
	options ...jwt.ParserOption,
) error {
	parsedURL, err := url.Parse(jwksEndpoint)
	if err != nil {
		return fmt.Errorf("%w: %w", errInvalidJWKSEndpoint, err)
	}

	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("%w: missing scheme or host in %q", errInvalidJWKSEndpoint, jwksEndpoint)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("%w: unsupported scheme %q", errInvalidJWKSEndpoint, parsedURL.Scheme)
	}

	baseURL := parsedURL.Scheme + "://" + parsedURL.Host
	jwksPath := strings.TrimPrefix(parsedURL.Path, "/")

	if parsedURL.RawQuery != "" {
		jwksPath += "?" + parsedURL.RawQuery
	}

	a.AddHTTPService("gofr_oauth", baseURL)

	oauthOption := middleware.OauthConfigs{
		Provider:        a.container.GetHTTPService("gofr_oauth"),
		RefreshInterval: time.Second * time.Duration(refreshInterval),
		Path:            jwksPath,
	}

	publicKeyProvider := middleware.NewOAuth(oauthOption)

	a.addAuthMiddleware(middleware.OAuth(publicKeyProvider, options...),
		grpcMiddleware.OAuthUnaryInterceptor(publicKeyProvider, options...),
		grpcMiddleware.OAuthStreamInterceptor(publicKeyProvider, options...))

	return nil
}

// EnableOAuth configures OAuth middleware for the application.
//
// Deprecated: use [App.EnableOAuthWithError], which returns the error instead of starting with
// authentication disabled. EnableOAuth will be removed in the next major release.
func (a *App) EnableOAuth(jwksEndpoint string, refreshInterval int, options ...jwt.ParserOption) {
	if err := a.EnableOAuthWithError(jwksEndpoint, refreshInterval, options...); err != nil {
		a.container.Errorf(authDisabledMsg, err, "OAuth authentication", "EnableOAuthWithError")
	}
}

func (a *App) addAuthMiddleware(httpMW func(http.Handler) http.Handler,
	grpcUnary grpc.UnaryServerInterceptor, grpcStream grpc.StreamServerInterceptor) {
	if a.httpServer != nil {
		a.httpServer.router.Use(httpMW)
	}

	if a.grpcServer != nil {
		a.grpcServer.addUnaryInterceptors(grpcUnary)
		a.grpcServer.addStreamInterceptors(grpcStream)
	}
}
