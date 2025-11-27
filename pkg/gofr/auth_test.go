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

func TestEnableRBAC_WithoutModule(_ *testing.T) {
	app := &App{
		container: container.NewContainer(config.NewMockConfig(nil)),
		httpServer: &httpServer{
			router: gofrHTTP.NewRouter(),
		},
	}

	app.EnableRBAC(
		WithPermissionsFile("test.json"),
		WithRoleExtractor(func(_ *http.Request, _ ...any) (string, error) {
			return "admin", nil
		}),
	)
}

func TestEnableRBAC_NoConfigProvided(_ *testing.T) {
	app := &App{
		container: container.NewContainer(config.NewMockConfig(nil)),
		httpServer: &httpServer{
			router: gofrHTTP.NewRouter(),
		},
	}

	app.EnableRBAC()
}

func TestRBACOptions_WithPermissionsFile(t *testing.T) {
	opts := &RBACOptions{}
	WithPermissionsFile("test.json")(opts)
	assert.Equal(t, "test.json", opts.PermissionsFile)
}

func TestRBACOptions_WithRoleExtractor(t *testing.T) {
	opts := &RBACOptions{}
	extractor := func(_ *http.Request, _ ...any) (string, error) {
		return "admin", nil
	}
	WithRoleExtractor(extractor)(opts)
	assert.NotNil(t, opts.RoleExtractor)
}

func TestRBACOptions_WithConfig(t *testing.T) {
	opts := &RBACOptions{}
	// Note: We can't create a real RBACConfig without importing the RBAC module
	// This test just verifies the option function works
	WithConfig(nil)(opts)
	assert.Nil(t, opts.Config)
}

func TestRBACOptions_WithJWT(t *testing.T) {
	opts := &RBACOptions{}
	WithJWT("role")(opts)
	assert.Equal(t, "role", opts.JWTRoleClaim)
}

func TestRBACOptions_WithPermissions(t *testing.T) {
	opts := &RBACOptions{}
	// Note: We can't create a real PermissionConfig without importing the RBAC module
	// This test just verifies the option function works
	WithPermissions(nil)(opts)
	assert.Nil(t, opts.PermissionConfig)
	assert.False(t, opts.RequiresContainer)
}

func TestRBACOptions_WithRequiresContainer(t *testing.T) {
	opts := &RBACOptions{}
	WithRequiresContainer(true)(opts)
	assert.True(t, opts.RequiresContainer)

	WithRequiresContainer(false)(opts)
	assert.False(t, opts.RequiresContainer)
}

func TestRBACOptions_MultipleOptions(t *testing.T) {
	opts := &RBACOptions{}
	extractor := func(_ *http.Request, _ ...any) (string, error) {
		return "admin", nil
	}

	WithPermissionsFile("test.json")(opts)
	WithRoleExtractor(extractor)(opts)
	WithRequiresContainer(true)(opts)

	assert.Equal(t, "test.json", opts.PermissionsFile)
	assert.NotNil(t, opts.RoleExtractor)
	assert.True(t, opts.RequiresContainer)
}

func TestRequireRole_WithoutModule(t *testing.T) {
	handler := func(_ *Context) (any, error) {
		return "success", nil
	}

	wrapped := RequireRole("admin", handler)
	require.NotNil(t, wrapped)

	ctx := &Context{}
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

	ctx := &Context{}
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

	ctx := &Context{}
	result, err := wrapped(ctx)
	assert.Nil(t, result)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RBAC module not imported")
}
