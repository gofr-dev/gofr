package rbac

import "testing"

func TestIsPathAllowed(t *testing.T) {
	type args struct {
		role   string
		route  string
		config *Config
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "exact match route",
			args: args{
				role:  "admin",
				route: "/dashboard",
				config: &Config{
					RoleWithPermissions: map[string][]string{
						"admin": {"/dashboard"},
					},
				},
			},
			want: true,
		},
		{
			name: "wildcard match any route",
			args: args{
				role:  "admin",
				route: "/anything/here",
				config: &Config{
					RoleWithPermissions: map[string][]string{
						"admin": {"*"},
					},
				},
			},
			want: true,
		},
		{
			name: "path.Match style match",
			args: args{
				role:  "admin",
				route: "/users/123",
				config: &Config{
					RoleWithPermissions: map[string][]string{
						"admin": {"/users/*"},
					},
				},
			},
			want: true,
		},
		{
			name: "prefix match with * suffix",
			args: args{
				role:  "editor",
				route: "/projects/42/files",
				config: &Config{
					RoleWithPermissions: map[string][]string{
						"editor": {"/projects/"},
					},
				},
			},
			want: false,
		},
		{
			name: "prefix * suffix allowed",
			args: args{
				role:  "editor",
				route: "/projects/42/files",
				config: &Config{
					RoleWithPermissions: map[string][]string{
						"editor": {"/projects/*"},
					},
				},
			},
			want: true,
		},
		{
			name: "trailing slash ignored",
			args: args{
				role:  "viewer",
				route: "/reports/",
				config: &Config{
					RoleWithPermissions: map[string][]string{
						"viewer": {"/reports"},
					},
				},
			},
			want: true,
		},
		{
			name: "no match but overridden by config",
			args: args{
				role:  "special",
				route: "/secret/area",
				config: &Config{
					RoleWithPermissions: map[string][]string{
						"special": {},
					},
					OverRides: map[string]bool{
						"special": true,
					},
				},
			},
			want: true,
		},
		{
			name: "no permissions no override",
			args: args{
				role:  "guest",
				route: "/dashboard",
				config: &Config{
					RoleWithPermissions: map[string][]string{
						"guest": {},
					},
					OverRides: map[string]bool{},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isPathAllowed(tt.args.role, tt.args.route, tt.args.config); got != tt.want {
				t.Errorf("isPathAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}
