package rbac

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfig_resolve_Cases covers the resolution cases that used to be exercised through the
// now-deleted matchEndpoint helper. They run against config.resolve, which is the single entry
// point the middleware and GetEndpointPermission both use.
func TestConfig_resolve_Cases(t *testing.T) {
	testCases := []struct {
		desc           string
		endpoints      []EndpointMapping
		method         string
		path           string
		expectedPath   string
		expectedPublic bool
	}{
		{
			desc:         "exact match",
			endpoints:    []EndpointMapping{{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}}},
			method:       http.MethodGet,
			path:         "/api/users",
			expectedPath: "/api/users",
		},
		{
			desc:           "public endpoint",
			endpoints:      []EndpointMapping{{Path: "/health", Methods: []string{"GET"}, Public: true}},
			method:         http.MethodGet,
			path:           "/health",
			expectedPath:   "/health",
			expectedPublic: true,
		},
		{
			desc:      "declared method does not cover the request method",
			endpoints: []EndpointMapping{{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}}},
			method:    http.MethodPost,
			path:      "/api/users",
		},
		{
			desc:         "wildcard method",
			endpoints:    []EndpointMapping{{Path: "/api", Methods: []string{"*"}, RequiredPermissions: []string{"api:read"}}},
			method:       http.MethodPost,
			path:         "/api",
			expectedPath: "/api",
		},
		{
			desc:         "omitted methods behaves as wildcard",
			endpoints:    []EndpointMapping{{Path: "/api", Methods: []string{}, RequiredPermissions: []string{"api:read"}}},
			method:       http.MethodPost,
			path:         "/api",
			expectedPath: "/api",
		},
		{
			desc:         "mux pattern path",
			endpoints:    []EndpointMapping{{Path: "/api/{resource}", Methods: []string{"GET"}, RequiredPermissions: []string{"api:read"}}},
			method:       http.MethodGet,
			path:         "/api/users",
			expectedPath: "/api/{resource}",
		},
		{
			desc:         "mux pattern with constraint",
			endpoints:    []EndpointMapping{{Path: "/api/users/{id:[0-9]+}", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}}},
			method:       http.MethodGet,
			path:         "/api/users/123",
			expectedPath: "/api/users/{id:[0-9]+}",
		},
		{
			desc:      "no configured endpoint matches",
			endpoints: []EndpointMapping{{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}}},
			method:    http.MethodGet,
			path:      "/api/posts",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			config := newTestConfig(t, tc.endpoints, nil)

			endpoint, isPublic := config.resolve(tc.method, tc.path)

			assert.Equal(t, tc.expectedPublic, isPublic)

			if tc.expectedPath == "" {
				assert.Nil(t, endpoint)
				return
			}

			require.NotNil(t, endpoint)
			assert.Equal(t, tc.expectedPath, endpoint.Path)
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

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			rule := endpointRule{
				pattern: tc.endpoint.Path,
				route:   compilePattern(tc.endpoint.Path),
			}

			result := rule.matchesPath(tc.route, newMatchContext(http.MethodGet, tc.route))

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

func TestCompilePattern(t *testing.T) {
	testCases := []struct {
		desc     string
		pattern  string
		path     string
		compiled bool
		expected bool
	}{
		{"matches single variable", "/api/users/{id}", "/api/users/123", true, true},
		{"matches variable with constraint", "/api/users/{id:[0-9]+}", "/api/users/123", true, true},
		{"does not match constraint violation", "/api/users/{id:[0-9]+}", "/api/users/abc", true, false},
		{"matches multi-level pattern", "/api/{path:.*}", "/api/users/123", true, true},
		{"matches middle variable", "/api/{category}/posts", "/api/tech/posts", true, true},
		{"matches multiple variables", "/api/{category}/posts/{id:[0-9]+}", "/api/tech/posts/123", true, true},
		{"does not match different path", "/api/users/{id}", "/api/posts/123", true, false},
		{"literal path is not compiled", "/api/users", "/api/users", false, false},
		{"empty path is not compiled", "", "/api/users", false, false},
		{"uncompilable pattern matches nothing", "/api/{id:[}", "/api/users", true, false},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			route := compilePattern(tc.pattern)

			if !tc.compiled {
				assert.Nil(t, route, "a literal path carries no compiled route")

				return
			}

			require.NotNil(t, route)
			assert.Equal(t, tc.expected, newMatchContext(http.MethodGet, tc.path).matches(route))
		})
	}
}

// TestCompilePattern_routeIsNotShared pins the fix for the unbounded router growth: compiling a
// pattern must not append to any router that outlives the call.
func TestCompilePattern_routeIsNotShared(t *testing.T) {
	const pattern = "/api/users/{id}"

	first, second := compilePattern(pattern), compilePattern(pattern)

	require.NotNil(t, first)
	require.NotNil(t, second)
	assert.NotSame(t, first, second, "each pattern must compile onto a router of its own")

	mc := newMatchContext(http.MethodGet, "/api/users/123")

	// Matching repeatedly must be a pure read - the same answer, every time.
	for range 3 {
		assert.True(t, mc.matches(first))
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

func TestResolveRBACConfigPath(t *testing.T) {
	t.Run("returns custom path when provided", func(t *testing.T) {
		customPath := "custom/path/rbac.json"
		result := ResolveRBACConfigPath(customPath)
		assert.Equal(t, customPath, result)
	})

	t.Run("returns default json path when file exists", func(t *testing.T) {
		// Create a temporary rbac.json file
		dir := "configs"
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)

		filePath := filepath.Join(dir, "rbac.json")
		err = os.WriteFile(filePath, []byte(`{"roles":[]}`), 0600)
		require.NoError(t, err)

		defer func() {
			os.Remove(filePath)
			os.Remove(dir)
		}()

		result := ResolveRBACConfigPath("")
		assert.Equal(t, filePath, result)
	})

	t.Run("returns default yaml path when json doesn't exist", func(t *testing.T) {
		// Create a temporary rbac.yaml file
		dir := "configs"
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)

		filePath := filepath.Join(dir, "rbac.yaml")
		err = os.WriteFile(filePath, []byte("roles: []"), 0600)
		require.NoError(t, err)

		defer func() {
			os.Remove(filePath)
			os.Remove(dir)
		}()

		result := ResolveRBACConfigPath("")
		assert.Equal(t, filePath, result)
	})

	t.Run("returns default yml path when json and yaml don't exist", func(t *testing.T) {
		// Create a temporary rbac.yml file
		dir := "configs"
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)

		filePath := filepath.Join(dir, "rbac.yml")
		err = os.WriteFile(filePath, []byte("roles: []"), 0600)
		require.NoError(t, err)

		defer func() {
			os.Remove(filePath)
			os.Remove(dir)
		}()

		result := ResolveRBACConfigPath("")
		assert.Equal(t, filePath, result)
	})

	t.Run("returns empty string when no default files exist", func(t *testing.T) {
		// Ensure configs directory doesn't exist or is empty
		dir := "configs"
		os.RemoveAll(dir)

		result := ResolveRBACConfigPath("")
		assert.Empty(t, result)
	})
}
