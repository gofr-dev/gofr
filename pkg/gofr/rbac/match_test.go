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
