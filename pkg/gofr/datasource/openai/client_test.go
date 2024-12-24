package openai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewClient(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		baseURL  string
		expected string
	}{
		{
			name:     "with default base URL",
			config:   &Config{APIKey: "test-key", Model: "gpt-4"},
			expected: "https://api.openai.com",
		},
		{
			name:     "with custom base URL",
			config:   &Config{APIKey: "test-key", Model: "gpt-4", BaseURL: "https://custom.openai.com"},
			expected: "https://custom.openai.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewCLient(tt.config)
			assert.NotNil(t, client)
			assert.Equal(t, tt.expected, client.config.BaseURL)
		})
	}
}
