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

func TestGetConfigs_MetricsCardinalityLimit(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected int
	}{
		{name: "neither set uses the SDK default", env: map[string]string{}, expected: 2000},
		{name: "METRICS_CARDINALITY_LIMIT", env: map[string]string{metricsCardinalityLimitKey: "100"}, expected: 100},
		{name: "OTEL_GO_X_CARDINALITY_LIMIT", env: map[string]string{otelCardinalityLimitKey: "300"}, expected: 300},
		{name: "GoFr key wins over the OTel key",
			env: map[string]string{metricsCardinalityLimitKey: "100", otelCardinalityLimitKey: "300"}, expected: 100},
		{name: "invalid GoFr key falls through to the OTel key",
			env: map[string]string{metricsCardinalityLimitKey: "lots", otelCardinalityLimitKey: "300"}, expected: 300},
		{name: "zero means unlimited and is kept", env: map[string]string{metricsCardinalityLimitKey: "0"}, expected: 0},
		{name: "whitespace is trimmed", env: map[string]string{metricsCardinalityLimitKey: " 64 "}, expected: 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, GetConfigs(config.NewMockConfig(tt.env)).MetricsCardinalityLimit)
		})
	}
}
