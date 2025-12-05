package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchEndpoint(t *testing.T) {
	testCases := []struct {
		desc           string
		method         string
		route          string
		endpoints      []EndpointMapping
		expectedMatch  bool
		expectedPublic bool
	}{
		{
			desc:   "matches exact endpoint",
			method: "GET",
			route:  "/api/users",
			endpoints: []EndpointMapping{
				{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
			},
			expectedMatch:  true,
			expectedPublic: false,
		},
		{
			desc:   "matches public endpoint",
			method: "GET",
			route:  "/health",
			endpoints: []EndpointMapping{
				{Path: "/health", Methods: []string{"GET"}, Public: true},
			},
			expectedMatch:  true,
			expectedPublic: true,
		},
		{
			desc:   "does not match different method",
			method: "POST",
			route:  "/api/users",
			endpoints: []EndpointMapping{
				{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
			},
			expectedMatch:  false,
			expectedPublic: false,
		},
		{
			desc:   "matches wildcard method",
			method: "POST",
			route:  "/api",
			endpoints: []EndpointMapping{
				{Path: "/api", Methods: []string{"*"}, RequiredPermissions: []string{"api:*"}},
			},
			expectedMatch:  true,
			expectedPublic: false,
		},
		{
			desc:   "matches wildcard path pattern",
			method: "GET",
			route:  "/api/users",
			endpoints: []EndpointMapping{
				{Path: "/api/*", Methods: []string{"GET"}, RequiredPermissions: []string{"api:read"}},
			},
			expectedMatch:  true,
			expectedPublic: false,
		},
		{
			desc:   "matches regex pattern",
			method: "GET",
			route:  "/api/users/123",
			endpoints: []EndpointMapping{
				{Regex: "^/api/users/\\d+$", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
			},
			expectedMatch:  true,
			expectedPublic: false,
		},
		{
			desc:   "does not match non-existent route",
			method: "GET",
			route:  "/api/posts",
			endpoints: []EndpointMapping{
				{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
			},
			expectedMatch:  false,
			expectedPublic: false,
		},
		{
			desc:   "matches empty methods as all methods",
			method: "POST",
			route:  "/api",
			endpoints: []EndpointMapping{
				{Path: "/api", Methods: []string{}, RequiredPermissions: []string{"api:*"}},
			},
			expectedMatch:  true,
			expectedPublic: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			endpoint, isPublic := matchEndpoint(tc.method, tc.route, tc.endpoints)

			if tc.expectedMatch {
				require.NotNil(t, endpoint, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Equal(t, tc.expectedPublic, isPublic, "TEST[%d], Failed.\n%s", i, tc.desc)
				return
			}

			require.Nil(t, endpoint, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.False(t, isPublic, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestMatchesHTTPMethod(t *testing.T) {
	testCases := []struct {
		desc          string
		method        string
		allowedMethods []string
		expected      bool
	}{
		{
			desc:          "matches exact method",
			method:        "GET",
			allowedMethods: []string{"GET"},
			expected:      true,
		},
		{
			desc:          "matches case-insensitive method",
			method:        "get",
			allowedMethods: []string{"GET"},
			expected:      true,
		},
		{
			desc:          "matches wildcard method",
			method:        "POST",
			allowedMethods: []string{"*"},
			expected:      true,
		},
		{
			desc:          "matches empty methods as all",
			method:        "DELETE",
			allowedMethods: []string{},
			expected:      true,
		},
		{
			desc:          "does not match different method",
			method:        "POST",
			allowedMethods: []string{"GET"},
			expected:      false,
		},
		{
			desc:          "matches one of multiple methods",
			method:        "PUT",
			allowedMethods: []string{"GET", "PUT", "POST"},
			expected:      true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := matchesHTTPMethod(tc.method, tc.allowedMethods)

			assert.Equal(t, tc.expected, result, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestMatchesEndpointPattern(t *testing.T) {
	testCases := []struct {
		desc     string
		endpoint *EndpointMapping
		route    string
		expected bool
	}{
		{
			desc: "matches exact path",
			endpoint: &EndpointMapping{
				Path: "/api/users",
			},
			route:    "/api/users",
			expected: true,
		},
		{
			desc: "matches regex pattern",
			endpoint: &EndpointMapping{
				Regex: "^/api/users/\\d+$",
			},
			route:    "/api/users/123",
			expected: true,
		},
		{
			desc: "regex takes precedence over path",
			endpoint: &EndpointMapping{
				Path:  "/api/users",
				Regex: "^/api/users/\\d+$",
			},
			route:    "/api/users/123",
			expected: true,
		},
		{
			desc: "matches wildcard pattern",
			endpoint: &EndpointMapping{
				Path: "/api/*",
			},
			route:    "/api/users",
			expected: true,
		},
		{
			desc: "matches wildcard pattern with exact prefix",
			endpoint: &EndpointMapping{
				Path: "/api/*",
			},
			route:    "/api",
			expected: true,
		},
		{
			desc: "does not match different path",
			endpoint: &EndpointMapping{
				Path: "/api/users",
			},
			route:    "/api/posts",
			expected: false,
		},
		{
			desc: "does not match invalid regex",
			endpoint: &EndpointMapping{
				Regex: "[invalid",
			},
			route:    "/api/users",
			expected: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := matchesEndpointPattern(tc.endpoint, tc.route)

			assert.Equal(t, tc.expected, result, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestCheckEndpointAuthorization(t *testing.T) {
	testCases := []struct {
		desc              string
		role              string
		endpoint          *EndpointMapping
		config            *Config
		expectedAuthorized bool
		expectedReason    string
	}{
		{
			desc: "authorizes public endpoint",
			role:  "any",
			endpoint: &EndpointMapping{
				Public: true,
			},
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "any", Permissions: []string{}},
				},
			},
			expectedAuthorized: true,
			expectedReason:     "public-endpoint",
		},
		{
			desc: "authorizes role with exact permission",
			role:  "admin",
			endpoint: &EndpointMapping{
				RequiredPermissions: []string{"users:read"},
			},
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"users:read"}},
				},
			},
			expectedAuthorized: true,
			expectedReason:     "permission-based",
		},
		{
			desc: "denies role with wildcard permission (wildcards not supported)",
			role:  "admin",
			endpoint: &EndpointMapping{
				RequiredPermissions: []string{"users:read"},
			},
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"*:*"}},
				},
			},
			expectedAuthorized: false,
			expectedReason:     "",
		},
		{
			desc: "denies role with resource wildcard (wildcards not supported)",
			role:  "admin",
			endpoint: &EndpointMapping{
				RequiredPermissions: []string{"users:read"},
			},
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"users:*"}},
				},
			},
			expectedAuthorized: false,
			expectedReason:     "",
		},
		{
			desc: "denies role without permission",
			role:  "viewer",
			endpoint: &EndpointMapping{
				RequiredPermissions: []string{"users:write"},
			},
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "viewer", Permissions: []string{"users:read"}},
				},
			},
			expectedAuthorized: false,
			expectedReason:     "",
		},
		{
			desc: "denies when requiredPermission is empty",
			role:  "admin",
			endpoint: &EndpointMapping{
				RequiredPermissions: []string{},
			},
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"*:*"}},
				},
			},
			expectedAuthorized: false,
			expectedReason:     "",
		},
		{
			desc: "denies when role has no permissions",
			role:  "guest",
			endpoint: &EndpointMapping{
				RequiredPermissions: []string{"users:read"},
			},
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "guest", Permissions: []string{}},
				},
			},
			expectedAuthorized: false,
			expectedReason:     "",
		},
		{
			desc: "authorizes with inherited permissions",
			role:  "editor",
			endpoint: &EndpointMapping{
				RequiredPermissions: []string{"users:read"},
			},
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "viewer", Permissions: []string{"users:read"}},
					{Name: "editor", Permissions: []string{"users:write"}, InheritsFrom: []string{"viewer"}},
				},
			},
			expectedAuthorized: true,
			expectedReason:     "permission-based",
		},
		{
			desc: "authorizes with multiple required permissions (OR logic - has first)",
			role:  "viewer",
			endpoint: &EndpointMapping{
				RequiredPermissions: []string{"users:read", "users:admin"},
			},
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "viewer", Permissions: []string{"users:read"}},
				},
			},
			expectedAuthorized: true,
			expectedReason:     "permission-based",
		},
		{
			desc: "authorizes with multiple required permissions (OR logic - has second)",
			role:  "admin",
			endpoint: &EndpointMapping{
				RequiredPermissions: []string{"users:read", "users:admin"},
			},
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"users:admin"}},
				},
			},
			expectedAuthorized: true,
			expectedReason:     "permission-based",
		},
		{
			desc: "denies when role has none of the required permissions",
			role:  "guest",
			endpoint: &EndpointMapping{
				RequiredPermissions: []string{"users:read", "users:write"},
			},
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "guest", Permissions: []string{"posts:read"}},
				},
			},
			expectedAuthorized: false,
			expectedReason:     "",
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.config.processUnifiedConfig()
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

			authorized, reason := checkEndpointAuthorization(tc.role, tc.endpoint, tc.config)

			assert.Equal(t, tc.expectedAuthorized, authorized, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.expectedReason, reason, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestGetEndpointForRequest(t *testing.T) {
	testCases := []struct {
		desc           string
		request        *http.Request
		config         *Config
		expectedMatch  bool
		expectedPublic bool
	}{
		{
			desc:    "matches endpoint for request",
			request: httptest.NewRequest("GET", "/api/users", nil),
			config: &Config{
				Endpoints: []EndpointMapping{
					{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
				},
			},
			expectedMatch:  true,
			expectedPublic: false,
		},
		{
			desc:    "matches public endpoint",
			request: httptest.NewRequest("GET", "/health", nil),
			config: &Config{
				Endpoints: []EndpointMapping{
					{Path: "/health", Methods: []string{"GET"}, Public: true},
				},
			},
			expectedMatch:  true,
			expectedPublic: true,
		},
		{
			desc:    "returns nil for empty endpoints",
			request: httptest.NewRequest("GET", "/api/users", nil),
			config: &Config{
				Endpoints: []EndpointMapping{},
			},
			expectedMatch:  false,
			expectedPublic: false,
		},
		{
			desc:    "returns nil for non-matching request",
			request: httptest.NewRequest("POST", "/api/posts", nil),
			config: &Config{
				Endpoints: []EndpointMapping{
					{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
				},
			},
			expectedMatch:  false,
			expectedPublic: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.config.processUnifiedConfig()
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

			endpoint, isPublic := getEndpointForRequest(tc.request, tc.config)

			if tc.expectedMatch {
				require.NotNil(t, endpoint, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Equal(t, tc.expectedPublic, isPublic, "TEST[%d], Failed.\n%s", i, tc.desc)
				return
			}

			require.Nil(t, endpoint, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.False(t, isPublic, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

