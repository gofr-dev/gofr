package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchEndpoint_ExactMatch(t *testing.T) {
	endpoints := []EndpointMapping{
		{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
	}
	endpoint, isPublic := matchEndpoint("GET", "/api/users", endpoints)
	require.NotNil(t, endpoint)
	assert.False(t, isPublic)
}

func TestMatchEndpoint_PublicEndpoint(t *testing.T) {
	endpoints := []EndpointMapping{
		{Path: "/health", Methods: []string{"GET"}, Public: true},
	}
	endpoint, isPublic := matchEndpoint("GET", "/health", endpoints)
	require.NotNil(t, endpoint)
	assert.True(t, isPublic)
}

func TestMatchEndpoint_DifferentMethod(t *testing.T) {
	endpoints := []EndpointMapping{
		{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
	}
	endpoint, isPublic := matchEndpoint("POST", "/api/users", endpoints)
	require.Nil(t, endpoint)
	assert.False(t, isPublic)
}

func TestMatchEndpoint_WildcardMethod(t *testing.T) {
	endpoints := []EndpointMapping{
		{Path: "/api", Methods: []string{"*"}, RequiredPermissions: []string{"api:*"}},
	}
	endpoint, isPublic := matchEndpoint("POST", "/api", endpoints)
	require.NotNil(t, endpoint)
	assert.False(t, isPublic)
}

func TestMatchEndpoint_WildcardPath(t *testing.T) {
	endpoints := []EndpointMapping{
		{Path: "/api/*", Methods: []string{"GET"}, RequiredPermissions: []string{"api:read"}},
	}
	endpoint, isPublic := matchEndpoint("GET", "/api/users", endpoints)
	require.NotNil(t, endpoint)
	assert.False(t, isPublic)
}

func TestMatchEndpoint_RegexPattern(t *testing.T) {
	endpoints := []EndpointMapping{
		{Regex: "^/api/users/\\d+$", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
	}
	endpoint, isPublic := matchEndpoint("GET", "/api/users/123", endpoints)
	require.NotNil(t, endpoint)
	assert.False(t, isPublic)
}

func TestMatchEndpoint_NotFound(t *testing.T) {
	endpoints := []EndpointMapping{
		{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
	}
	endpoint, isPublic := matchEndpoint("GET", "/api/posts", endpoints)
	require.Nil(t, endpoint)
	assert.False(t, isPublic)
}

func TestMatchEndpoint_EmptyMethods(t *testing.T) {
	endpoints := []EndpointMapping{
		{Path: "/api", Methods: []string{}, RequiredPermissions: []string{"api:*"}},
	}
	endpoint, isPublic := matchEndpoint("POST", "/api", endpoints)
	require.NotNil(t, endpoint)
	assert.False(t, isPublic)
}

func TestMatchesHTTPMethod(t *testing.T) {
	testCases := []struct {
		desc           string
		method         string
		allowedMethods []string
		expected       bool
	}{
		{
			desc:           "matches exact method",
			method:         "GET",
			allowedMethods: []string{"GET"},
			expected:       true,
		},
		{
			desc:           "matches case-insensitive method",
			method:         "get",
			allowedMethods: []string{"GET"},
			expected:       true,
		},
		{
			desc:           "matches wildcard method",
			method:         "POST",
			allowedMethods: []string{"*"},
			expected:       true,
		},
		{
			desc:           "matches empty methods as all",
			method:         "DELETE",
			allowedMethods: []string{},
			expected:       true,
		},
		{
			desc:           "does not match different method",
			method:         "POST",
			allowedMethods: []string{"GET"},
			expected:       false,
		},
		{
			desc:           "matches one of multiple methods",
			method:         "PUT",
			allowedMethods: []string{"GET", "PUT", "POST"},
			expected:       true,
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

func TestCheckEndpointAuthorization_PublicEndpoint(t *testing.T) {
	config := &Config{
		Roles: []RoleDefinition{
			{Name: "any", Permissions: []string{}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	endpoint := &EndpointMapping{Public: true}
	authorized, reason := checkEndpointAuthorization("any", endpoint, config)
	assert.True(t, authorized)
	assert.Equal(t, "public-endpoint", reason)
}

func TestCheckEndpointAuthorization_ExactPermission(t *testing.T) {
	config := &Config{
		Roles: []RoleDefinition{
			{Name: "admin", Permissions: []string{"users:read"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	endpoint := &EndpointMapping{RequiredPermissions: []string{"users:read"}}
	authorized, reason := checkEndpointAuthorization("admin", endpoint, config)
	assert.True(t, authorized)
	assert.Equal(t, "permission-based", reason)
}

func TestCheckEndpointAuthorization_WildcardsNotSupported(t *testing.T) {
	config := &Config{
		Roles: []RoleDefinition{
			{Name: "admin", Permissions: []string{"*:*"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	endpoint := &EndpointMapping{RequiredPermissions: []string{"users:read"}}
	authorized, reason := checkEndpointAuthorization("admin", endpoint, config)
	assert.False(t, authorized)
	assert.Empty(t, reason)
}

func TestCheckEndpointAuthorization_ResourceWildcardNotSupported(t *testing.T) {
	config := &Config{
		Roles: []RoleDefinition{
			{Name: "admin", Permissions: []string{"users:*"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	endpoint := &EndpointMapping{RequiredPermissions: []string{"users:read"}}
	authorized, reason := checkEndpointAuthorization("admin", endpoint, config)
	assert.False(t, authorized)
	assert.Empty(t, reason)
}

func TestCheckEndpointAuthorization_NoPermission(t *testing.T) {
	config := &Config{
		Roles: []RoleDefinition{
			{Name: "viewer", Permissions: []string{"users:read"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	endpoint := &EndpointMapping{RequiredPermissions: []string{"users:write"}}
	authorized, reason := checkEndpointAuthorization("viewer", endpoint, config)
	assert.False(t, authorized)
	assert.Empty(t, reason)
}

func TestCheckEndpointAuthorization_EmptyRequiredPermissions(t *testing.T) {
	config := &Config{
		Roles: []RoleDefinition{
			{Name: "admin", Permissions: []string{"*:*"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	endpoint := &EndpointMapping{RequiredPermissions: []string{}}
	authorized, reason := checkEndpointAuthorization("admin", endpoint, config)
	assert.False(t, authorized)
	assert.Empty(t, reason)
}

func TestCheckEndpointAuthorization_NoRolePermissions(t *testing.T) {
	config := &Config{
		Roles: []RoleDefinition{
			{Name: "guest", Permissions: []string{}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	endpoint := &EndpointMapping{RequiredPermissions: []string{"users:read"}}
	authorized, reason := checkEndpointAuthorization("guest", endpoint, config)
	assert.False(t, authorized)
	assert.Empty(t, reason)
}

func TestCheckEndpointAuthorization_InheritedPermissions(t *testing.T) {
	config := &Config{
		Roles: []RoleDefinition{
			{Name: "viewer", Permissions: []string{"users:read"}},
			{Name: "editor", Permissions: []string{"users:write"}, InheritsFrom: []string{"viewer"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	endpoint := &EndpointMapping{RequiredPermissions: []string{"users:read"}}
	authorized, reason := checkEndpointAuthorization("editor", endpoint, config)
	assert.True(t, authorized)
	assert.Equal(t, "permission-based", reason)
}

func TestCheckEndpointAuthorization_MultiplePermissions_OR_First(t *testing.T) {
	config := &Config{
		Roles: []RoleDefinition{
			{Name: "viewer", Permissions: []string{"users:read"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	endpoint := &EndpointMapping{RequiredPermissions: []string{"users:read", "users:admin"}}
	authorized, reason := checkEndpointAuthorization("viewer", endpoint, config)
	assert.True(t, authorized)
	assert.Equal(t, "permission-based", reason)
}

func TestCheckEndpointAuthorization_MultiplePermissions_OR_Second(t *testing.T) {
	config := &Config{
		Roles: []RoleDefinition{
			{Name: "admin", Permissions: []string{"users:admin"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	endpoint := &EndpointMapping{RequiredPermissions: []string{"users:read", "users:admin"}}
	authorized, reason := checkEndpointAuthorization("admin", endpoint, config)
	assert.True(t, authorized)
	assert.Equal(t, "permission-based", reason)
}

func TestCheckEndpointAuthorization_MultiplePermissions_None(t *testing.T) {
	config := &Config{
		Roles: []RoleDefinition{
			{Name: "guest", Permissions: []string{"posts:read"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	endpoint := &EndpointMapping{RequiredPermissions: []string{"users:read", "users:write"}}
	authorized, reason := checkEndpointAuthorization("guest", endpoint, config)
	assert.False(t, authorized)
	assert.Empty(t, reason)
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
			request: httptest.NewRequest(http.MethodGet, "/api/users", http.NoBody),
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
			request: httptest.NewRequest(http.MethodGet, "/health", http.NoBody),
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
			request: httptest.NewRequest(http.MethodGet, "/api/users", http.NoBody),
			config: &Config{
				Endpoints: []EndpointMapping{},
			},
			expectedMatch:  false,
			expectedPublic: false,
		},
		{
			desc:    "returns nil for non-matching request",
			request: httptest.NewRequest(http.MethodPost, "/api/posts", http.NoBody),
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
