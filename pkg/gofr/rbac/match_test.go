package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsRoleAllowed(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/admin/*": {"admin"},
			"/user/*":  {"user", "admin"},
			"*":        {"guest"},
		},
		OverRides: map[string]bool{
			"/admin/home": true,
		},
	}

	tests := []struct {
		name     string
		role     string
		route    string
		expected bool
	}{
		{"Override true", "anyone", "/admin/home", true},
		{"Pattern match /admin/*", "admin", "/admin/dashboard", true},
		{"Pattern match negative", "user", "/admin/dashboard", false},
		{"Non-pattern route", "user", "/user/profile", true},
		{"Wildcard permission", "guest", "/anything", true},
		{"No route or global match", "unknown", "/private", false},
		{"Not matched or globally allowed", "nobody", "/wildcard", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isRoleAllowed(tc.role, tc.route, config)
			assert.Equal(t, tc.expected, got, tc.name)
		})
	}
}

func TestIsRoleAllowed_NilConfig(t *testing.T) {
	got := isRoleAllowed("admin", "/test", nil)
	assert.False(t, got)
}

func TestIsRoleAllowed_OverrideFalse(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/test": {"admin"},
		},
		OverRides: map[string]bool{
			"/test": false,
		},
	}

	got := isRoleAllowed("admin", "/test", config)
	assert.True(t, got) // Override false still allows access (override means bypass)
}

func TestFindRoutePermissions_ExactMatch(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/api/users": {"admin", "editor"},
		},
	}

	perms := findRoutePermissions("/api/users", config)
	assert.Equal(t, []string{"admin", "editor"}, perms)
}

func TestFindRoutePermissions_PatternMatch(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/api/*": {"admin"},
		},
	}

	perms := findRoutePermissions("/api/users", config)
	assert.Equal(t, []string{"admin"}, perms)
}

func TestFindRoutePermissions_WithGlobalWildcard(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/api/users": {"admin"},
			"*":          {"guest"},
		},
	}

	// Specific routes take precedence - global wildcard is not appended
	perms := findRoutePermissions("/api/users", config)
	assert.Equal(t, []string{"admin"}, perms)
}

func TestFindRoutePermissions_OnlyGlobalWildcard(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"*": {"guest"},
		},
	}

	perms := findRoutePermissions("/any/route", config)
	assert.Equal(t, []string{"guest"}, perms)
}

func TestFindRoutePermissions_NoMatch(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/other": {"admin"},
		},
	}

	perms := findRoutePermissions("/api/users", config)
	assert.Nil(t, perms)
}

func TestFindRoutePermissions_NilConfig(t *testing.T) {
	perms := findRoutePermissions("/api/users", nil)
	assert.Nil(t, perms)
}

func TestFindRoutePermissions_NilRouteWithPermissions(t *testing.T) {
	config := &Config{}
	perms := findRoutePermissions("/api/users", config)
	assert.Nil(t, perms)
}

func TestFindRoutePermissions_EmptyRoute(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"": {"admin"},
		},
	}

	perms := findRoutePermissions("/api/users", config)
	assert.Nil(t, perms) // Empty route should not match
}

func TestFindRoutePermissions_PrefixMatchForWildcardPattern(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/api/admin/*": {"admin"},
			"*":            {"viewer"},
		},
	}

	// /api/admin should match /api/admin/* pattern (prefix match)
	perms := findRoutePermissions("/api/admin", config)
	assert.Equal(t, []string{"admin"}, perms) // Should only have admin, not viewer

	// /api/admin/something should also match
	perms2 := findRoutePermissions("/api/admin/something", config)
	assert.Equal(t, []string{"admin"}, perms2)
}

func TestFindRoutePermissions_SpecificRouteTakesPrecedenceOverGlobal(t *testing.T) {
	config := &Config{
		RouteWithPermissions: map[string][]string{
			"/api/users": {"admin", "editor"},
			"*":          {"viewer"},
		},
	}

	perms := findRoutePermissions("/api/users", config)
	assert.Equal(t, []string{"admin", "editor"}, perms) // Should not include viewer
}

func TestIsRoleInPermissions(t *testing.T) {
	tests := []struct {
		name        string
		role        string
		permissions []string
		expected    bool
	}{
		{"ExactMatch", "admin", []string{"admin", "editor"}, true},
		{"NoMatch", "viewer", []string{"admin", "editor"}, false},
		{"WildcardRole", "anyone", []string{"*"}, true},
		{"WildcardWithOthers", "admin", []string{"*", "editor"}, true},
		{"EmptyPermissions", "admin", []string{}, false},
		{"NilPermissions", "admin", nil, false},
		{"EmptyRole", "", []string{"admin"}, false},
		{"EmptyRoleWithWildcard", "", []string{"*"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isRoleInPermissions(tc.role, tc.permissions)
			assert.Equal(t, tc.expected, got, tc.name)
		})
	}
}
