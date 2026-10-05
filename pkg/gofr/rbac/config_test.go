package rbac

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/container"
)

func TestLoadPermissions_ValidConfigs(t *testing.T) {
	t.Run("loads valid json config", func(t *testing.T) {
		fileContent := `{
			"roles": [{"name": "admin", "permissions": ["admin:read", "admin:write"]}],
			"endpoints": [{"path": "/api", "methods": ["GET"], "requiredPermissions": ["admin:read"]}]
		}`
		path, err := createTestConfigFile("test_config.json", fileContent)
		require.NoError(t, err)

		defer os.Remove(path)

		config, err := LoadPermissions("test_config.json", nil, nil, nil)
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

		config, err := LoadPermissions("test_config.yaml", nil, nil, nil)
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

		config, err := LoadPermissions("test_config.yml", nil, nil, nil)
		require.NoError(t, err)
		require.NotNil(t, config)
		assert.NotNil(t, config.rolePermissionsMap)
	})
}

func TestLoadPermissions_ErrorCases(t *testing.T) {
	t.Run("returns error for non-existent file", func(t *testing.T) {
		config, err := LoadPermissions("nonexistent.json", nil, nil, nil)
		require.Error(t, err)
		require.Nil(t, config)
	})

	t.Run("returns error for invalid json", func(t *testing.T) {
		path, err := createTestConfigFile("test_invalid.json", `invalid json{`)
		require.NoError(t, err)

		defer os.Remove(path)

		config, err := LoadPermissions("test_invalid.json", nil, nil, nil)
		require.Error(t, err)
		require.Nil(t, config)
	})

	t.Run("returns error for invalid yaml", func(t *testing.T) {
		path, err := createTestConfigFile("test_invalid.yaml", `invalid: yaml: [`)
		require.NoError(t, err)

		defer os.Remove(path)

		config, err := LoadPermissions("test_invalid.yaml", nil, nil, nil)
		require.Error(t, err)
		require.Nil(t, config)
	})

	t.Run("returns error for unsupported format", func(t *testing.T) {
		path, err := createTestConfigFile("test.txt", `some content`)
		require.NoError(t, err)

		defer os.Remove(path)

		config, err := LoadPermissions("test.txt", nil, nil, nil)
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

		config, err := LoadPermissions("test_missing_perm.json", nil, nil, nil)
		require.Error(t, err)
		require.Nil(t, config)
	})
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

	perms, isPublic := config.GetEndpointPermission("GET", "/api/users")
	assert.Equal(t, []string{"users:read"}, perms)
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

	perms, isPublic := config.GetEndpointPermission("GET", "/health")
	assert.Nil(t, perms)
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

	perms, isPublic := config.GetEndpointPermission("POST", "/api/posts")
	assert.Nil(t, perms)
	assert.False(t, isPublic)
}

func TestConfig_GetEndpointPermission_MuxPattern(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/api/{resource}", Methods: []string{"GET"}, RequiredPermissions: []string{"api:read"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	perms, isPublic := config.GetEndpointPermission("GET", "/api/users")
	assert.Equal(t, []string{"api:read"}, perms)
	assert.False(t, isPublic)
}

func TestConfig_GetEndpointPermission_MuxPatternWithConstraint(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{Path: "/api/users/{id:[0-9]+}", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	perms, isPublic := config.GetEndpointPermission("GET", "/api/users/123")
	assert.Equal(t, []string{"users:read"}, perms)
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

	perms, isPublic := config.GetEndpointPermission("GET", "/api")
	assert.Equal(t, []string{"api:read"}, perms)
	assert.False(t, isPublic)
}

func TestConfig_GetEndpointPermission_MultiplePermissions(t *testing.T) {
	config := &Config{
		Endpoints: []EndpointMapping{
			{
				Path:                "/api/users",
				Methods:             []string{"GET"},
				RequiredPermissions: []string{"users:read", "users:admin"},
			},
		},
	}
	err := config.processUnifiedConfig()
	require.NoError(t, err)

	perms, isPublic := config.GetEndpointPermission("GET", "/api/users")
	assert.Equal(t, []string{"users:read", "users:admin"}, perms)
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
					{Path: "/api/users/{id:[0-9]+}", Methods: []string{"GET"}, RequiredPermissions: []string{"admin:*"}},
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

func TestConfig_Resolve(t *testing.T) {
	t.Run("finds endpoint with mux pattern", func(t *testing.T) {
		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/api/{resource}", Methods: []string{"GET"}, RequiredPermissions: []string{"api:read"}},
			},
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		endpoint, isPublic := config.resolve("GET", "/api/users")
		assert.NotNil(t, endpoint)
		assert.Equal(t, "/api/{resource}", endpoint.Path)
		assert.False(t, isPublic)
	})

	t.Run("finds endpoint with mux pattern constraint", func(t *testing.T) {
		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/api/users/{id:[0-9]+}", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
			},
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		endpoint, isPublic := config.resolve("GET", "/api/users/123")
		assert.NotNil(t, endpoint)
		assert.False(t, isPublic)
	})

	t.Run("finds public endpoint with pattern", func(t *testing.T) {
		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/public/{path:.*}", Methods: []string{"GET"}, Public: true},
			},
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		endpoint, isPublic := config.resolve("GET", "/public/files")
		assert.NotNil(t, endpoint)
		assert.True(t, isPublic)
	})

	t.Run("returns nil when no pattern matches", func(t *testing.T) {
		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/api/{resource}", Methods: []string{"GET"}, RequiredPermissions: []string{"api:read"}},
			},
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		endpoint, isPublic := config.resolve("GET", "/other/path")
		assert.Nil(t, endpoint)
		assert.False(t, isPublic)
	})

	t.Run("returns nil when method doesn't match", func(t *testing.T) {
		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/api/{resource}", Methods: []string{"GET"}, RequiredPermissions: []string{"api:read"}},
			},
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		endpoint, isPublic := config.resolve("POST", "/api/users")
		assert.Nil(t, endpoint)
		assert.False(t, isPublic)
	})
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

func TestLoadPermissions_ClaimModes(t *testing.T) {
	const endpoints = `"endpoints": [{"path": "/orders", "methods": ["GET"], "requiredPermissions": ["orders:read"]}]`

	testCases := []struct {
		desc      string
		file      string
		content   string
		wantErr   error
		perms     string
		audience  []string
		startLine string
	}{
		{
			desc: "permissions mode from json", file: "rbac.json",
			content:  `{"permissionsClaimPath": "scope", "audience": ["orders-api"], ` + endpoints + `}`,
			perms:    "scope",
			audience: []string{"orders-api"}, startLine: "RBAC enabled: mode=permissions, claim=scope, audience=[orders-api]",
		},
		{
			desc: "permissions mode from yaml", file: "rbac.yaml",
			content: "permissionsClaimPath: scope\naudience: [orders-api, billing-api]\nendpoints:\n" +
				"  - path: /orders\n    methods: [GET]\n    requiredPermissions: [orders:read]\n",
			perms: "scope", audience: []string{"orders-api", "billing-api"},
			startLine: "RBAC enabled: mode=permissions, claim=scope, audience=[orders-api billing-api]",
		},
		{
			desc: "roles mode", file: "rbac.json",
			content:   `{"jwtClaimPath": "roles", "roles": [{"name": "a", "permissions": ["orders:read"]}], ` + endpoints + `}`,
			startLine: "RBAC enabled: mode=roles, claim=roles, audience=[]",
		},
		{
			desc: "header mode", file: "rbac.json",
			content:   `{"roleHeader": "X-User-Role", "roles": [{"name": "a", "permissions": ["orders:read"]}], ` + endpoints + `}`,
			startLine: "RBAC enabled: mode=header, claim=X-User-Role, audience=[]",
		},
		{
			desc: "both claim paths are rejected", file: "rbac.json",
			content: `{"jwtClaimPath": "roles", "permissionsClaimPath": "scope", "audience": ["x"], ` + endpoints + `}`,
			wantErr: errBothClaimPaths,
		},
		{
			desc: "permissions mode without audience is rejected", file: "rbac.json",
			content: `{"permissionsClaimPath": "scope", ` + endpoints + `}`,
			wantErr: errAudienceRequired,
		},
		{
			desc: "audience without a JWT claim path is rejected", file: "rbac.json",
			content: `{"roleHeader": "X-User-Role", "audience": ["x"], ` + endpoints + `}`,
			wantErr: errAudienceWithoutJWT,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.file)
			require.NoError(t, os.WriteFile(path, []byte(tc.content), 0600))

			ctrl := gomock.NewController(t)
			metrics := container.NewMockMetrics(ctrl)
			logger := &mockLogger{}

			if tc.wantErr == nil {
				metrics.EXPECT().NewCounter("rbac_role_extraction_failures", gomock.Any())
			}

			config, err := LoadPermissions(path, logger, metrics, nil)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr, "TEST[%d], Failed.\n%s", i, tc.desc)
				assert.Nil(t, config, "TEST[%d], Failed.\n%s", i, tc.desc)

				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.perms, config.PermissionsClaimPath, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.audience, config.Audience, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Contains(t, logger.infoLogs, tc.startLine, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
