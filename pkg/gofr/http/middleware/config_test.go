package middleware

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/config"
)

func TestGetConfigs(t *testing.T) {
	// Without LOG_DISABLE_PROBES configs provided
	{
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

		assert.Equal(t, expectedConfigs, middlewareConfigs, "TestGetConfigs Failed!")
		assert.NotContains(t, middlewareConfigs, "Access-Control-Allow-CustomHeader", "TestGetConfigs Failed!")
	}

	// With LOG_DISABLE_PROBES config provided
	{
		mockConfig := config.NewMockConfig(map[string]string{
			"ACCESS_CONTROL_ALLOW_ORIGIN":       "*",
			"ACCESS_CONTROL_ALLOW_HEADERS":      "Authorization, Content-Type",
			"ACCESS_CONTROL_ALLOW_CREDENTIALS":  "true",
			"ACCESS_CONTROL_ALLOW_CUSTOMHEADER": "abc",
			"LOG_DISABLE_PROBES":                "true",
		})

		middlewareConfigs := GetConfigs(mockConfig)

		expectedConfigs := map[string]string{
			"Access-Control-Allow-Origin":      "*",
			"Access-Control-Allow-Headers":     "Authorization, Content-Type",
			"Access-Control-Allow-Credentials": "true",
			"LOG_DISABLE_PROBES":               "true",
		}

		assert.Equal(t, expectedConfigs, middlewareConfigs, "TestGetConfigs Failed!")
		assert.NotContains(t, middlewareConfigs, "Access-Control-Allow-CustomHeader", "TestGetConfigs Failed!")
	}
}
