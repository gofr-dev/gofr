// Package middleware provides a collection of middleware functions that handles various aspects of request handling,
// such as authentication, logging, tracing, and metrics collection.
package middleware

import (
	"errors"
	"net/http"

	"gofr.dev/pkg/gofr/container"
)

var (
	errAPIKeyEmpty = errors.New("api keys list is empty")
)

// APIKeyAuthProvider represents a basic authentication provider.
type APIKeyAuthProvider struct {
	ValidateFunc                func(apiKey string) bool
	ValidateFuncWithDatasources func(c *container.Container, apiKey string) bool
	Container                   *container.Container
	APIKeys                     []string
}

// NewAPIKeyAuthProvider instantiates an instance of type AuthProvider interface.
func NewAPIKeyAuthProvider(apiKeys []string) (AuthProvider, error) {
	if len(apiKeys) == 0 {
		return nil, errAPIKeyEmpty
	}

	return &APIKeyAuthProvider{APIKeys: apiKeys}, nil
}

// NewAPIKeyAuthProviderWithValidateFunc instantiates an instance of type AuthProvider interface.
func NewAPIKeyAuthProviderWithValidateFunc(c *container.Container,
	validateFunc func(*container.Container, string) bool) (AuthProvider, error) {
	switch {
	case c == nil:
		return nil, errContainerNil
	case validateFunc == nil:
		return nil, errValidateFuncEmpty
	default:
		return &APIKeyAuthProvider{Container: c, ValidateFuncWithDatasources: validateFunc}, nil
	}
}

func (a *APIKeyAuthProvider) ExtractAuthHeader(r *http.Request) (any, ErrorHTTP) {
	header, err := getAuthHeaderFromRequest(r, headerXAPIKey, "")
	if err != nil {
		return nil, err
	}

	if !a.validateAPIKey(header) {
		return nil, NewInvalidAuthorizationHeaderError(headerXAPIKey)
	}

	return header, nil
}

func (*APIKeyAuthProvider) GetAuthMethod() AuthMethod {
	return APIKey
}

// validateAPIKey verifies the given apiKey as per the configured APIKeyAuthProvider.
func (a *APIKeyAuthProvider) validateAPIKey(apiKey string) bool {
	switch {
	case a.ValidateFuncWithDatasources != nil:
		return a.ValidateFuncWithDatasources(a.Container, apiKey)
	case a.ValidateFunc != nil:
		return a.ValidateFunc(apiKey)
	default:
		for _, key := range a.APIKeys {
			if apiKey == key {
				return true
			}
		}

		return false
	}
}

// APIKeyAuthMiddleware creates a middleware function that enforces API key authentication based on the provided API
// keys or a validation function.
func APIKeyAuthMiddleware(a APIKeyAuthProvider, apiKeys ...string) func(handler http.Handler) http.Handler {
	a.APIKeys = apiKeys
	return AuthMiddleware(&a)
}
