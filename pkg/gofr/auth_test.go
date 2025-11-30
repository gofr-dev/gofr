package gofr

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/container"
	gofrHTTP "gofr.dev/pkg/gofr/http"
)

func TestEnableRBAC_WithoutProvider(t *testing.T) {
	app := &App{
		container: container.NewContainer(config.NewMockConfig(nil)),
		httpServer: &httpServer{
			router: gofrHTTP.NewRouter(),
		},
	}

	// Test new API - EnableRBAC requires provider
	app.EnableRBAC(nil, "test.json")
	// Should log error about nil provider
}

func TestEnableRBAC_NoConfigProvided(_ *testing.T) {
	app := &App{
		container: container.NewContainer(config.NewMockConfig(nil)),
		httpServer: &httpServer{
			router: gofrHTTP.NewRouter(),
		},
	}

	// Create a mock provider
	mockProvider := &mockRBACProvider{}
	app.EnableRBAC(mockProvider, "") // Empty string uses default paths
}

// mockRBACProvider is a minimal mock for testing
type mockRBACProvider struct{}

func (m *mockRBACProvider) LoadPermissions(file string) (any, error) {
	return nil, nil
}

func (m *mockRBACProvider) GetMiddleware(config any) func(http.Handler) http.Handler {
	return func(handler http.Handler) http.Handler {
		return handler
	}
}

func (m *mockRBACProvider) RequireRole(allowedRole string, handlerFunc func(any) (any, error)) func(any) (any, error) {
	return handlerFunc
}

func (m *mockRBACProvider) RequireAnyRole(allowedRoles []string, handlerFunc func(any) (any, error)) func(any) (any, error) {
	return handlerFunc
}

func (m *mockRBACProvider) RequirePermission(requiredPermission string, permissionConfig any, handlerFunc func(any) (any, error)) func(any) (any, error) {
	return handlerFunc
}

func (m *mockRBACProvider) ErrAccessDenied() error {
	return nil
}

func (m *mockRBACProvider) ErrPermissionDenied() error {
	return nil
}

// Old functional options tests removed - new API uses interface-based options
// See examples/rbac for usage of new API with HeaderRoleExtractor, JWTExtractor, etc.

func TestRequireRole_WithoutModule(t *testing.T) {
	handler := func(_ *Context) (any, error) {
		return "success", nil
	}

	wrapped := RequireRole("admin", handler)
	require.NotNil(t, wrapped)

	ctx := &Context{
		Container: container.NewContainer(config.NewMockConfig(nil)),
	}
	result, err := wrapped(ctx)
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RBAC module not imported")
}

func TestRequireAnyRole_WithoutModule(t *testing.T) {
	handler := func(_ *Context) (any, error) {
		return "success", nil
	}

	wrapped := RequireAnyRole([]string{"admin", "editor"}, handler)
	require.NotNil(t, wrapped)

	ctx := &Context{
		Container: container.NewContainer(config.NewMockConfig(nil)),
	}
	result, err := wrapped(ctx)
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RBAC module not imported")
}

func TestRequirePermission_WithoutModule(t *testing.T) {
	handler := func(_ *Context) (any, error) {
		return "success", nil
	}

	wrapped := RequirePermission("users:read", nil, handler)
	require.NotNil(t, wrapped)

	ctx := &Context{
		Container: container.NewContainer(config.NewMockConfig(nil)),
	}
	result, err := wrapped(ctx)
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RBAC module not imported")
}
