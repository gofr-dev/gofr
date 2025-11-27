package gofr

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/middleware"
)

// EnableBasicAuth enables basic authentication for the application.
//
// It takes a variable number of credentials as alternating username and password strings.
// An error is logged if an odd number of arguments is provided.
func (a *App) EnableBasicAuth(credentials ...string) {
	if len(credentials) == 0 {
		a.container.Error("No credentials provided for EnableBasicAuth. Proceeding without Authentication")
		return
	}

	if len(credentials)%2 != 0 {
		a.container.Error("Invalid number of arguments for EnableBasicAuth. Proceeding without Authentication")

		return
	}

	users := make(map[string]string)
	for i := 0; i < len(credentials); i += 2 {
		users[credentials[i]] = credentials[i+1]
	}

	a.httpServer.router.Use(middleware.BasicAuthMiddleware(middleware.BasicAuthProvider{Users: users}))
}

// EnableBasicAuthWithFunc enables basic authentication for the HTTP server with a custom validation function.
//
// Deprecated: This method is deprecated and will be removed in future releases, users must use
// [App.EnableBasicAuthWithValidator] as it has access to application datasources.
func (a *App) EnableBasicAuthWithFunc(validateFunc func(username, password string) bool) {
	a.httpServer.router.Use(middleware.BasicAuthMiddleware(middleware.BasicAuthProvider{ValidateFunc: validateFunc, Container: a.container}))
}

// EnableBasicAuthWithValidator enables basic authentication for the HTTP server with a custom validator.
//
// The provided `validateFunc` is invoked for each authentication attempt. It receives a container instance,
// username, and password. The function should return `true` if the credentials are valid, `false` otherwise.
func (a *App) EnableBasicAuthWithValidator(validateFunc func(c *container.Container, username, password string) bool) {
	a.httpServer.router.Use(middleware.BasicAuthMiddleware(middleware.BasicAuthProvider{
		ValidateFuncWithDatasources: validateFunc, Container: a.container}))
}

// EnableAPIKeyAuth enables API key authentication for the application.
//
// It requires at least one API key to be provided. The provided API keys will be used to authenticate requests.
func (a *App) EnableAPIKeyAuth(apiKeys ...string) {
	a.httpServer.router.Use(middleware.APIKeyAuthMiddleware(middleware.APIKeyAuthProvider{}, apiKeys...))
}

// EnableAPIKeyAuthWithFunc enables API key authentication for the application with a custom validation function.
//
// Deprecated: This method is deprecated and will be removed in future releases, users must use
// [App.EnableAPIKeyAuthWithValidator] as it has access to application datasources.
func (a *App) EnableAPIKeyAuthWithFunc(validateFunc func(apiKey string) bool) {
	a.httpServer.router.Use(middleware.APIKeyAuthMiddleware(middleware.APIKeyAuthProvider{
		ValidateFunc: validateFunc,
		Container:    a.container,
	}))
}

// EnableAPIKeyAuthWithValidator enables API key authentication for the application with a custom validation function.
//
// The provided `validateFunc` is used to determine the validity of an API key. It receives the request container
// and the API key as arguments and should return `true` if the key is valid, `false` otherwise.
func (a *App) EnableAPIKeyAuthWithValidator(validateFunc func(c *container.Container, apiKey string) bool) {
	a.httpServer.router.Use(middleware.APIKeyAuthMiddleware(middleware.APIKeyAuthProvider{
		ValidateFuncWithDatasources: validateFunc,
		Container:                   a.container,
	}))
}

// EnableOAuth configures OAuth middleware for the application.
//
// It registers a new HTTP service for fetching JWKS and sets up OAuth middleware
// with the given JWKS endpoint and refresh interval.
//
// The JWKS endpoint is used to retrieve JSON Web Key Sets for verifying tokens.
// The refresh interval specifies how often to refresh the token cache.
// We can define optional JWT claim validation settings, including issuer, audience, and expiration checks.
// Accepts jwt.ParserOption for additional parsing options:
// https://pkg.go.dev/github.com/golang-jwt/jwt/v4#ParserOption
func (a *App) EnableOAuth(jwksEndpoint string,
	refreshInterval int,
	options ...jwt.ParserOption,
) {
	a.AddHTTPService("gofr_oauth", jwksEndpoint)

	oauthOption := middleware.OauthConfigs{
		Provider:        a.container.GetHTTPService("gofr_oauth"),
		RefreshInterval: time.Second * time.Duration(refreshInterval),
	}

	a.httpServer.router.Use(middleware.OAuth(middleware.NewOAuth(oauthOption), options...))
}
