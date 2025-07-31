// Package service provides an HTTP client with features for logging, metrics, and resilience.It supports various
// functionalities like health checks, circuit-breaker and various authentication.
package service

import (
	"context"
	"fmt"
	"strings"
)

// #nosec G101
const xAPIKeyHeader = "X-Api-Key"

type APIKeyConfig struct {
	APIKey string
}

func (a *APIKeyConfig) AddOption(h HTTP) HTTP {
	return &authProvider{auth: a.addAuthorizationHeader, HTTP: h}
}

func NewAPIKeyConfig(apiKey string) (Options, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, AuthErr{Message: "non empty api key is required"}
	}

	return &APIKeyConfig{APIKey: apiKey}, nil
}

func (a *APIKeyConfig) addAuthorizationHeader(_ context.Context, headers map[string]string) (map[string]string, error) {
	if headers == nil {
		headers = make(map[string]string)
	}

	if value, exists := headers[xAPIKeyHeader]; exists {
		return headers, AuthErr{Message: fmt.Sprintf("value %v already exists for header %v", value, xAPIKeyHeader)}
	}

	headers[xAPIKeyHeader] = a.APIKey

	return headers, nil
}
