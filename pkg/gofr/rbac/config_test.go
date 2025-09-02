package rbac

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadPermissions_Success(t *testing.T) {
	jsonContent := `{
        "route": {"admin":["read", "write"], "user":["read"]},
        "OverRides": {"admin":true, "user":false}
    }`
	tempFile, err := os.CreateTemp("", "test_permissions_*.json")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())

	_, err = tempFile.Write([]byte(jsonContent))
	assert.NoError(t, err)
	tempFile.Close()

	cfg, err := LoadPermissions(tempFile.Name())
	assert.NoError(t, err)
	assert.Equal(t, map[string][]string{"admin": {"read", "write"}, "user": {"read"}}, cfg.RouteWithPermissions)
	assert.Equal(t, map[string]bool{"admin": true, "user": false}, cfg.OverRides)
}

func TestLoadPermissions_FileNotFound(t *testing.T) {
	cfg, err := LoadPermissions("non_existent_file.json")
	assert.Nil(t, cfg)
	assert.Error(t, err)
}

func TestLoadPermissions_InvalidJSON(t *testing.T) {
	tempFile, err := os.CreateTemp("", "badjson_*.json")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())

	_, err = tempFile.Write([]byte(`{"route": [INVALID JSON}`))
	assert.NoError(t, err)
	tempFile.Close()

	cfg, err := LoadPermissions(tempFile.Name())
	assert.Nil(t, cfg)
	assert.Error(t, err)
}
