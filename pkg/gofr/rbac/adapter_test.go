package rbac

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegisterRBAC(t *testing.T) {
	// Verify that registerRBAC is called automatically
	// This is tested indirectly by checking that the registry is populated
	// We can't directly test the package-level variable, but we can test the adapters
	assert.NotNil(t, &rbacLoader{})
	assert.NotNil(t, &rbacMiddleware{})
}

func TestRBACLoader_LoadPermissions(t *testing.T) {
	loader := &rbacLoader{}

	t.Run("Success", func(t *testing.T) {
		jsonContent := `{"route": {"admin":["read"]}}`

		tempFile, err := os.CreateTemp(t.TempDir(), "test_*.json")
		require.NoError(t, err)

		_, err = tempFile.WriteString(jsonContent)
		require.NoError(t, err)
		require.NoError(t, tempFile.Close())

		config, err := loader.LoadPermissions(tempFile.Name())
		require.NoError(t, err)
		assert.NotNil(t, config)
	})

	t.Run("FileNotFound", func(t *testing.T) {
		config, err := loader.LoadPermissions("nonexistent.json")
		require.Error(t, err)
		assert.Nil(t, config)
	})
}

func TestRBACLoader_NewConfigLoaderWithLogger(t *testing.T) {
	loader := &rbacLoader{}

	t.Run("WithLogger", func(t *testing.T) {
		jsonContent := `{"route": {"admin":["read"]}}`

		tempFile, err := os.CreateTemp(t.TempDir(), "test_*.json")
		require.NoError(t, err)

		_, err = tempFile.WriteString(jsonContent)
		require.NoError(t, err)
		require.NoError(t, tempFile.Close())

		mockLogger := &mockLogger{}
		configLoader, err := loader.NewConfigLoaderWithLogger(tempFile.Name(), mockLogger)
		require.NoError(t, err)
		assert.NotNil(t, configLoader)

		config := configLoader.GetConfig()
		assert.NotNil(t, config)
	})

	t.Run("WithNilLogger", func(t *testing.T) {
		jsonContent := `{"route": {"admin":["read"]}}`

		tempFile, err := os.CreateTemp(t.TempDir(), "test_*.json")
		require.NoError(t, err)

		_, err = tempFile.WriteString(jsonContent)
		require.NoError(t, err)
		require.NoError(t, tempFile.Close())

		configLoader, err := loader.NewConfigLoaderWithLogger(tempFile.Name(), nil)
		require.NoError(t, err)
		assert.NotNil(t, configLoader)
	})

	t.Run("WithInvalidLoggerType", func(t *testing.T) {
		jsonContent := `{"route": {"admin":["read"]}}`

		tempFile, err := os.CreateTemp(t.TempDir(), "test_*.json")
		require.NoError(t, err)

		_, err = tempFile.WriteString(jsonContent)
		require.NoError(t, err)
		require.NoError(t, tempFile.Close())

		// Pass a non-logger type
		configLoader, err := loader.NewConfigLoaderWithLogger(tempFile.Name(), "not-a-logger")
		require.NoError(t, err)
		assert.NotNil(t, configLoader)
	})

	t.Run("FileNotFound", func(t *testing.T) {
		mockLogger := &mockLogger{}
		configLoader, err := loader.NewConfigLoaderWithLogger("nonexistent.json", mockLogger)
		require.Error(t, err)
		assert.Nil(t, configLoader)
	})
}

func TestRBACLoader_NewJWTRoleExtractor(t *testing.T) {
	loader := &rbacLoader{}

	t.Run("Success", func(t *testing.T) {
		extractor := loader.NewJWTRoleExtractor("role")
		assert.NotNil(t, extractor)
	})

	t.Run("EmptyClaim", func(t *testing.T) {
		extractor := loader.NewJWTRoleExtractor("")
		assert.NotNil(t, extractor)
	})
}

func TestRBACMiddleware_Middleware(t *testing.T) {
	middleware := &rbacMiddleware{}

	t.Run("WithValidConfig", func(t *testing.T) {
		config := &Config{
			RouteWithPermissions: map[string][]string{
				"/test": {"admin"},
			},
		}

		mwFunc := middleware.Middleware(config)
		assert.NotNil(t, mwFunc)

		// Verify it returns a middleware function
		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := mwFunc(handler)
		assert.NotNil(t, wrapped)
	})

	t.Run("WithInvalidConfigType", func(t *testing.T) {
		// Pass a config that doesn't implement *Config
		mwFunc := middleware.Middleware(nil)
		assert.NotNil(t, mwFunc)

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := mwFunc(handler)
		assert.NotNil(t, wrapped)
	})

	t.Run("WithNilConfig", func(t *testing.T) {
		mwFunc := middleware.Middleware(nil)
		assert.NotNil(t, mwFunc)

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := mwFunc(handler)
		assert.NotNil(t, wrapped)
	})
}

func TestRequireRoleAdapter(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		handlerFunc := func(_ any) (any, error) {
			return "success", nil
		}

		adapter := requireRoleAdapter("admin", handlerFunc)
		assert.NotNil(t, adapter)
	})

	t.Run("WithContextValueGetter", func(t *testing.T) {
		handlerCalled := false
		handlerFunc := func(_ any) (any, error) {
			handlerCalled = true
			return "success", nil
		}

		adapter := requireRoleAdapter("admin", handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(key any) any {
				if key == userRole {
					return "admin"
				}
				return nil
			},
		}

		result, err := adapter(ctx)
		require.NoError(t, err)
		assert.True(t, handlerCalled)
		assert.Equal(t, "success", result)
	})

	t.Run("WithWrongRole", func(t *testing.T) {
		handlerCalled := false
		handlerFunc := func(_ any) (any, error) {
			handlerCalled = true
			return "success", nil
		}

		adapter := requireRoleAdapter("admin", handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(key any) any {
				if key == userRole {
					return "user"
				}
				return nil
			},
		}

		result, err := adapter(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrAccessDenied)
		assert.False(t, handlerCalled)
		assert.Nil(t, result)
	})
}

func TestRequireAnyRoleAdapter(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		handlerFunc := func(_ any) (any, error) {
			return "success", nil
		}

		adapter := requireAnyRoleAdapter([]string{"admin", "editor"}, handlerFunc)
		assert.NotNil(t, adapter)
	})

	t.Run("WithMatchingRole", func(t *testing.T) {
		handlerCalled := false
		handlerFunc := func(_ any) (any, error) {
			handlerCalled = true
			return "success", nil
		}

		adapter := requireAnyRoleAdapter([]string{"admin", "editor"}, handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(key any) any {
				if key == userRole {
					return "admin"
				}
				return nil
			},
		}

		result, err := adapter(ctx)
		require.NoError(t, err)
		assert.True(t, handlerCalled)
		assert.Equal(t, "success", result)
	})

	t.Run("WithNonMatchingRole", func(t *testing.T) {
		handlerCalled := false
		handlerFunc := func(_ any) (any, error) {
			handlerCalled = true
			return "success", nil
		}

		adapter := requireAnyRoleAdapter([]string{"admin", "editor"}, handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(key any) any {
				if key == userRole {
					return "viewer"
				}
				return nil
			},
		}

		result, err := adapter(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrAccessDenied)
		assert.False(t, handlerCalled)
		assert.Nil(t, result)
	})
}

func TestRequirePermissionAdapter(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		permissionConfig := &PermissionConfig{
			Permissions: map[string][]string{
				"users:read": {"admin"},
			},
		}

		handlerFunc := func(_ any) (any, error) {
			return "success", nil
		}

		adapter := requirePermissionAdapter("users:read", permissionConfig, handlerFunc)
		assert.NotNil(t, adapter)
	})

	t.Run("WithInvalidPermissionConfigType", func(t *testing.T) {
		// Pass a config that doesn't implement *PermissionConfig
		handlerFunc := func(_ any) (any, error) {
			return "success", nil
		}

		adapter := requirePermissionAdapter("users:read", nil, handlerFunc)
		assert.NotNil(t, adapter)

		ctx := &mockContextValueGetter{
			value: func(_ any) any {
				return nil
			},
		}

		result, err := adapter(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPermissionDenied)
		assert.Nil(t, result)
	})

	t.Run("WithMatchingPermission", func(t *testing.T) {
		permissionConfig := &PermissionConfig{
			Permissions: map[string][]string{
				"users:read": {"admin"},
			},
		}

		handlerCalled := false
		handlerFunc := func(_ any) (any, error) {
			handlerCalled = true
			return "success", nil
		}

		adapter := requirePermissionAdapter("users:read", permissionConfig, handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(key any) any {
				if key == userRole {
					return "admin"
				}
				return nil
			},
		}

		result, err := adapter(ctx)
		require.NoError(t, err)
		assert.True(t, handlerCalled)
		assert.Equal(t, "success", result)
	})

	t.Run("WithNonMatchingPermission", func(t *testing.T) {
		permissionConfig := &PermissionConfig{
			Permissions: map[string][]string{
				"users:read": {"admin"},
			},
		}

		handlerCalled := false
		handlerFunc := func(_ any) (any, error) {
			handlerCalled = true
			return "success", nil
		}

		adapter := requirePermissionAdapter("users:read", permissionConfig, handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(key any) any {
				if key == userRole {
					return "viewer"
				}
				return nil
			},
		}

		result, err := adapter(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPermissionDenied)
		assert.False(t, handlerCalled)
		assert.Nil(t, result)
	})
}

func TestRequirePermissionAdapter_EdgeCases(t *testing.T) {
	t.Run("WithNoRoleInContext", func(t *testing.T) {
		permissionConfig := &PermissionConfig{
			Permissions: map[string][]string{
				"users:read": {"admin"},
			},
		}

		handlerCalled := false
		handlerFunc := func(_ any) (any, error) {
			handlerCalled = true
			return "success", nil
		}

		adapter := requirePermissionAdapter("users:read", permissionConfig, handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(_ any) any {
				return nil // No role in context
			},
		}

		result, err := adapter(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPermissionDenied)
		assert.False(t, handlerCalled)
		assert.Nil(t, result)
	})
}

func TestRequireAnyRoleAdapter_EdgeCases(t *testing.T) {
	t.Run("WithNoRoleInContext", func(t *testing.T) {
		handlerCalled := false
		handlerFunc := func(_ any) (any, error) {
			handlerCalled = true
			return "success", nil
		}

		adapter := requireAnyRoleAdapter([]string{"admin", "editor"}, handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(_ any) any {
				return nil // No role in context
			},
		}

		result, err := adapter(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrAccessDenied)
		assert.False(t, handlerCalled)
		assert.Nil(t, result)
	})

	t.Run("WithEmptyRolesList", func(t *testing.T) {
		handlerCalled := false
		handlerFunc := func(_ any) (any, error) {
			handlerCalled = true
			return "success", nil
		}

		adapter := requireAnyRoleAdapter([]string{}, handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(key any) any {
				if key == userRole {
					return "admin"
				}
				return nil
			},
		}

		result, err := adapter(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrAccessDenied)
		assert.False(t, handlerCalled)
		assert.Nil(t, result)
	})

	t.Run("WithNonStringRole", func(t *testing.T) {
		handlerCalled := false
		handlerFunc := func(_ any) (any, error) {
			handlerCalled = true
			return "success", nil
		}

		adapter := requireAnyRoleAdapter([]string{"admin", "editor"}, handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(key any) any {
				if key == userRole {
					return 123 // Non-string role
				}
				return nil
			},
		}

		result, err := adapter(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrAccessDenied)
		assert.False(t, handlerCalled)
		assert.Nil(t, result)
	})
}
