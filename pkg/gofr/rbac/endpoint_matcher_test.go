package rbac

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchEndpoint_ExactMatch(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
		},
	}
	_ = config.processUnifiedConfig()
	endpoints := config.Endpoints
	endpoint, isPublic := matchEndpoint("GET", "/api/users", endpoints, config)
	require.NotNil(t, endpoint)
	assert.False(t, isPublic)
}

func TestMatchEndpoint_PublicEndpoint(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/health", Methods: []string{"GET"}, Public: true},
		},
	}
	_ = config.processUnifiedConfig()
	endpoints := config.Endpoints
	endpoint, isPublic := matchEndpoint("GET", "/health", endpoints, config)
	require.NotNil(t, endpoint)
	assert.True(t, isPublic)
}

func TestMatchEndpoint_DifferentMethod(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
		},
	}
	_ = config.processUnifiedConfig()
	endpoints := config.Endpoints
	endpoint, isPublic := matchEndpoint("POST", "/api/users", endpoints, config)
	require.Nil(t, endpoint)
	assert.False(t, isPublic)
}

func TestMatchEndpoint_WildcardMethod(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/api", Methods: []string{"*"}, RequiredPermissions: []string{"api:*"}},
		},
	}
	_ = config.processUnifiedConfig()
	endpoints := config.Endpoints
	endpoint, isPublic := matchEndpoint("POST", "/api", endpoints, config)
	require.NotNil(t, endpoint)
	assert.False(t, isPublic)
}

func TestMatchEndpoint_MuxPatternPath(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/api/{resource}", Methods: []string{"GET"}, RequiredPermissions: []string{"api:read"}},
		},
	}
	_ = config.processUnifiedConfig()
	endpoints := config.Endpoints
	endpoint, isPublic := matchEndpoint("GET", "/api/users", endpoints, config)
	require.NotNil(t, endpoint)
	assert.False(t, isPublic)
}

func TestMatchEndpoint_MuxPatternWithConstraint(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/api/users/{id:[0-9]+}", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
		},
	}
	_ = config.processUnifiedConfig()
	endpoints := config.Endpoints
	endpoint, isPublic := matchEndpoint("GET", "/api/users/123", endpoints, config)
	require.NotNil(t, endpoint)
	assert.False(t, isPublic)
}

func TestMatchEndpoint_NotFound(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
		},
	}
	_ = config.processUnifiedConfig()
	endpoints := config.Endpoints
	endpoint, isPublic := matchEndpoint("GET", "/api/posts", endpoints, config)
	require.Nil(t, endpoint)
	assert.False(t, isPublic)
}

func TestMatchEndpoint_EmptyMethods(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/api", Methods: []string{}, RequiredPermissions: []string{"api:*"}},
		},
	}
	_ = config.processUnifiedConfig()
	endpoints := config.Endpoints
	endpoint, isPublic := matchEndpoint("POST", "/api", endpoints, config)
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
			desc: "matches mux pattern with constraint",
			endpoint: &EndpointMapping{
				Path: "/api/users/{id:[0-9]+}",
			},
			route:    "/api/users/123",
			expected: true,
		},
		{
			desc: "matches mux pattern single variable",
			endpoint: &EndpointMapping{
				Path: "/api/{resource}",
			},
			route:    "/api/users",
			expected: true,
		},
		{
			desc: "matches mux pattern with exact prefix",
			endpoint: &EndpointMapping{
				Path: "/api/{resource}",
			},
			route:    "/api",
			expected: false, // /api/{resource} requires a segment after /api
		},
		{
			desc: "matches mux pattern multi-level",
			endpoint: &EndpointMapping{
				Path: "/api/{path:.*}",
			},
			route:    "/api/users/123",
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
			desc: "does not match invalid mux pattern",
			endpoint: &EndpointMapping{
				Path: "/api/{invalid",
			},
			route:    "/api/users",
			expected: false,
		},
		{
			desc: "does not match constraint violation",
			endpoint: &EndpointMapping{
				Path: "/api/users/{id:[0-9]+}",
			},
			route:    "/api/users/abc",
			expected: false,
		},
	}

	config := &Config{}
	_ = config.processUnifiedConfig()

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := matchesEndpointPattern(tc.endpoint, tc.route, config)

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

func TestIsMuxPattern(t *testing.T) {
	testCases := []struct {
		desc     string
		pattern  string
		expected bool
	}{
		{
			desc:     "detects mux pattern with single variable",
			pattern:  "/api/users/{id}",
			expected: true,
		},
		{
			desc:     "detects mux pattern with constraint",
			pattern:  "/api/users/{id:[0-9]+}",
			expected: true,
		},
		{
			desc:     "detects mux pattern multi-level",
			pattern:  "/api/{path:.*}",
			expected: true,
		},
		{
			desc:     "does not detect exact path",
			pattern:  "/api/users",
			expected: false,
		},
		{
			desc:     "does not detect wildcard",
			pattern:  "/api/*",
			expected: false,
		},
		{
			desc:     "does not detect regex",
			pattern:  "^/api/users/\\d+$",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := isMuxPattern(tc.pattern)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestMatchMuxPattern(t *testing.T) {
	router := mux.NewRouter()

	testCases := []struct {
		desc     string
		pattern  string
		method   string
		path     string
		expected bool
	}{
		{
			desc:     "matches single variable",
			pattern:  "/api/users/{id}",
			method:   "GET",
			path:     "/api/users/123",
			expected: true,
		},
		{
			desc:     "matches variable with constraint",
			pattern:  "/api/users/{id:[0-9]+}",
			method:   "GET",
			path:     "/api/users/123",
			expected: true,
		},
		{
			desc:     "does not match constraint violation",
			pattern:  "/api/users/{id:[0-9]+}",
			method:   "GET",
			path:     "/api/users/abc",
			expected: false,
		},
		{
			desc:     "matches multi-level pattern",
			pattern:  "/api/{path:.*}",
			method:   "GET",
			path:     "/api/users/123",
			expected: true,
		},
		{
			desc:     "matches middle variable",
			pattern:  "/api/{category}/posts",
			method:   "GET",
			path:     "/api/tech/posts",
			expected: true,
		},
		{
			desc:     "matches multiple variables",
			pattern:  "/api/{category}/posts/{id:[0-9]+}",
			method:   "GET",
			path:     "/api/tech/posts/123",
			expected: true,
		},
		{
			desc:     "does not match different path",
			pattern:  "/api/users/{id}",
			method:   "GET",
			path:     "/api/posts/123",
			expected: false,
		},
		{
			desc:     "returns false for nil router",
			pattern:  "/api/users/{id}",
			method:   "GET",
			path:     "/api/users/123",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var testRouter *mux.Router

			if tc.desc != "returns false for nil router" {
				testRouter = router
			}

			result := matchMuxPattern(tc.pattern, tc.method, tc.path, testRouter)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValidateMuxPattern(t *testing.T) {
	testCases := []struct {
		desc        string
		pattern     string
		expectError bool
	}{
		{
			desc:        "validates single variable",
			pattern:     "/api/users/{id}",
			expectError: false,
		},
		{
			desc:        "validates variable with constraint",
			pattern:     "/api/users/{id:[0-9]+}",
			expectError: false,
		},
		{
			desc:        "validates multi-level pattern",
			pattern:     "/api/{path:.*}",
			expectError: false,
		},
		{
			desc:        "validates non-pattern path",
			pattern:     "/api/users",
			expectError: false,
		},
		{
			desc:        "rejects unbalanced braces",
			pattern:     "/api/users/{id",
			expectError: true,
		},
		{
			desc:        "rejects unbalanced braces close",
			pattern:     "/api/users/id}",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := validateMuxPattern(tc.pattern)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
