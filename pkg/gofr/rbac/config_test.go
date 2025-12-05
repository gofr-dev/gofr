package rbac

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr/logging"
)

type mockLoggerForConfig struct {
	logs []string
}

func (m *mockLoggerForConfig) Debug(_ ...any)             { m.logs = append(m.logs, "DEBUG") }
func (m *mockLoggerForConfig) Debugf(_ string, _ ...any)  { m.logs = append(m.logs, "DEBUGF") }
func (m *mockLoggerForConfig) Log(_ ...any)               { m.logs = append(m.logs, "LOG") }
func (m *mockLoggerForConfig) Logf(_ string, _ ...any)    { m.logs = append(m.logs, "LOGF") }
func (m *mockLoggerForConfig) Info(_ ...any)              { m.logs = append(m.logs, "INFO") }
func (m *mockLoggerForConfig) Infof(_ string, _ ...any)   { m.logs = append(m.logs, "INFOF") }
func (m *mockLoggerForConfig) Notice(_ ...any)            { m.logs = append(m.logs, "NOTICE") }
func (m *mockLoggerForConfig) Noticef(_ string, _ ...any) { m.logs = append(m.logs, "NOTICEF") }
func (m *mockLoggerForConfig) Error(_ ...any)             { m.logs = append(m.logs, "ERROR") }
func (m *mockLoggerForConfig) Errorf(_ string, _ ...any)  { m.logs = append(m.logs, "ERRORF") }
func (m *mockLoggerForConfig) Warn(_ ...any)              { m.logs = append(m.logs, "WARN") }
func (m *mockLoggerForConfig) Warnf(_ string, _ ...any)   { m.logs = append(m.logs, "WARNF") }
func (m *mockLoggerForConfig) Fatal(_ ...any)             { m.logs = append(m.logs, "FATAL") }
func (m *mockLoggerForConfig) Fatalf(_ string, _ ...any)  { m.logs = append(m.logs, "FATALF") }
func (*mockLoggerForConfig) ChangeLevel(logging.Level)    {}

func TestLoadPermissions_ValidConfigs(t *testing.T) {
	t.Run("loads valid json config", func(t *testing.T) {
		fileContent := `{
			"roles": [{"name": "admin", "permissions": ["admin:read", "admin:write"]}],
			"endpoints": [{"path": "/api", "methods": ["GET"], "requiredPermissions": ["admin:read"]}]
		}`
		path, err := createTestConfigFile("test_config.json", fileContent)
		require.NoError(t, err)

		defer os.Remove(path)

		config, err := LoadPermissions("test_config.json")
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.NotNil(t, config.rolePermissionsMap)
	})

	t.Run("loads valid yaml config", func(t *testing.T) {
		fileContent := `roles:
  - name: admin
    permissions: ["admin:read", "admin:write"]
endpoints:
  - path: /api
    methods: ["GET"]
    requiredPermissions: ["admin:read"]`
		path, err := createTestConfigFile("test_config.yaml", fileContent)
		require.NoError(t, err)

		defer os.Remove(path)

		config, err := LoadPermissions("test_config.yaml")
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.NotNil(t, config.rolePermissionsMap)
	})

	t.Run("loads valid yml config", func(t *testing.T) {
		fileContent := `roles:
  - name: viewer
    permissions: ["users:read"]`
		path, err := createTestConfigFile("test_config.yml", fileContent)
		require.NoError(t, err)

		defer os.Remove(path)

		config, err := LoadPermissions("test_config.yml")
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.NotNil(t, config.rolePermissionsMap)
	})
}

func TestLoadPermissions_ErrorCases(t *testing.T) {
	t.Run("returns error for non-existent file", func(t *testing.T) {
		config, err := LoadPermissions("nonexistent.json")
		require.Error(t, err)
		require.Nil(t, config)
	})

	t.Run("returns error for invalid json", func(t *testing.T) {
		path, err := createTestConfigFile("test_invalid.json", `invalid json{`)
		require.NoError(t, err)

		defer os.Remove(path)

		config, err := LoadPermissions("test_invalid.json")
		require.Error(t, err)
		require.Nil(t, config)
	})

	t.Run("returns error for invalid yaml", func(t *testing.T) {
		path, err := createTestConfigFile("test_invalid.yaml", `invalid: yaml: [`)
		require.NoError(t, err)

		defer os.Remove(path)

		config, err := LoadPermissions("test_invalid.yaml")
		require.Error(t, err)
		require.Nil(t, config)
	})

	t.Run("returns error for unsupported format", func(t *testing.T) {
		path, err := createTestConfigFile("test.txt", `some content`)
		require.NoError(t, err)

		defer os.Remove(path)

		config, err := LoadPermissions("test.txt")
		require.Error(t, err)
		require.Nil(t, config)
	})

	t.Run("returns error for endpoint without requiredPermissions", func(t *testing.T) {
		fileContent := `{
			"roles": [{"name": "admin", "permissions": ["*:*"]}],
			"endpoints": [{"path": "/api", "methods": ["GET"]}]
		}`
		path, err := createTestConfigFile("test_missing_perm.json", fileContent)
		require.NoError(t, err)

		defer os.Remove(path)

		config, err := LoadPermissions("test_missing_perm.json")
		require.Error(t, err)
		require.Nil(t, config)
	})
}

func TestConfig_SetLogger(t *testing.T) {
	testCases := []struct {
		desc      string
		logger    any
		expectSet bool
	}{
		{
			desc:      "sets logger when logger implements Logger interface",
			logger:    &mockLoggerForConfig{},
			expectSet: true,
		},
		{
			desc:      "does not set logger when type mismatch",
			logger:    "not a logger",
			expectSet: false,
		},
		{
			desc:      "does not set logger when nil",
			logger:    nil,
			expectSet: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			config := &Config{}
			config.SetLogger(tc.logger)

			if tc.expectSet {
				require.NotNil(t, config.Logger, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Equal(t, tc.logger, config.Logger, "TEST[%d], Failed.\n%s", i, tc.desc)

				return
			}

			assert.Nil(t, config.Logger, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestConfig_GetRolePermissions(t *testing.T) {
	testCases := []struct {
		desc          string
		config        *Config
		role          string
		expectedPerms []string
	}{
		{
			desc: "returns permissions for existing role",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"*:*"}},
					{Name: "viewer", Permissions: []string{"users:read"}},
				},
			},
			role:          "admin",
			expectedPerms: []string{"*:*"},
		},
		{
			desc: "returns empty for non-existent role",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"*:*"}},
				},
			},
			role:          "nonexistent",
			expectedPerms: nil,
		},
		{
			desc: "returns permissions with inheritance",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "viewer", Permissions: []string{"users:read"}},
					{Name: "editor", Permissions: []string{"users:write"}, InheritsFrom: []string{"viewer"}},
				},
			},
			role:          "editor",
			expectedPerms: []string{"users:write", "users:read"},
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.config.processUnifiedConfig()
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

			result := tc.config.GetRolePermissions(tc.role)

			assert.Equal(t, tc.expectedPerms, result, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestConfig_GetEndpointPermission_ExactMatch(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	perm, isPublic := config.GetEndpointPermission("GET", "/api/users")
	assert.Equal(t, "users:read", perm)
	assert.False(t, isPublic)
}

func TestConfig_GetEndpointPermission_PublicEndpoint(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/health", Methods: []string{"GET"}, Public: true},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	perm, isPublic := config.GetEndpointPermission("GET", "/health")
	assert.Empty(t, perm)
	assert.True(t, isPublic)
}

func TestConfig_GetEndpointPermission_NotFound(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	perm, isPublic := config.GetEndpointPermission("POST", "/api/posts")
	assert.Empty(t, perm)
	assert.False(t, isPublic)
}

func TestConfig_GetEndpointPermission_WildcardPattern(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/api/*", Methods: []string{"GET"}, RequiredPermissions: []string{"api:read"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	perm, isPublic := config.GetEndpointPermission("GET", "/api/users")
	assert.Equal(t, "api:read", perm)
	assert.False(t, isPublic)
}

func TestConfig_GetEndpointPermission_RegexPattern(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Regex: "^/api/users/\\d+$", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	perm, isPublic := config.GetEndpointPermission("GET", "/api/users/123")
	assert.Equal(t, "users:read", perm)
	assert.False(t, isPublic)
}

func TestConfig_GetEndpointPermission_CaseInsensitive(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/api", Methods: []string{"get"}, RequiredPermissions: []string{"api:read"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	perm, isPublic := config.GetEndpointPermission("GET", "/api")
	assert.Equal(t, "api:read", perm)
	assert.False(t, isPublic)
}

func TestConfig_processUnifiedConfig(t *testing.T) {
	testCases := []struct {
		desc        string
		config      *Config
		expectError bool
	}{
		{
			desc: "processes config with roles and endpoints",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"*:*"}},
				},
				Endpoints: []EndpointMapping{
					{Path: "/api", Methods: []string{"GET"}, RequiredPermissions: []string{"admin:*"}},
				},
			},
			expectError: false,
		},
		{
			desc: "processes config with role inheritance",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "viewer", Permissions: []string{"users:read"}},
					{Name: "editor", Permissions: []string{"users:write"}, InheritsFrom: []string{"viewer"}},
				},
				Endpoints: []EndpointMapping{
					{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
				},
			},
			expectError: false,
		},
		{
			desc: "returns error for endpoint without requiredPermissions",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"*:*"}},
				},
				Endpoints: []EndpointMapping{
					{Path: "/api", Methods: []string{"GET"}},
				},
			},
			expectError: true,
		},
		{
			desc: "processes config with public endpoints",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"*:*"}},
				},
				Endpoints: []EndpointMapping{
					{Path: "/health", Methods: []string{"GET"}, Public: true},
				},
			},
			expectError: false,
		},
		{
			desc: "processes config with empty methods",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"*:*"}},
				},
				Endpoints: []EndpointMapping{
					{Path: "/api", Methods: []string{}, RequiredPermissions: []string{"admin:*"}},
				},
			},
			expectError: false,
		},
		{
			desc: "processes config with regex endpoints",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"*:*"}},
				},
				Endpoints: []EndpointMapping{
					{Regex: "^/api/users/\\d+$", Methods: []string{"GET"}, RequiredPermissions: []string{"admin:*"}},
				},
			},
			expectError: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.config.processUnifiedConfig()

			if tc.expectError {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.NotNil(t, tc.config.rolePermissionsMap, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.NotNil(t, tc.config.endpointPermissionMap, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestMatchesPathPattern(t *testing.T) {
	testCases := []struct {
		desc     string
		pattern  string
		path     string
		expected bool
	}{
		{
			desc:     "matches exact path",
			pattern:  "/api/users",
			path:     "/api/users",
			expected: true,
		},
		{
			desc:     "does not match different path",
			pattern:  "/api/users",
			path:     "/api/posts",
			expected: false,
		},
		{
			desc:     "matches wildcard pattern",
			pattern:  "/api/*",
			path:     "/api/users",
			expected: true,
		},
		{
			desc:     "matches wildcard pattern with exact prefix",
			pattern:  "/api/*",
			path:     "/api",
			expected: true,
		},
		{
			desc:     "does not match wildcard pattern with different prefix",
			pattern:  "/api/*",
			path:     "/v1/users",
			expected: false,
		},
		{
			desc:     "returns false for empty pattern",
			pattern:  "",
			path:     "/api/users",
			expected: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := matchesPathPattern(tc.pattern, tc.path)

			assert.Equal(t, tc.expected, result, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func createTestConfigFile(filename, content string) (string, error) {
	dir := filepath.Dir(filename)
	if dir != "." && dir != "" {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return "", err
		}
	}

	err := os.WriteFile(filename, []byte(content), 0600)

	return filename, err
}

func TestConfig_getEffectivePermissions(t *testing.T) {
	testCases := []struct {
		desc          string
		config        *Config
		roleName      string
		expectedPerms []string
	}{
		{
			desc: "returns permissions for role without inheritance",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "viewer", Permissions: []string{"users:read"}},
				},
			},
			roleName:      "viewer",
			expectedPerms: []string{"users:read"},
		},
		{
			desc: "returns permissions with single level inheritance",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "viewer", Permissions: []string{"users:read"}},
					{Name: "editor", Permissions: []string{"users:write"}, InheritsFrom: []string{"viewer"}},
				},
			},
			roleName:      "editor",
			expectedPerms: []string{"users:write", "users:read"},
		},
		{
			desc: "returns permissions with multi-level inheritance",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "viewer", Permissions: []string{"users:read"}},
					{Name: "editor", Permissions: []string{"users:write"}, InheritsFrom: []string{"viewer"}},
					{Name: "admin", Permissions: []string{"users:delete"}, InheritsFrom: []string{"editor"}},
				},
			},
			roleName:      "admin",
			expectedPerms: []string{"users:delete", "users:write", "users:read"},
		},
		{
			desc: "handles circular inheritance gracefully",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "role1", Permissions: []string{"perm1"}, InheritsFrom: []string{"role2"}},
					{Name: "role2", Permissions: []string{"perm2"}, InheritsFrom: []string{"role1"}},
				},
			},
			roleName:      "role1",
			expectedPerms: []string{"perm1", "perm2"},
		},
		{
			desc: "returns empty for non-existent role",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "viewer", Permissions: []string{"users:read"}},
				},
			},
			roleName:      "nonexistent",
			expectedPerms: nil,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := tc.config.getEffectivePermissions(tc.roleName)

			assert.Equal(t, tc.expectedPerms, result, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestExtractNestedClaim_Additional(t *testing.T) {
	testCases := []struct {
		desc        string
		claims      jwt.MapClaims
		path        string
		expected    any
		expectError bool
	}{
		{
			desc: "extracts claim with jwt.MapClaims nested",
			claims: jwt.MapClaims{
				"user": jwt.MapClaims{
					"role": "admin",
				},
			},
			path:        "user.role",
			expected:    "admin",
			expectError: false,
		},
		{
			desc: "returns error when intermediate value is not map",
			claims: jwt.MapClaims{
				"user": "not a map",
			},
			path:        "user.role",
			expected:    nil,
			expectError: true,
		},
		{
			desc: "extracts from mixed map types",
			claims: jwt.MapClaims{
				"level1": map[string]any{
					"level2": jwt.MapClaims{
						"value": "test",
					},
				},
			},
			path:        "level1.level2.value",
			expected:    "test",
			expectError: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result, err := extractNestedClaim(tc.claims, tc.path)

			if tc.expectError {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Nil(t, result, "TEST[%d], Failed.\n%s", i, tc.desc)

				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.expected, result, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestExtractArrayClaim_Additional(t *testing.T) {
	testCases := []struct {
		desc        string
		claims      jwt.MapClaims
		path        string
		expected    any
		expectError bool
	}{
		{
			desc: "extracts from array with valid index",
			claims: jwt.MapClaims{
				"roles": []any{"admin", "user", "guest"},
			},
			path:        "roles[2]",
			expected:    "guest",
			expectError: false,
		},
		{
			desc: "returns error for invalid array notation format",
			claims: jwt.MapClaims{
				"roles": []any{"admin"},
			},
			path:        "roles]0[",
			expected:    nil,
			expectError: true,
		},
		{
			desc: "returns error for non-numeric index",
			claims: jwt.MapClaims{
				"roles": []any{"admin"},
			},
			path:        "roles[abc]",
			expected:    nil,
			expectError: true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			idx := 0

			for j, c := range tc.path {
				if c == '[' {
					idx = j
					break
				}
			}

			result, err := extractArrayClaim(tc.claims, tc.path, idx)

			if tc.expectError {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Nil(t, result, "TEST[%d], Failed.\n%s", i, tc.desc)

				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.expected, result, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
