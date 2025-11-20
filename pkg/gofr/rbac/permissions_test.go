package rbac

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasPermission(t *testing.T) {
	config := &PermissionConfig{
		Permissions: map[string][]string{
			"users:read":  {"admin", "editor", "viewer"},
			"users:write": {"admin", "editor"},
			"posts:read":   {"admin", "author", "viewer"},
		},
	}

	tests := []struct {
		name       string
		ctx        context.Context
		permission string
		want       bool
	}{
		{
			name:       "Has permission with admin role",
			ctx:        context.WithValue(context.Background(), userRole, "admin"),
			permission: "users:read",
			want:       true,
		},
		{
			name:       "Has permission with editor role",
			ctx:        context.WithValue(context.Background(), userRole, "editor"),
			permission: "users:read",
			want:       true,
		},
		{
			name:       "No permission with viewer role",
			ctx:        context.WithValue(context.Background(), userRole, "viewer"),
			permission: "users:write",
			want:       false,
		},
		{
			name:       "Permission not found",
			ctx:        context.WithValue(context.Background(), userRole, "admin"),
			permission: "nonexistent:permission",
			want:       false,
		},
		{
			name:       "No role in context",
			ctx:        context.Background(),
			permission: "users:read",
			want:       false,
		},
		{
			name:       "Wildcard permission",
			ctx:        context.WithValue(context.Background(), userRole, "admin"),
			permission: "users:read",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasPermission(tt.ctx, tt.permission, config)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetRequiredPermission(t *testing.T) {
	config := &PermissionConfig{
		RoutePermissionMap: map[string]string{
			"GET /api/users":    "users:read",
			"POST /api/users":   "users:write",
			"GET /api/posts/*":  "posts:read",
			"* /api/public/*":   "public:read",
		},
		DefaultPermission: "default:read",
	}

	tests := []struct {
		name       string
		method     string
		route      string
		want       string
		wantErr    bool
		checkErr   func(*testing.T, error)
	}{
		{
			name:   "Exact match",
			method: "GET",
			route:  "/api/users",
			want:   "users:read",
			wantErr: false,
		},
		{
			name:   "Pattern match with wildcard",
			method: "GET",
			route:  "/api/posts/123",
			want:   "posts:read",
			wantErr: false,
		},
		{
			name:   "Method wildcard",
			method: "PUT",
			route:  "/api/public/test",
			want:   "public:read",
			wantErr: false,
		},
		{
			name:   "Default permission",
			method: "GET",
			route:  "/api/unknown",
			want:   "default:read",
			wantErr: false,
		},
		{
			name:    "No match and no default",
			method:  "GET",
			route:   "/api/unknown",
			want:    "",
			wantErr: true,
			checkErr: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), "no permission mapping found")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create config with or without default
			testConfig := config
			if tt.name == "No match and no default" {
				testConfig = &PermissionConfig{
					RoutePermissionMap: config.RoutePermissionMap,
					// No DefaultPermission
				}
			}

			got, err := GetRequiredPermission(tt.method, tt.route, testConfig)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.checkErr != nil {
					tt.checkErr(t, err)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestCheckPermission(t *testing.T) {
	config := &PermissionConfig{
		Permissions: map[string][]string{
			"users:read":  {"admin", "editor", "viewer"},
			"users:write": {"admin", "editor"},
		},
		RoutePermissionMap: map[string]string{
			"GET /api/users":  "users:read",
			"POST /api/users": "users:write",
		},
	}

	tests := []struct {
		name      string
		method    string
		route     string
		role      string
		wantErr   bool
		wantErrIs error
	}{
		{
			name:    "Authorized - admin with read",
			method:  "GET",
			route:   "/api/users",
			role:    "admin",
			wantErr: false,
		},
		{
			name:    "Authorized - editor with read",
			method:  "GET",
			route:   "/api/users",
			role:    "editor",
			wantErr: false,
		},
		{
			name:      "Unauthorized - viewer with write",
			method:    "POST",
			route:     "/api/users",
			role:      "viewer",
			wantErr:   true,
			wantErrIs: ErrPermissionDenied,
		},
		{
			name:      "No role in context",
			method:    "GET",
			route:     "/api/users",
			role:      "",
			wantErr:   true,
			wantErrIs: ErrPermissionDenied,
		},
		{
			name:      "No permission mapping",
			method:    "GET",
			route:     "/api/unknown",
			role:      "admin",
			wantErr:   true,
			wantErrIs: ErrPermissionDenied,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.route, nil)
			ctx := context.WithValue(req.Context(), userRole, tt.role)
			req = req.WithContext(ctx)

			err := CheckPermission(req, config)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.wantErrIs != nil {
					assert.ErrorIs(t, err, tt.wantErrIs)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCheckPermission_NilConfig(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/users", nil)
	err := CheckPermission(req, nil)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrPermissionDenied)
}

func TestMatchesRoutePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		method  string
		route   string
		want    bool
	}{
		{
			name:    "Exact match",
			pattern: "GET /api/users",
			method:  "GET",
			route:   "/api/users",
			want:    true,
		},
		{
			name:    "Method mismatch",
			pattern: "GET /api/users",
			method:  "POST",
			route:   "/api/users",
			want:    false,
		},
		{
			name:    "Route mismatch",
			pattern: "GET /api/users",
			method:  "GET",
			route:   "/api/posts",
			want:    false,
		},
		{
			name:    "Wildcard method",
			pattern: "* /api/users",
			method:  "POST",
			route:   "/api/users",
			want:    true,
		},
		{
			name:    "Wildcard route",
			pattern: "GET /api/*",
			method:  "GET",
			route:   "/api/users",
			want:    true,
		},
		{
			name:    "Invalid pattern format",
			pattern: "invalid",
			method:  "GET",
			route:   "/api/users",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesRoutePattern(tt.pattern, tt.method, tt.route)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRequirePermission(t *testing.T) {
	config := &PermissionConfig{
		Permissions: map[string][]string{
			"users:read": {"admin", "editor"},
		},
	}

	tests := []struct {
		name           string
		permission     string
		ctxRole        string
		wantErr        bool
		wantHandlerCall bool
	}{
		{
			name:           "Has permission",
			permission:     "users:read",
			ctxRole:        "admin",
			wantErr:        false,
			wantHandlerCall: true,
		},
		{
			name:           "No permission",
			permission:     "users:read",
			ctxRole:        "viewer",
			wantErr:        true,
			wantHandlerCall: false,
		},
		{
			name:           "No role",
			permission:     "users:read",
			ctxRole:        "",
			wantErr:        true,
			wantHandlerCall: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlerCalled := false
			handlerFunc := func(ctx interface{}) (any, error) {
				handlerCalled = true
				return "success", nil
			}

			wrapped := RequirePermission(tt.permission, config, handlerFunc)

			// Create mock context that implements ContextValueGetter
			ctx := &mockContextValueGetter{
				value: func(key interface{}) interface{} {
					if key == userRole {
						return tt.ctxRole
					}
					return nil
				},
			}

			result, err := wrapped(ctx)
			if tt.wantErr {
				assert.Error(t, err)
				assert.ErrorIs(t, err, ErrPermissionDenied)
				assert.False(t, handlerCalled)
			} else {
				assert.NoError(t, err)
				assert.True(t, handlerCalled)
				assert.Equal(t, "success", result)
			}
		})
	}
}

func TestRequirePermission_NilConfig(t *testing.T) {
	handlerFunc := func(ctx interface{}) (any, error) {
		return "success", nil
	}

	wrapped := RequirePermission("users:read", nil, handlerFunc)

	ctx := &mockContextValueGetter{
		value: func(key interface{}) interface{} {
			if key == userRole {
				return "admin"
			}
			return nil
		},
	}

	result, err := wrapped(ctx)
	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrPermissionDenied)
	assert.Nil(t, result)
}

func TestPermissionConfig_WithDefaultPermission(t *testing.T) {
	config := &PermissionConfig{
		Permissions: map[string][]string{
			"default:read": {"admin", "viewer"},
		},
		DefaultPermission: "default:read",
		RoutePermissionMap: map[string]string{
			"GET /api/specific": "specific:read",
		},
	}

	// Test default permission is used when route not in map
	permission, err := GetRequiredPermission("GET", "/api/unknown", config)
	require.NoError(t, err)
	assert.Equal(t, "default:read", permission)

	// Test specific permission takes precedence
	permission, err = GetRequiredPermission("GET", "/api/specific", config)
	require.NoError(t, err)
	assert.Equal(t, "specific:read", permission)
}

