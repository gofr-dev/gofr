package rbac

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadPermissions_Success(t *testing.T) {
	jsonContent := `{
        "route": {"admin":["read", "write"], "user":["read"]},
        "overrides": {"admin":true, "user":false}
    }`
	tempFile, err := os.CreateTemp(t.TempDir(), "test_permissions_*.json")
	require.NoError(t, err)

	_, err = tempFile.WriteString(jsonContent)
	require.NoError(t, err)
	tempFile.Close()

	cfg, err := LoadPermissions(tempFile.Name())
	require.NoError(t, err)
	assert.Equal(t, map[string][]string{"admin": {"read", "write"}, "user": {"read"}}, cfg.RouteWithPermissions)
	assert.Equal(t, map[string]bool{"admin": true, "user": false}, cfg.OverRides)
}

func TestLoadPermissions_FileNotFound(t *testing.T) {
	cfg, err := LoadPermissions("non_existent_file.json")
	assert.Nil(t, cfg)
	assert.Error(t, err)
}

func TestLoadPermissions_InvalidJSON(t *testing.T) {
	tempFile, err := os.CreateTemp(t.TempDir(), "badjson_*.json")
	require.NoError(t, err)

	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString(`{"route": [INVALID JSON}`)
	require.NoError(t, err)
	tempFile.Close()

	cfg, err := LoadPermissions(tempFile.Name())
	assert.Nil(t, cfg)
	assert.Error(t, err)
}

func TestLoadPermissions_YAML(t *testing.T) {
	yamlContent := `route:
  /api/users:
    - admin
    - editor
  /api/posts:
    - admin
    - author
overrides:
  /health: true
defaultRole: viewer
`
	tempFile, err := os.CreateTemp(t.TempDir(), "test_permissions_*.yaml")
	require.NoError(t, err)

	_, err = tempFile.WriteString(yamlContent)
	require.NoError(t, err)
	tempFile.Close()

	cfg, err := LoadPermissions(tempFile.Name())
	require.NoError(t, err)
	assert.Equal(t, []string{"admin", "editor"}, cfg.RouteWithPermissions["/api/users"])
	assert.Equal(t, []string{"admin", "author"}, cfg.RouteWithPermissions["/api/posts"])
	assert.True(t, cfg.OverRides["/health"])
	assert.Equal(t, "viewer", cfg.DefaultRole)
}

func TestLoadPermissions_YML(t *testing.T) {
	yamlContent := `route:
  /api/users:
    - admin
`
	tempFile, err := os.CreateTemp(t.TempDir(), "test_permissions_*.yml")
	require.NoError(t, err)

	_, err = tempFile.WriteString(yamlContent)
	require.NoError(t, err)
	tempFile.Close()

	cfg, err := LoadPermissions(tempFile.Name())
	require.NoError(t, err)
	assert.Equal(t, []string{"admin"}, cfg.RouteWithPermissions["/api/users"])
}

func TestLoadPermissions_UnsupportedFormat(t *testing.T) {
	tempFile, err := os.CreateTemp(t.TempDir(), "test_permissions_*.xml")
	require.NoError(t, err)

	_, err = tempFile.WriteString(`<config></config>`)
	require.NoError(t, err)
	tempFile.Close()

	cfg, err := LoadPermissions(tempFile.Name())
	assert.Nil(t, cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported config file format")
}

func TestLoadPermissions_InvalidYAML(t *testing.T) {
	tempFile, err := os.CreateTemp(t.TempDir(), "badyaml_*.yaml")
	require.NoError(t, err)

	_, err = tempFile.WriteString(`route: [invalid: yaml`)
	require.NoError(t, err)
	tempFile.Close()

	cfg, err := LoadPermissions(tempFile.Name())
	assert.Nil(t, cfg)
	assert.Error(t, err)
}

func TestLoadPermissions_EnvOverrides(t *testing.T) {
	// Set environment variables
	t.Setenv("RBAC_DEFAULT_ROLE", "test-role")
	t.Setenv("RBAC_ROUTE_/api/test", "admin,editor")
	t.Setenv("RBAC_OVERRIDE_/public", "true")

	defer func() {
		os.Unsetenv("RBAC_DEFAULT_ROLE")
		os.Unsetenv("RBAC_ROUTE_/api/test")
		os.Unsetenv("RBAC_OVERRIDE_/public")
	}()

	jsonContent := `{
        "route": {"admin":["read"]},
        "overrides": {}
    }`
	tempFile, err := os.CreateTemp(t.TempDir(), "test_permissions_*.json")
	require.NoError(t, err)

	_, err = tempFile.WriteString(jsonContent)
	require.NoError(t, err)
	tempFile.Close()

	cfg, err := LoadPermissions(tempFile.Name())
	require.NoError(t, err)

	// Check environment variable overrides
	assert.Equal(t, "test-role", cfg.DefaultRole)
	assert.Equal(t, []string{"admin", "editor"}, cfg.RouteWithPermissions["/api/test"])
	assert.True(t, cfg.OverRides["/public"])
}

func TestLoadPermissions_InitializesEmptyMaps(t *testing.T) {
	jsonContent := `{}`
	tempFile, err := os.CreateTemp(t.TempDir(), "test_permissions_*.json")
	require.NoError(t, err)

	_, err = tempFile.WriteString(jsonContent)
	require.NoError(t, err)
	tempFile.Close()

	cfg, err := LoadPermissions(tempFile.Name())
	require.NoError(t, err)
	assert.NotNil(t, cfg.RouteWithPermissions)
	assert.NotNil(t, cfg.OverRides)
}

func TestNewConfigLoaderWithLogger_NilLogger(t *testing.T) {
	jsonContent := `{
        "route": {"admin":["read"]}
    }`
	tempFile, err := os.CreateTemp(t.TempDir(), "test_permissions_*.json")
	require.NoError(t, err)

	_, err = tempFile.WriteString(jsonContent)
	require.NoError(t, err)
	tempFile.Close()

	// Test config loader
		loader, err := NewConfigLoaderWithLogger(tempFile.Name(), nil)
	require.NoError(t, err)
	assert.NotNil(t, loader)

	config := loader.GetConfig()
	assert.NotNil(t, config)
	assert.Equal(t, []string{"read"}, config.GetRouteWithPermissions()["admin"])
}

func TestConfigLoader_GetConfig_ThreadSafe(t *testing.T) {
	jsonContent := `{
        "route": {"admin":["read"]}
    }`
	tempFile, err := os.CreateTemp(t.TempDir(), "test_permissions_*.json")
	require.NoError(t, err)

	_, err = tempFile.WriteString(jsonContent)
	require.NoError(t, err)
	tempFile.Close()

		loader, err := NewConfigLoaderWithLogger(tempFile.Name(), nil)
	require.NoError(t, err)

	// Concurrent reads
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			config := loader.GetConfig()
			assert.NotNil(t, config)

			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestLoadPermissions_WithPermissionConfig(t *testing.T) {
	jsonContent := `{
        "route": {"admin":["read"]},
        "permissions": {
            "permissions": {
                "users:read": ["admin", "editor"],
                "users:write": ["admin"]
            },
            "routePermissions": {
                "GET /api/users": "users:read",
                "POST /api/users": "users:write"
            }
        },
        "enablePermissions": true
    }`
	tempFile, err := os.CreateTemp(t.TempDir(), "test_permissions_*.json")
	require.NoError(t, err)

	_, err = tempFile.WriteString(jsonContent)
	require.NoError(t, err)
	tempFile.Close()

	cfg, err := LoadPermissions(tempFile.Name())
	require.NoError(t, err)
	assert.NotNil(t, cfg.PermissionConfig)
	assert.Equal(t, []string{"admin", "editor"}, cfg.PermissionConfig.Permissions["users:read"])
	assert.Equal(t, "users:read", cfg.PermissionConfig.RoutePermissionMap["GET /api/users"])
	assert.True(t, cfg.EnablePermissions)
}

func TestLoadPermissions_WithRoleHierarchy(t *testing.T) {
	jsonContent := `{
        "route": {"admin":["read"]},
        "roleHierarchy": {
            "admin": ["editor", "author", "viewer"],
            "editor": ["author", "viewer"]
        }
    }`
	tempFile, err := os.CreateTemp(t.TempDir(), "test_permissions_*.json")
	require.NoError(t, err)

	_, err = tempFile.WriteString(jsonContent)
	require.NoError(t, err)
	tempFile.Close()

	cfg, err := LoadPermissions(tempFile.Name())
	require.NoError(t, err)
	assert.Equal(t, []string{"editor", "author", "viewer"}, cfg.RoleHierarchy["admin"])
	assert.Equal(t, []string{"author", "viewer"}, cfg.RoleHierarchy["editor"])
}

func TestLoadPermissions_FileExtensionDetection(t *testing.T) {
	tests := []struct {
		name     string
		ext      string
		content  string
		wantErr  bool
		checkErr func(*testing.T, error)
	}{
		{
			name:    "JSON file",
			ext:     ".json",
			content: `{"route": {"admin":["read"]}}`,
			wantErr: false,
		},
		{
			name:    "YAML file",
			ext:     ".yaml",
			content: `route:\n  admin: [read]`,
			wantErr: false,
		},
		{
			name:    "YML file",
			ext:     ".yml",
			content: `route:\n  admin: [read]`,
			wantErr: false,
		},
		{
			name:    "No extension defaults to JSON",
			ext:     "",
			content: `{"route": {"admin":["read"]}}`,
			wantErr: false,
		},
		{
			name:    "Unsupported format",
			ext:     ".xml",
			content: `<config></config>`,
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				t.Helper()
				assert.Contains(t, err.Error(), "unsupported config file format")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempFile, err := os.CreateTemp(t.TempDir(), "test_permissions_*"+tt.ext)
			require.NoError(t, err)

			defer os.Remove(tempFile.Name())

			_, err = tempFile.WriteString(tt.content)
			require.NoError(t, err)
			tempFile.Close()

			cfg, err := LoadPermissions(tempFile.Name())
			assertLoadPermissionsResult(t, cfg, err, tt.wantErr, tt.checkErr)
		})
	}
}

func TestLoadPermissions_NoExtensionFile(t *testing.T) {
	jsonContent := `{"route": {"admin":["read"]}}`
	tempFile, err := os.CreateTemp(t.TempDir(), "test_permissions")
	require.NoError(t, err)

	_, err = tempFile.WriteString(jsonContent)
	require.NoError(t, err)
	tempFile.Close()

	cfg, err := LoadPermissions(tempFile.Name())
	require.NoError(t, err)
	assert.Equal(t, []string{"read"}, cfg.RouteWithPermissions["admin"])
}

func TestConfigLoader_GetConfig_ConcurrentAccess(t *testing.T) {
	jsonContent := `{
        "route": {"admin":["read"]}
    }`
	tempFile, err := os.CreateTemp(t.TempDir(), "test_permissions_*.json")
	require.NoError(t, err)

	_, err = tempFile.WriteString(jsonContent)
	require.NoError(t, err)
	tempFile.Close()

	loader, err := NewConfigLoaderWithLogger(tempFile.Name(), nil)
	require.NoError(t, err)

	// Concurrent reads and writes
	done := make(chan bool, 20)

	for i := 0; i < 10; i++ {
		go func() {
			config := loader.GetConfig()
			assert.NotNil(t, config)

			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestConfig_GetPermissionConfig_Nil(t *testing.T) {
	config := &Config{}
	assert.Nil(t, config.GetPermissionConfig())
}

func TestConfig_GetLogger_Nil(t *testing.T) {
	config := &Config{}
	assert.Nil(t, config.GetLogger())
}

func TestLoadPermissions_ErrorMessages(t *testing.T) {
	tests := []struct {
		name       string
		setup      func() string
		wantErrMsg string
		cleanup    func(string)
	}{
		{
			name: "File not found",
			setup: func() string {
				return "nonexistent_file.json"
			},
			wantErrMsg: "failed to read RBAC config file",
			cleanup:    func(string) {},
		},
		{
			name: "Invalid JSON",
			setup: func() string {
				tempFile, _ := os.CreateTemp(t.TempDir(), "bad_*.json")
				_, _ = tempFile.WriteString(`{invalid json}`)
				tempFile.Close()
				return tempFile.Name()
			},
			wantErrMsg: "failed to parse JSON config file",
			cleanup: func(path string) {
				os.Remove(path)
			},
		},
		{
			name: "Invalid YAML",
			setup: func() string {
				tempFile, _ := os.CreateTemp(t.TempDir(), "bad_*.yaml")
				_, _ = tempFile.WriteString(`invalid: yaml: [`)
				tempFile.Close()
				return tempFile.Name()
			},
			wantErrMsg: "failed to parse YAML config file",
			cleanup: func(path string) {
				os.Remove(path)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			defer tt.cleanup(path)

			cfg, err := LoadPermissions(path)
			assert.Nil(t, cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrMsg)
		})
	}
}

// assertLoadPermissionsResult asserts LoadPermissions results without nested if-else.
func assertLoadPermissionsResult(t *testing.T, cfg *Config, err error, wantErr bool, checkErr func(*testing.T, error)) {
	t.Helper()

	if wantErr {
		require.Error(t, err)

		if checkErr != nil {
			checkErr(t, err)
		}

		return
	}

	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestConfig_GetterMethods(t *testing.T) {
	extractor := func(_ *http.Request, _ ...any) (string, error) {
		return "admin", nil
	}
	permissionConfig := &PermissionConfig{
		Permissions: map[string][]string{
			"users:read": {"admin"},
		},
	}
	mockLogger := &mockLogger{}

	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/api/users": {"admin"},
		},
		RoleExtractorFunc: extractor,
		PermissionConfig:  permissionConfig,
		OverRides:         map[string]bool{"/health": true},
		Logger:            mockLogger,
		EnablePermissions: true,
	}

	assert.Equal(t, map[string][]string{"/api/users": {"admin"}}, config.GetRouteWithPermissions())
	assert.NotNil(t, config.GetRoleExtractorFunc())
	assert.Equal(t, permissionConfig, config.GetPermissionConfig())
	assert.Equal(t, map[string]bool{"/health": true}, config.GetOverRides())
	assert.Equal(t, mockLogger, config.GetLogger())
}

func TestConfig_SetterMethods(t *testing.T) {
	config := &Config{}
	extractor := func(_ *http.Request, _ ...any) (string, error) {
		return "admin", nil
	}
	mockLogger := &mockLogger{}

	config.SetRoleExtractorFunc(extractor)
	assert.NotNil(t, config.RoleExtractorFunc)

	config.SetLogger(mockLogger)
	assert.Equal(t, mockLogger, config.Logger)

	config.SetLogger("not-a-logger")
	assert.Equal(t, mockLogger, config.Logger) // Should remain unchanged

	config.SetEnablePermissions(true)
	assert.True(t, config.EnablePermissions)

	config.SetEnablePermissions(false)
	assert.False(t, config.EnablePermissions)
}

func TestConfig_InitializeMaps(t *testing.T) {
	t.Run("AllMapsNil", func(t *testing.T) {
		config := &Config{}
		config.InitializeMaps()

		assert.NotNil(t, config.RouteWithPermissions)
		assert.NotNil(t, config.OverRides)
		assert.NotNil(t, config.RoleHierarchy)
	})

	t.Run("SomeMapsNil", func(t *testing.T) {
		config := &Config{
			RouteWithPermissions: map[string][]string{"/test": {"admin"}},
		}
		config.InitializeMaps()

		assert.NotNil(t, config.RouteWithPermissions)
		assert.Equal(t, []string{"admin"}, config.RouteWithPermissions["/test"])
		assert.NotNil(t, config.OverRides)
		assert.NotNil(t, config.RoleHierarchy)
	})

	t.Run("AllMapsInitialized", func(t *testing.T) {
		config := &Config{
			RouteWithPermissions: map[string][]string{"/test": {"admin"}},
			OverRides:            map[string]bool{"/health": true},
			RoleHierarchy:        map[string][]string{"admin": {"editor"}},
		}
		config.InitializeMaps()

		assert.Equal(t, []string{"admin"}, config.RouteWithPermissions["/test"])
		assert.True(t, config.OverRides["/health"])
		assert.Equal(t, []string{"editor"}, config.RoleHierarchy["admin"])
	})
}

func TestNewConfigLoaderWithLogger(t *testing.T) {
	jsonContent := `{
        "route": {"admin":["read"]}
    }`
	tempFile, err := os.CreateTemp(t.TempDir(), "test_permissions_*.json")
	require.NoError(t, err)

	_, err = tempFile.WriteString(jsonContent)
	require.NoError(t, err)
	tempFile.Close()

	mockLogger := &mockLogger{}

	loader, err := NewConfigLoaderWithLogger(tempFile.Name(), mockLogger)
	require.NoError(t, err)
	assert.NotNil(t, loader)

	config := loader.GetConfig()
	assert.NotNil(t, config)
	assert.Equal(t, mockLogger, config.GetLogger())
}

func TestApplyEnvOverrides_EdgeCases(t *testing.T) {
	t.Run("RouteOverrideWithUnderscores", func(t *testing.T) {
		t.Setenv("RBAC_ROUTE_/api_test_users", "admin,editor")

		defer os.Unsetenv("RBAC_ROUTE_/api_test_users")

		config := &Config{}
		applyEnvOverrides(config)

		assert.Equal(t, []string{"admin", "editor"}, config.RouteWithPermissions["/api/test/users"])
	})

	t.Run("RouteOverrideWithSpaces", func(t *testing.T) {
		t.Setenv("RBAC_ROUTE_/api/users", "admin, editor, viewer")

		defer os.Unsetenv("RBAC_ROUTE_/api/users")

		config := &Config{}
		applyEnvOverrides(config)

		assert.Equal(t, []string{"admin", "editor", "viewer"}, config.RouteWithPermissions["/api/users"])
	})

	t.Run("OverrideFlagFalse", func(t *testing.T) {
		t.Setenv("RBAC_OVERRIDE_/public", "false")

		defer os.Unsetenv("RBAC_OVERRIDE_/public")

		config := &Config{}
		applyEnvOverrides(config)

		assert.Nil(t, config.OverRides)
	})

	t.Run("OverrideFlagZero", func(t *testing.T) {
		t.Setenv("RBAC_OVERRIDE_/public", "0")

		defer os.Unsetenv("RBAC_OVERRIDE_/public")

		config := &Config{}
		applyEnvOverrides(config)

		assert.Nil(t, config.OverRides)
	})

	t.Run("OverrideFlagOne", func(t *testing.T) {
		t.Setenv("RBAC_OVERRIDE_/public", "1")

		defer os.Unsetenv("RBAC_OVERRIDE_/public")

		config := &Config{}
		applyEnvOverrides(config)

		assert.True(t, config.OverRides["/public"])
	})

	t.Run("OverrideFlagCaseInsensitive", func(t *testing.T) {
		t.Setenv("RBAC_OVERRIDE_/public", "TRUE")

		defer os.Unsetenv("RBAC_OVERRIDE_/public")

		config := &Config{}
		applyEnvOverrides(config)

		assert.True(t, config.OverRides["/public"])
	})

	t.Run("RouteOverrideSingleRole", func(t *testing.T) {
		t.Setenv("RBAC_ROUTE_/api/test", "admin")

		defer os.Unsetenv("RBAC_ROUTE_/api/test")

		config := &Config{}
		applyEnvOverrides(config)

		assert.Equal(t, []string{"admin"}, config.RouteWithPermissions["/api/test"])
	})

	t.Run("RouteOverrideEmptyValue", func(t *testing.T) {
		t.Setenv("RBAC_ROUTE_/api/test", "")

		defer os.Unsetenv("RBAC_ROUTE_/api/test")

		config := &Config{}
		applyEnvOverrides(config)

		assert.Equal(t, []string{""}, config.RouteWithPermissions["/api/test"])
	})
}

func TestApplyRouteOverride(t *testing.T) {
	t.Run("WithNilRouteWithPermissions", func(t *testing.T) {
		config := &Config{}
		applyRouteOverride(config, "RBAC_ROUTE_/api/test", "admin,editor")

		assert.NotNil(t, config.RouteWithPermissions)
		assert.Equal(t, []string{"admin", "editor"}, config.RouteWithPermissions["/api/test"])
	})

	t.Run("WithExistingRouteWithPermissions", func(t *testing.T) {
		config := &Config{
			RouteWithPermissions: map[string][]string{
				"/other": {"viewer"},
			},
		}
		applyRouteOverride(config, "RBAC_ROUTE_/api/test", "admin,editor")

		assert.Equal(t, []string{"admin", "editor"}, config.RouteWithPermissions["/api/test"])
		assert.Equal(t, []string{"viewer"}, config.RouteWithPermissions["/other"])
	})

	t.Run("WithMultipleUnderscores", func(t *testing.T) {
		config := &Config{}
		applyRouteOverride(config, "RBAC_ROUTE_/api_test_users", "admin")

		assert.Equal(t, []string{"admin"}, config.RouteWithPermissions["/api/test/users"])
	})
}

func TestApplyOverrideFlag(t *testing.T) {
	t.Run("WithTrueValue", func(t *testing.T) {
		config := &Config{}
		applyOverrideFlag(config, "RBAC_OVERRIDE_/public", "true")

		assert.NotNil(t, config.OverRides)
		assert.True(t, config.OverRides["/public"])
	})

	t.Run("WithOneValue", func(t *testing.T) {
		config := &Config{}
		applyOverrideFlag(config, "RBAC_OVERRIDE_/public", "1")

		assert.NotNil(t, config.OverRides)
		assert.True(t, config.OverRides["/public"])
	})

	t.Run("WithFalseValue", func(t *testing.T) {
		config := &Config{}
		applyOverrideFlag(config, "RBAC_OVERRIDE_/public", "false")

		assert.Nil(t, config.OverRides)
	})

	t.Run("WithNilOverRides", func(t *testing.T) {
		config := &Config{}
		applyOverrideFlag(config, "RBAC_OVERRIDE_/public", "true")

		assert.NotNil(t, config.OverRides)
		assert.True(t, config.OverRides["/public"])
	})

	t.Run("WithExistingOverRides", func(t *testing.T) {
		config := &Config{
			OverRides: map[string]bool{
				"/other": true,
			},
		}
		applyOverrideFlag(config, "RBAC_OVERRIDE_/public", "true")

		assert.True(t, config.OverRides["/public"])
		assert.True(t, config.OverRides["/other"])
	})
}
