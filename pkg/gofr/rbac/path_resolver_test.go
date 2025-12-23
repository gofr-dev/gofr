package rbac

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveRBACConfigPath(t *testing.T) {
	t.Run("returns custom path when provided", func(t *testing.T) {
		customPath := "custom/path/rbac.json"
		result := ResolveRBACConfigPath(customPath)
		assert.Equal(t, customPath, result)
	})

	t.Run("returns default json path when file exists", func(t *testing.T) {
		// Create a temporary rbac.json file
		dir := "configs"
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)

		filePath := filepath.Join(dir, "rbac.json")
		err = os.WriteFile(filePath, []byte(`{"roles":[]}`), 0600)
		require.NoError(t, err)

		defer func() {
			os.Remove(filePath)
			os.Remove(dir)
		}()

		result := ResolveRBACConfigPath("")
		assert.Equal(t, filePath, result)
	})

	t.Run("returns default yaml path when json doesn't exist", func(t *testing.T) {
		// Create a temporary rbac.yaml file
		dir := "configs"
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)

		filePath := filepath.Join(dir, "rbac.yaml")
		err = os.WriteFile(filePath, []byte("roles: []"), 0600)
		require.NoError(t, err)

		defer func() {
			os.Remove(filePath)
			os.Remove(dir)
		}()

		result := ResolveRBACConfigPath("")
		assert.Equal(t, filePath, result)
	})

	t.Run("returns default yml path when json and yaml don't exist", func(t *testing.T) {
		// Create a temporary rbac.yml file
		dir := "configs"
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)

		filePath := filepath.Join(dir, "rbac.yml")
		err = os.WriteFile(filePath, []byte("roles: []"), 0600)
		require.NoError(t, err)

		defer func() {
			os.Remove(filePath)
			os.Remove(dir)
		}()

		result := ResolveRBACConfigPath("")
		assert.Equal(t, filePath, result)
	})

	t.Run("returns empty string when no default files exist", func(t *testing.T) {
		// Ensure configs directory doesn't exist or is empty
		dir := "configs"
		os.RemoveAll(dir)

		result := ResolveRBACConfigPath("")
		assert.Empty(t, result)
	})
}
