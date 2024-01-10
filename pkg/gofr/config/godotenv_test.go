package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEnvFile(t *testing.T) {
	resp := NewEnvFile("/configs")

	assert.IsType(t, &EnvFile{}, resp, "TEST Failed.\n")
}

func TestNewGoDotEnvProvider(t *testing.T) {
	f := new(EnvFile)

	t.Setenv("APP_NAME", "gofr-sample")

	app := f.Get("APP_NAME")

	assert.Equal(t, "gofr-sample", app, "TEST Failed.\n")
}

func TestEnvFile_GetOrDefault(t *testing.T) {
	var (
		key   = "random123"
		value = "value123"
		f     = new(EnvFile)
	)

	t.Setenv(key, value)

	tests := []struct {
		desc  string
		key   string
		value string
	}{
		{"success case", key, value},
		{"key doesn't exists", "someKeyThatDoesntExist", "default"},
	}

	for i, tc := range tests {
		resp := f.GetOrDefault(tc.key, "default")

		assert.Equal(t, tc.value, resp, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}
