package rbac

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/container"
)

func TestNewProvider(t *testing.T) {
	provider := NewProvider()
	assert.NotNil(t, provider)
	
	// Verify it implements the interface
	var _ container.RBACProvider = provider
}

func TestProvider_LoadPermissions(t *testing.T) {
	provider := NewProvider()

	t.Run("Success", func(t *testing.T) {
		jsonContent := `{"route": {"/admin": ["admin"]}}`

		tempFile, err := os.CreateTemp(t.TempDir(), "test_*.json")
		require.NoError(t, err)

		_, err = tempFile.WriteString(jsonContent)
		require.NoError(t, err)
		require.NoError(t, tempFile.Close())

		config, err := provider.LoadPermissions(tempFile.Name())
		require.NoError(t, err)
		assert.NotNil(t, config)

		// Verify it's a *Config
		rbacConfig, ok := config.(*Config)
		require.True(t, ok)
		assert.NotNil(t, rbacConfig)
	})

	t.Run("FileNotFound", func(t *testing.T) {
		config, err := provider.LoadPermissions("nonexistent.json")
		require.Error(t, err)
		assert.Nil(t, config)
	})
}

func TestProvider_GetMiddleware(t *testing.T) {
	provider := NewProvider()

	t.Run("WithValidConfig", func(t *testing.T) {
		config := &Config{
			RouteWithPermissions: map[string][]string{
				"/test": {"admin"},
			},
		}

		mwFunc := provider.GetMiddleware(config)
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
		mwFunc := provider.GetMiddleware(nil)
		assert.NotNil(t, mwFunc)

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := mwFunc(handler)
		assert.NotNil(t, wrapped)
	})

	t.Run("WithNonConfigType", func(t *testing.T) {
		// Pass a non-Config type
		mwFunc := provider.GetMiddleware("not-a-config")
		assert.NotNil(t, mwFunc)

		handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		wrapped := mwFunc(handler)
		assert.NotNil(t, wrapped)
	})
}

func TestProvider_RequireRole(t *testing.T) {
	provider := NewProvider()

	t.Run("Success", func(t *testing.T) {
		handlerFunc := func(_ any) (any, error) {
			return "success", nil
		}

		wrapped := provider.RequireRole("admin", handlerFunc)
		assert.NotNil(t, wrapped)
	})

	t.Run("WithContextValueGetter", func(t *testing.T) {
		handlerCalled := false
		handlerFunc := func(_ any) (any, error) {
			handlerCalled = true
			return "success", nil
		}

		wrapped := provider.RequireRole("admin", handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(key any) any {
				if key == userRole {
					return "admin"
				}
				return nil
			},
		}

		result, err := wrapped(ctx)
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

		wrapped := provider.RequireRole("admin", handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(key any) any {
				if key == userRole {
					return "user"
				}
				return nil
			},
		}

		result, err := wrapped(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrAccessDenied)
		assert.False(t, handlerCalled)
		assert.Nil(t, result)
	})
}

func TestProvider_RequireAnyRole(t *testing.T) {
	provider := NewProvider()

	t.Run("Success", func(t *testing.T) {
		handlerFunc := func(_ any) (any, error) {
			return "success", nil
		}

		wrapped := provider.RequireAnyRole([]string{"admin", "editor"}, handlerFunc)
		assert.NotNil(t, wrapped)
	})

	t.Run("WithMatchingRole", func(t *testing.T) {
		handlerCalled := false
		handlerFunc := func(_ any) (any, error) {
			handlerCalled = true
			return "success", nil
		}

		wrapped := provider.RequireAnyRole([]string{"admin", "editor"}, handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(key any) any {
				if key == userRole {
					return "admin"
				}
				return nil
			},
		}

		result, err := wrapped(ctx)
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

		wrapped := provider.RequireAnyRole([]string{"admin", "editor"}, handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(key any) any {
				if key == userRole {
					return "viewer"
				}
				return nil
			},
		}

		result, err := wrapped(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrAccessDenied)
		assert.False(t, handlerCalled)
		assert.Nil(t, result)
	})
}

func TestProvider_RequirePermission(t *testing.T) {
	provider := NewProvider()

	t.Run("Success", func(t *testing.T) {
		permissionConfig := &PermissionConfig{
			Permissions: map[string][]string{
				"users:read": {"admin"},
			},
		}

		handlerFunc := func(_ any) (any, error) {
			return "success", nil
		}

		wrapped := provider.RequirePermission("users:read", permissionConfig, handlerFunc)
		assert.NotNil(t, wrapped)
	})

	t.Run("WithInvalidPermissionConfigType", func(t *testing.T) {
		handlerFunc := func(_ any) (any, error) {
			return "success", nil
		}

		wrapped := provider.RequirePermission("users:read", nil, handlerFunc)
		assert.NotNil(t, wrapped)

		ctx := &mockContextValueGetter{
			value: func(_ any) any {
				return nil
			},
		}

		result, err := wrapped(ctx)
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

		wrapped := provider.RequirePermission("users:read", permissionConfig, handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(key any) any {
				if key == userRole {
					return "admin"
				}
				return nil
			},
		}

		result, err := wrapped(ctx)
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

		wrapped := provider.RequirePermission("users:read", permissionConfig, handlerFunc)

		ctx := &mockContextValueGetter{
			value: func(key any) any {
				if key == userRole {
					return "viewer"
				}
				return nil
			},
		}

		result, err := wrapped(ctx)
		require.Error(t, err)
		require.ErrorIs(t, err, ErrPermissionDenied)
		assert.False(t, handlerCalled)
		assert.Nil(t, result)
	})
}

func TestProvider_ErrAccessDenied(t *testing.T) {
	provider := NewProvider()
	err := provider.ErrAccessDenied()
	assert.Equal(t, ErrAccessDenied, err)
}

func TestProvider_ErrPermissionDenied(t *testing.T) {
	provider := NewProvider()
	err := provider.ErrPermissionDenied()
	assert.Equal(t, ErrPermissionDenied, err)
}

