package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewEnvFile(t *testing.T) {
	resp := NewEnvFile("/configs")

	assert.IsType(t, &EnvFile{}, resp, "TEST Failed.\n")
}

func TestNewGoDotEnvProvider(t *testing.T) {
	var (
		configs = "TEST=test\nNAME=gofr\nKEY_123=value123"
		path    = createTestConfigFile(t, configs)
		f       = NewEnvFile(path)
	)

	defer os.RemoveAll(path)

	testCases := []struct {
		key      string
		expected string
	}{
		{"TEST", "test"},
		{"NAME", "gofr"},
		{"KEY_123", "value123"},
	}

	for i, tc := range testCases {
		resp := f.Get(tc.key)

		assert.Equal(t, tc.expected, resp, "TEST[%d], Failed.\n", i)
	}
}

func TestEnvFile_GetOrDefault(t *testing.T) {
	var (
		configs = "VALID=true"
		path    = createTestConfigFile(t, configs)
		f       = NewEnvFile(path)
	)

	defer os.RemoveAll(path)

	testCases := []struct {
		desc     string
		key      string
		expected string
	}{
		{"success case", "VALID", "true"},
		{"key doesn't exists", "someKeyThatDoesntExist", "default"},
	}

	for i, tc := range testCases {
		resp := f.GetOrDefault(tc.key, "default")

		assert.Equal(t, tc.expected, resp, "TEST[%d], Failed.\n%s", i, tc.desc)
	}
}

func createTestConfigFile(t *testing.T, configs string) string {
	path, err := os.MkdirTemp("", "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_ = os.Chdir(path)

	f, err := os.Create(".env")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, err = f.WriteString(configs)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	return path
}
