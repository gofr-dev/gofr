package middleware

import (
	"github.com/stretchr/testify/assert"
	"gofr.dev/pkg/gofr/config"
	"testing"
)

func TestGetConfigs(t *testing.T) {
	mockConfig := config.NewMockConfig(map[string]string{
		"Access-Control-Allow-Origin":       "*",
		"Access-Control-Allow-Headers":      "Authorization, Content-Type",
		"Access-Control-Allow-Methods":      "GET, POST",
		"Access-Control-Allow-Credentials":  "true",
		"Access-Control-Allow-CustomHeader": "abc",
	})

	middlewareConfigs := GetConfigs(mockConfig)

	expectedConfigs := map[string]string{
		"Access-Control-Allow-Origin":      "*",
		"Access-Control-Allow-Headers":     "Authorization, Content-Type",
		"Access-Control-Allow-Methods":     "GET, POST",
		"Access-Control-Allow-Credentials": "true",
	}

	assert.Equal(t, middlewareConfigs, expectedConfigs, "TestGetConfigs Failed!")
	assert.NotContains(t, middlewareConfigs, "Access-Control-Allow-CustomHeader", "TestGetConfigs Failed!")

}
