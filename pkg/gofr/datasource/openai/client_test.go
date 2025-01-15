package openai

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewClient(t *testing.T) {
	tests := []struct {
		name          string
		config        *Config
		httpClient    *http.Client
		baseURL       string
		expected      string
		expectedError error
	}{
		{
			name:          "with default base URL",
			config:        &Config{APIKey: "test-key", Model: "gpt-4"},
			httpClient:    &http.Client{},
			expected:      "https://api.openai.com",
			expectedError: nil,
		},
		{
			name:          "with custom base URL",
			config:        &Config{APIKey: "test-key", Model: "gpt-4", BaseURL: "https://custom.openai.com"},
			httpClient:    &http.Client{},
			expected:      "https://custom.openai.com",
			expectedError: nil,
		},
		{
			name:          "missing api key",
			config:        &Config{Model: "gpt-4"},
			httpClient:    &http.Client{},
			expectedError: ErrorMissingAPIKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.config, WithClientHTTP(tt.httpClient))
			if tt.expectedError != nil {
				assert.Equal(t, tt.expectedError, err)
				assert.Nil(t, client)
			} else {
				assert.Equal(t, tt.expected, client.config.BaseURL)
				assert.NoError(t, err)
			}
		})
	}
}
