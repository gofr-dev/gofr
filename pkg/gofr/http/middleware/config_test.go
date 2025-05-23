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
