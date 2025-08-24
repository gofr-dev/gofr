package rbac

import "testing"

func TestIsPathAllowed_ExactAndWildcard(t *testing.T) {
	cases := []struct {
		name   string
		role   string
		route  string
		config *Config
		want   bool
	}{
		{
			name:  "exact match route",
			role:  "admin",
			route: "/dashboard",
			config: &Config{
				RoleWithPermissions: map[string][]string{"admin": {"/dashboard"}},
			},
			want: true,
		},
		{
			name:  "wildcard match any route",
			role:  "admin",
			route: "/anything/here",
			config: &Config{
				RoleWithPermissions: map[string][]string{"admin": {"*"}},
			},
			want: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPathAllowed(tt.role, tt.route, tt.config); got != tt.want {
				t.Errorf("isPathAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPathAllowed_PatternsAndPrefixes(t *testing.T) {
	cases := []struct {
		name   string
		role   string
		route  string
		config *Config
		want   bool
	}{
		{
			name:  "path.Match style match",
			role:  "admin",
			route: "/users/123",
			config: &Config{
				RoleWithPermissions: map[string][]string{"admin": {"/users/*"}},
			},
			want: true,
		},
		{
			name:  "prefix match with * suffix",
			role:  "editor",
			route: "/projects/42/files",
			config: &Config{
				RoleWithPermissions: map[string][]string{"editor": {"/projects/*"}},
			},
			want: true,
		},
		{
			name:  "trailing slash ignored",
			role:  "viewer",
			route: "/reports/",
			config: &Config{
				RoleWithPermissions: map[string][]string{"viewer": {"/reports"}},
			},
			want: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPathAllowed(tt.role, tt.route, tt.config); got != tt.want {
				t.Errorf("isPathAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsPathAllowed_Overrides(t *testing.T) {
	cases := []struct {
		name   string
		role   string
		route  string
		config *Config
		want   bool
	}{
		{
			name:  "no match but overridden",
			role:  "special",
			route: "/secret/area",
			config: &Config{
				RoleWithPermissions: map[string][]string{"special": {}},
				OverRides:           map[string]bool{"special": true},
			},
			want: true,
		},
		{
			name:  "no permissions no override",
			role:  "guest",
			route: "/dashboard",
			config: &Config{
				RoleWithPermissions: map[string][]string{"guest": {}},
				OverRides:           map[string]bool{},
			},
			want: false,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPathAllowed(tt.role, tt.route, tt.config); got != tt.want {
				t.Errorf("isPathAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}
