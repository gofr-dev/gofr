package auth

import (
	"context"
	"strings"

	"gofr.dev/pkg/gofr/service"
)

// #nosec G101
const apiKeyHeader = "X-Api-Key"

type apiKeyConfig struct {
	apiKey string
}

func (*apiKeyConfig) GetHeaderKey() string {
	return apiKeyHeader
}

func (a *apiKeyConfig) GetHeaderValue(_ context.Context) (string, error) {
	return a.apiKey, nil
}

// NewAPIKeyConfig validates the provided API key and returns a service.Options
// that injects the X-Api-Key header into outgoing HTTP requests.
func NewAPIKeyConfig(apiKey string) (service.Options, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, Err{Message: "api key is required"}
	}

	return NewAuthOption(&apiKeyConfig{apiKey: apiKey}), nil
}
