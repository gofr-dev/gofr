package openai

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewClient(t *testing.T) {
	tests := []struct {
		name       string
		config     *Config
		httpClient *http.Client
		baseURL    string
		expected   string
	}{
		{
			name:       "with default base URL",
			config:     &Config{APIKey: "test-key", Model: "gpt-4"},
			httpClient: &http.Client{},
			expected:   "https://api.openai.com",
		},
		{
			name:       "with custom base URL",
			config:     &Config{APIKey: "test-key", Model: "gpt-4", BaseURL: "https://custom.openai.com"},
			httpClient: &http.Client{},
			expected:   "https://custom.openai.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewClient(tt.config, tt.httpClient)
			assert.NotNil(t, client)
			assert.Equal(t, tt.expected, client.config.BaseURL)
		})
	}
}
