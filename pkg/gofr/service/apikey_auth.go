// Package service provides an HTTP client with features for logging, metrics, and resilience.It supports various
// functionalities like health checks, circuit-breaker and various authentication.
package service

import (
	"context"
	"fmt"
	"strings"
)

const xAPIKeyHeader = "X-Api-Key"

type APIKeyConfig struct {
	APIKey string
}

func (a *APIKeyConfig) AddOption(h HTTP) HTTP {
	return &apiKeyAuthProvider{
		apiKey:       a.APIKey,
		authProvider: authProvider{h},
	}
}

type apiKeyAuthProvider struct {
	apiKey string

	authProvider
}

func NewAPIKeyConfig(apiKey string) (Options, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, AuthErr{Message: "non empty api key is required"}
	}

	return &APIKeyConfig{APIKey: apiKey}, nil
}

func (a *apiKeyAuthProvider) addAuthorizationHeader(ctx context.Context, headers map[string]string) (map[string]string, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	if value, exists := headers[xAPIKeyHeader]; exists {
		return headers, AuthErr{Message: fmt.Sprintf("value %v already exists for header %v", value, xAPIKeyHeader)}
	}

	headers[xAPIKeyHeader] = a.apiKey

	return headers, nil
}
