package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewMockConfig(t *testing.T) {
	cfg := NewMockConfig(map[string]string{"config": "value"})

	assert.Equal(t, "value", cfg.Get("config"))

	assert.Equal(t, "value1", cfg.GetOrDefault("config1", "value1"))
}
