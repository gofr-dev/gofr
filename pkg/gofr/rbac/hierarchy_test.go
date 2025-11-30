package rbac

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRoleHierarchy(t *testing.T) {
	hierarchy := map[string][]string{
		"admin":  {"editor", "author", "viewer"},
		"editor": {"author", "viewer"},
		"author": {"viewer"},
	}

	rh := NewRoleHierarchy(hierarchy)
	assert.NotNil(t, rh)

	// Test nil hierarchy
	rh2 := NewRoleHierarchy(nil)
	assert.NotNil(t, rh2)
}

func TestRoleHierarchy_GetEffectiveRoles(t *testing.T) {
	hierarchy := map[string][]string{
		"admin":  {"editor", "author", "viewer"},
		"editor": {"author", "viewer"},
		"author": {"viewer"},
	}

	rh := NewRoleHierarchy(hierarchy)

	tests := []struct {
		name     string
		role     string
		want     []string
		contains []string // Roles that should be in the result
	}{
		{
			name:     "Admin role",
			role:     "admin",
			contains: []string{"admin", "editor", "author", "viewer"},
		},
		{
			name:     "Editor role",
			role:     "editor",
			contains: []string{"editor", "author", "viewer"},
		},
		{
			name:     "Author role",
			role:     "author",
			contains: []string{"author", "viewer"},
		},
		{
			name:     "Viewer role",
			role:     "viewer",
			contains: []string{"viewer"},
		},
		{
			name:     "Unknown role",
			role:     "unknown",
			contains: []string{"unknown"},
		},
		{
			name:     "Empty role",
			role:     "",
			contains: []string{}, // Empty role returns empty slice
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effectiveRoles := rh.GetEffectiveRoles(tt.role)
			// Empty role returns empty slice, so skip the contains check
			if tt.role != "" {
				assert.Contains(t, effectiveRoles, tt.role) // Should always contain itself
			}

			for _, expectedRole := range tt.contains {
				assert.Contains(t, effectiveRoles, expectedRole, "Effective roles should contain %s", expectedRole)
			}
		})
	}
}

func TestRoleHierarchy_GetEffectiveRoles_Circular(t *testing.T) {
	// Test that circular references don't cause infinite loops
	hierarchy := map[string][]string{
		"admin":  {"editor"},
		"editor": {"admin"}, // Circular reference
	}

	rh := NewRoleHierarchy(hierarchy)

	effectiveRoles := rh.GetEffectiveRoles("admin")
	assert.Contains(t, effectiveRoles, "admin")
	assert.Contains(t, effectiveRoles, "editor")
	// Should not have duplicates
	roleCount := make(map[string]int)
	for _, role := range effectiveRoles {
		roleCount[role]++
	}

	for _, count := range roleCount {
		assert.Equal(t, 1, count, "No role should appear more than once")
	}
}

func TestRoleHierarchy_HasRole(t *testing.T) {
	hierarchy := map[string][]string{
		"admin":  {"editor", "author", "viewer"},
		"editor": {"author", "viewer"},
	}

	rh := NewRoleHierarchy(hierarchy)

	tests := []struct {
		name         string
		ctxRole      string
		requiredRole string
		want         bool
	}{
		{
			name:         "Direct match",
			ctxRole:      "admin",
			requiredRole: "admin",
			want:         true,
		},
		{
			name:         "Inherited role match",
			ctxRole:      "admin",
			requiredRole: "editor",
			want:         true,
		},
		{
			name:         "Deep inheritance",
			ctxRole:      "admin",
			requiredRole: "viewer",
			want:         true,
		},
		{
			name:         "No match",
			ctxRole:      "viewer",
			requiredRole: "admin",
			want:         false,
		},
		{
			name:         "No role in context",
			ctxRole:      "",
			requiredRole: "admin",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), userRole, tt.ctxRole)
			got := rh.HasRole(ctx, tt.requiredRole)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestRoleHierarchy_HasAnyRole(t *testing.T) {
	hierarchy := map[string][]string{
		"admin":  {"editor", "author", "viewer"},
		"editor": {"author", "viewer"},
	}

	rh := NewRoleHierarchy(hierarchy)

	tests := []struct {
		name          string
		ctxRole       string
		requiredRoles []string
		want          bool
	}{
		{
			name:          "Direct match",
			ctxRole:       "admin",
			requiredRoles: []string{"admin", "editor"},
			want:          true,
		},
		{
			name:          "Inherited role match",
			ctxRole:       "admin",
			requiredRoles: []string{"editor", "author"},
			want:          true,
		},
		{
			name:          "No match",
			ctxRole:       "viewer",
			requiredRoles: []string{"admin", "editor"},
			want:          false,
		},
		{
			name:          "Empty required roles",
			ctxRole:       "admin",
			requiredRoles: []string{},
			want:          false,
		},
		{
			name:          "No role in context",
			ctxRole:       "",
			requiredRoles: []string{"admin", "editor"},
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), userRole, tt.ctxRole)
			got := rh.HasAnyRole(ctx, tt.requiredRoles)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsRoleAllowedWithHierarchy(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/api/users": {"editor", "viewer"},
			"/api/admin": {"admin"},
		},
	}

	hierarchy := map[string][]string{
		"admin": {"editor", "viewer"},
	}

	rh := NewRoleHierarchy(hierarchy)

	tests := []struct {
		name    string
		role    string
		route   string
		want    bool
		useHier bool
	}{
		{
			name:    "With hierarchy - admin can access editor route",
			role:    "admin",
			route:   "/api/users",
			want:    true,
			useHier: true,
		},
		{
			name:    "With hierarchy - editor can access editor route",
			role:    "editor",
			route:   "/api/users",
			want:    true,
			useHier: true,
		},
		{
			name:    "With hierarchy - viewer cannot access admin route",
			role:    "viewer",
			route:   "/api/admin",
			want:    false,
			useHier: true,
		},
		{
			name:    "Without hierarchy - admin cannot access editor route",
			role:    "admin",
			route:   "/api/users",
			want:    false,
			useHier: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getIsRoleAllowedWithHierarchy(tt.role, tt.route, config, rh, tt.useHier)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsRoleAllowedWithHierarchy_NilHierarchy(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/api/users": {"admin"},
		},
	}

	// Should fallback to regular role check
	got := IsRoleAllowedWithHierarchy("admin", "/api/users", config, nil)
	assert.True(t, got)

	got = IsRoleAllowedWithHierarchy("user", "/api/users", config, nil)
	assert.False(t, got)
}

func TestRoleHierarchy_ThreadSafety(t *testing.T) {
	hierarchy := map[string][]string{
		"admin": {"editor", "viewer"},
	}

	rh := NewRoleHierarchy(hierarchy)

	// Concurrent reads
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			roles := rh.GetEffectiveRoles("admin")

			assert.NotEmpty(t, roles)

			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestRoleHierarchy_ComplexInheritance(t *testing.T) {
	hierarchy := map[string][]string{
		"superadmin": {"admin", "editor", "author", "viewer"},
		"admin":      {"editor", "author", "viewer"},
		"editor":     {"author", "viewer"},
		"author":     {"viewer"},
	}

	rh := NewRoleHierarchy(hierarchy)

	// Test deep inheritance
	effectiveRoles := rh.GetEffectiveRoles("superadmin")
	assert.Contains(t, effectiveRoles, "superadmin")
	assert.Contains(t, effectiveRoles, "admin")
	assert.Contains(t, effectiveRoles, "editor")
	assert.Contains(t, effectiveRoles, "author")
	assert.Contains(t, effectiveRoles, "viewer")

	// Test that superadmin has all roles
	ctx := context.WithValue(context.Background(), userRole, "superadmin")
	assert.True(t, rh.HasRole(ctx, "viewer"))
	assert.True(t, rh.HasRole(ctx, "author"))
	assert.True(t, rh.HasRole(ctx, "editor"))
	assert.True(t, rh.HasRole(ctx, "admin"))
	assert.True(t, rh.HasRole(ctx, "superadmin"))
}

func TestRoleHierarchy_EmptyHierarchy(t *testing.T) {
	rh := NewRoleHierarchy(map[string][]string{})

	effectiveRoles := rh.GetEffectiveRoles("admin")
	assert.Equal(t, []string{"admin"}, effectiveRoles)

	ctx := context.WithValue(context.Background(), userRole, "admin")
	assert.True(t, rh.HasRole(ctx, "admin"))
	assert.False(t, rh.HasRole(ctx, "editor"))
}

func getIsRoleAllowedWithHierarchy(role, route string, config *Config, hierarchy *RoleHierarchy, useHier bool) bool {
	if !useHier {
		return IsRoleAllowedWithHierarchy(role, route, config, nil)
	}

	return IsRoleAllowedWithHierarchy(role, route, config, hierarchy)
}
