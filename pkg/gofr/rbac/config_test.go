package rbac

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestConfig_FindEndpointByPattern(t *testing.T) {
	t.Run("finds endpoint with mux pattern", func(t *testing.T) {
		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/api/{resource}", Methods: []string{"GET"}, RequiredPermissions: []string{"api:read"}},
			},
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		endpoint, isPublic := config.findEndpointByPattern("GET", "/api/users")
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

		endpoint, isPublic := config.findEndpointByPattern("GET", "/api/users/123")
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

		endpoint, isPublic := config.findEndpointByPattern("GET", "/public/files")
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

		endpoint, isPublic := config.findEndpointByPattern("GET", "/other/path")
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

		endpoint, isPublic := config.findEndpointByPattern("POST", "/api/users")
		assert.Nil(t, endpoint)
		assert.False(t, isPublic)
	})
}

func TestConfig_MatchesKey(t *testing.T) {
	t.Run("matches exact path", func(t *testing.T) {
		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
			},
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		result := config.matchesKey("GET:/api/users", "GET", "/api/users")
		assert.True(t, result)
	})

	t.Run("matches mux pattern", func(t *testing.T) {
		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/api/{resource}", Methods: []string{"GET"}, RequiredPermissions: []string{"api:read"}},
			},
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		result := config.matchesKey("GET:/api/{resource}", "GET", "/api/users")
		assert.True(t, result)
	})

	t.Run("matches mux pattern with constraint", func(t *testing.T) {
		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/api/users/{id:[0-9]+}", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
			},
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		result := config.matchesKey("GET:/api/users/{id:[0-9]+}", "GET", "/api/users/123")
		assert.True(t, result)
	})

	t.Run("matches mux pattern with constraint", func(t *testing.T) {
		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/test/{id:[0-9]+}", Methods: []string{"GET"}, RequiredPermissions: []string{"test:read"}},
			},
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		result := config.matchesKey("GET:/test/{id:[0-9]+}", "GET", "/test/456")
		assert.True(t, result)
	})

	t.Run("returns false when method doesn't match", func(t *testing.T) {
		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/api/users", Methods: []string{"GET"}, RequiredPermissions: []string{"users:read"}},
			},
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		result := config.matchesKey("GET:/api/users", "POST", "/api/users")
		assert.False(t, result)
	})

	t.Run("returns false for invalid mux pattern", func(t *testing.T) {
		config := &Config{
			Endpoints: []EndpointMapping{
				{Path: "/api/invalid{", Methods: []string{"GET"}, RequiredPermissions: []string{"test:read"}},
			},
		}
		err := config.processUnifiedConfig()
		require.NoError(t, err)

		result := config.matchesKey("GET:/api/invalid{", "GET", "/test")
		assert.False(t, result)
	})
}

func TestLoadPermissions_RegisterMetrics(t *testing.T) {
	t.Run("registers metrics when metrics is provided", func(t *testing.T) {
		mockMetrics := &configMockMetrics{
			histogramCreated: false,
			counterCreated:   false,
		}

		fileContent := `{
			"roles": [{"name": "admin", "permissions": ["admin:read"]}],
			"endpoints": [{"path": "/api", "methods": ["GET"], "requiredPermissions": ["admin:read"]}]
		}`
		path, err := createTestConfigFile("test_metrics.json", fileContent)
		require.NoError(t, err)

		defer os.Remove(path)

		_, err = LoadPermissions(path, nil, mockMetrics, nil)
		require.NoError(t, err)

		assert.True(t, mockMetrics.histogramCreated, "histogram should be created")
		assert.True(t, mockMetrics.counterCreated, "counter should be created")
	})
}

// configMockMetrics for config tests.
type configMockMetrics struct {
	histogramCreated bool
	counterCreated   bool
}

func (m *configMockMetrics) NewHistogram(_, _ string, _ ...float64) {
	m.histogramCreated = true
}

func (*configMockMetrics) RecordHistogram(_ context.Context, _ string, _ float64, _ ...string) {}
func (m *configMockMetrics) NewCounter(_, _ string) {
	m.counterCreated = true
}
func (*configMockMetrics) IncrementCounter(_ context.Context, _ string, _ ...string)              {}
func (*configMockMetrics) NewUpDownCounter(_, _ string)                                           {}
func (*configMockMetrics) NewGauge(_, _ string)                                                   {}
func (*configMockMetrics) DeltaUpDownCounter(_ context.Context, _ string, _ float64, _ ...string) {}
func (*configMockMetrics) SetGauge(_ string, _ float64, _ ...string)                              {}

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
