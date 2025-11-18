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

// Validate implements the Validator interface for APIKeyConfig.
// Returns an error if the API key is empty.
func (a *APIKeyConfig) Validate() error {
	apiKey := strings.TrimSpace(a.APIKey)
	if apiKey == "" {
		return AuthErr{Message: "non empty api key is required"}
	}

	return nil
}

// FeatureName implements the Validator interface.
func (*APIKeyConfig) FeatureName() string {
	return "APIKey Authentication"
}

func NewAPIKeyConfig(apiKey string) (Options, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, AuthErr{Message: "non empty api key is required"}
	}

	config := &APIKeyConfig{APIKey: apiKey}

	// Validate during creation as well for immediate feedback
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return config, nil
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
