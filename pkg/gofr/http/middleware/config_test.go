package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
)

func TestGetConfigs(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"ACCESS_CONTROL_ALLOW_ORIGIN":       "*",
		"ACCESS_CONTROL_ALLOW_HEADERS":      "Authorization, Content-Type",
		"ACCESS_CONTROL_ALLOW_CREDENTIALS":  "true",
		"ACCESS_CONTROL_ALLOW_CUSTOMHEADER": "abc",
	})

	middlewareConfigs := GetConfigs(mockConfig)

	expectedConfigs := map[string]string{
		"Access-Control-Allow-Origin":      "*",
		"Access-Control-Allow-Headers":     "Authorization, Content-Type",
		"Access-Control-Allow-Credentials": "true",
	}

	assert.Equal(t, expectedConfigs, middlewareConfigs.CorsHeaders, "TestGetConfigs Failed!")
	assert.NotContains(t, middlewareConfigs.CorsHeaders, "Access-Control-Allow-CustomHeader", "TestGetConfigs Failed!")
}

func TestLogDisableProbesConfig(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"LOG_DISABLE_PROBES": "true",
	})

	middlewareConfigs := GetConfigs(mockConfig)

	assert.True(t, middlewareConfigs.LogProbes.Disabled, "TestLogDisableProbesConfig Failed!")
}

func TestMaxBodySizeConfig(t *testing.T) {
	tests := []struct {
		name         string
		configValue  string
		expectedSize int64
		description  string
	}{
		{
			name:         "Valid body size config",
			configValue:  "10485760", // 10 MB in bytes
			expectedSize: 10485760,
			description:  "Should parse valid body size from config",
		},
		{
			name:         "Empty body size config",
			configValue:  "",
			expectedSize: 0,
			description:  "Should default to 0 when config is empty",
		},
		{
			name:         "Invalid body size config",
			configValue:  "invalid",
			expectedSize: 0,
			description:  "Should default to 0 when config is invalid",
		},
		{
			name:         "Zero body size config",
			configValue:  "0",
			expectedSize: 0,
			description:  "Should be 0 when config is zero",
		},
		{
			name:         "Negative body size config",
			configValue:  "-100",
			expectedSize: 0,
			description:  "Should default to 0 when config is negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConfig := config.NewMockConfig(map[string]string{
				"HTTP_MAX_BODY_SIZE": tt.configValue,
			})

			middlewareConfigs := GetConfigs(mockConfig)

			assert.Equal(t, tt.expectedSize, middlewareConfigs.MaxBodySize, tt.description)
		})
	}
}
