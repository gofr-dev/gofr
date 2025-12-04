package rbac

import (
	"github.com/golang-jwt/jwt/v5"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLoggerForConfig struct {
	logs []string
}

func (m *mockLoggerForConfig) Debug(args ...any)                 { m.logs = append(m.logs, "DEBUG") }
func (m *mockLoggerForConfig) Debugf(format string, args ...any) { m.logs = append(m.logs, "DEBUGF") }
func (m *mockLoggerForConfig) Info(args ...any)                  { m.logs = append(m.logs, "INFO") }
func (m *mockLoggerForConfig) Infof(format string, args ...any)  { m.logs = append(m.logs, "INFOF") }
func (m *mockLoggerForConfig) Error(args ...any)                 { m.logs = append(m.logs, "ERROR") }
func (m *mockLoggerForConfig) Errorf(format string, args ...any) { m.logs = append(m.logs, "ERRORF") }
func (m *mockLoggerForConfig) Warn(args ...any)                  { m.logs = append(m.logs, "WARN") }
func (m *mockLoggerForConfig) Warnf(format string, args ...any)  { m.logs = append(m.logs, "WARNF") }

type mockHotReloadSourceForConfig struct {
	data []byte
	err  error
}

func (m *mockHotReloadSourceForConfig) FetchConfig() ([]byte, error) {
	return m.data, m.err
}

func TestLoadPermissions(t *testing.T) {
	testCases := []struct {
		desc         string
		fileContent  string
		fileName     string
		expectError  bool
		expectConfig bool
	}{
		{
			desc: "loads valid json config",
			fileContent: `{
				"roles": [{"name": "admin", "permissions": ["*:*"]}],
				"endpoints": [{"path": "/api", "methods": ["GET"], "requiredPermission": "admin:*"}]
			}`,
			fileName:     "test_config.json",
			expectError:  false,
			expectConfig: true,
		},
		{
			desc: "loads valid yaml config",
			fileContent: `roles:
  - name: admin
    permissions: ["*:*"]
endpoints:
  - path: /api
    methods: ["GET"]
    requiredPermission: admin:*`,
			fileName:     "test_config.yaml",
			expectError:  false,
			expectConfig: true,
		},
		{
			desc: "loads valid yml config",
			fileContent: `roles:
  - name: viewer
    permissions: ["users:read"]`,
			fileName:     "test_config.yml",
			expectError:  false,
			expectConfig: true,
		},
		{
			desc:         "returns error for non-existent file",
			fileContent:  "",
			fileName:     "nonexistent.json",
			expectError:  true,
			expectConfig: false,
		},
		{
			desc:         "returns error for invalid json",
			fileContent:  `invalid json{`,
			fileName:     "test_invalid.json",
			expectError:  true,
			expectConfig: false,
		},
		{
			desc:         "returns error for invalid yaml",
			fileContent:  `invalid: yaml: [`,
			fileName:     "test_invalid.yaml",
			expectError:  true,
			expectConfig: false,
		},
		{
			desc:         "returns error for unsupported format",
			fileContent:  `some content`,
			fileName:     "test.txt",
			expectError:  true,
			expectConfig: false,
		},
		{
			desc: "returns error for endpoint without requiredPermission",
			fileContent: `{
				"roles": [{"name": "admin", "permissions": ["*:*"]}],
				"endpoints": [{"path": "/api", "methods": ["GET"]}]
			}`,
			fileName:     "test_missing_perm.json",
			expectError:  true,
			expectConfig: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var filePath string

			if tc.fileContent != "" {
				path, err := createTestConfigFile(tc.fileName, tc.fileContent)
				require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				filePath = path
				defer os.Remove(filePath)
			}

			config, err := LoadPermissions(tc.fileName)

			if tc.expectError {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				require.Nil(t, config, "TEST[%d], Failed.\n%s", i, tc.desc)
				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			require.NotNil(t, config, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.NotNil(t, config.rolePermissionsMap, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
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

func TestConfig_GetEndpointPermission(t *testing.T) {
	testCases := []struct {
		desc           string
		config         *Config
		method         string
		path           string
		expectedPerm   string
		expectedPublic bool
		expectedFound  bool
	}{
		{
			desc: "returns permission for exact match",
			config: &Config{
				Endpoints: []EndpointMapping{
					{Path: "/api/users", Methods: []string{"GET"}, RequiredPermission: "users:read"},
				},
			},
			method:         "GET",
			path:           "/api/users",
			expectedPerm:   "users:read",
			expectedPublic: false,
			expectedFound:  true,
		},
		{
			desc: "returns public for public endpoint",
			config: &Config{
				Endpoints: []EndpointMapping{
					{Path: "/health", Methods: []string{"GET"}, Public: true},
				},
			},
			method:         "GET",
			path:           "/health",
			expectedPerm:   "",
			expectedPublic: true,
			expectedFound:  true,
		},
		{
			desc: "returns empty for non-existent endpoint",
			config: &Config{
				Endpoints: []EndpointMapping{
					{Path: "/api/users", Methods: []string{"GET"}, RequiredPermission: "users:read"},
				},
			},
			method:         "POST",
			path:           "/api/posts",
			expectedPerm:   "",
			expectedPublic: false,
			expectedFound:  false,
		},
		{
			desc: "matches wildcard pattern",
			config: &Config{
				Endpoints: []EndpointMapping{
					{Path: "/api/*", Methods: []string{"GET"}, RequiredPermission: "api:read"},
				},
			},
			method:         "GET",
			path:           "/api/users",
			expectedPerm:   "api:read",
			expectedPublic: false,
			expectedFound:  true,
		},
		{
			desc: "matches regex pattern",
			config: &Config{
				Endpoints: []EndpointMapping{
					{Regex: "^/api/users/\\d+$", Methods: []string{"GET"}, RequiredPermission: "users:read"},
				},
			},
			method:         "GET",
			path:           "/api/users/123",
			expectedPerm:   "users:read",
			expectedPublic: false,
			expectedFound:  true,
		},
		{
			desc: "does not match * method in GetEndpointPermission (handled by matchEndpoint)",
			config: &Config{
				Endpoints: []EndpointMapping{
					{Path: "/api", Methods: []string{"*"}, RequiredPermission: "api:*"},
				},
			},
			method:         "GET",
			path:           "/api",
			expectedPerm:   "",
			expectedPublic: false,
			expectedFound:  false,
		},
		{
			desc: "matches method case-insensitive",
			config: &Config{
				Endpoints: []EndpointMapping{
					{Path: "/api", Methods: []string{"get"}, RequiredPermission: "api:read"},
				},
			},
			method:         "GET",
			path:           "/api",
			expectedPerm:   "api:read",
			expectedPublic: false,
			expectedFound:  true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.config.processUnifiedConfig()
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

			perm, isPublic := tc.config.GetEndpointPermission(tc.method, tc.path)

			assert.Equal(t, tc.expectedPerm, perm, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.expectedPublic, isPublic, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
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
					{Path: "/api", Methods: []string{"GET"}, RequiredPermission: "admin:*"},
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
					{Path: "/api/users", Methods: []string{"GET"}, RequiredPermission: "users:read"},
				},
			},
			expectError: false,
		},
		{
			desc: "returns error for endpoint without requiredPermission",
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
					{Path: "/api", Methods: []string{}, RequiredPermission: "admin:*"},
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
					{Regex: "^/api/users/\\d+$", Methods: []string{"GET"}, RequiredPermission: "admin:*"},
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

func TestConfig_StartHotReload(t *testing.T) {
	testCases := []struct {
		desc          string
		config        *Config
		expectStarted bool
	}{
		{
			desc: "does not start when hot reload disabled",
			config: &Config{
				HotReloadConfig: &HotReloadConfig{
					Enabled: false,
				},
			},
			expectStarted: false,
		},
		{
			desc: "does not start when hot reload config is nil",
			config: &Config{
				HotReloadConfig: nil,
			},
			expectStarted: false,
		},
		{
			desc: "does not start when source is nil",
			config: &Config{
				HotReloadConfig: &HotReloadConfig{
					Enabled: true,
					Source:  nil,
				},
				Logger: &mockLoggerForConfig{},
			},
			expectStarted: false,
		},
		{
			desc: "starts hot reload when enabled with source",
			config: &Config{
				HotReloadConfig: &HotReloadConfig{
					Enabled:         true,
					IntervalSeconds: 1,
					Source: &mockHotReloadSourceForConfig{
						data: []byte(`{"roles":[],"endpoints":[]}`),
					},
				},
				Logger: &mockLoggerForConfig{},
			},
			expectStarted: true,
		},
		{
			desc: "uses default interval when interval is zero",
			config: &Config{
				HotReloadConfig: &HotReloadConfig{
					Enabled:         true,
					IntervalSeconds: 0,
					Source: &mockHotReloadSourceForConfig{
						data: []byte(`{"roles":[],"endpoints":[]}`),
					},
				},
				Logger: &mockLoggerForConfig{},
			},
			expectStarted: true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			tc.config.processUnifiedConfig()
			tc.config.StartHotReload()

			time.Sleep(100 * time.Millisecond)

			if tc.expectStarted {
				require.NotNil(t, tc.config.HotReloadConfig.Source, "TEST[%d], Failed.\n%s", i, tc.desc)
			}
		})
	}
}

func TestConfig_reloadConfig(t *testing.T) {
	testCases := []struct {
		desc        string
		source      *mockHotReloadSourceForConfig
		expectError bool
	}{
		{
			desc: "reloads config successfully",
			source: &mockHotReloadSourceForConfig{
				data: []byte(`{
					"roles": [{"name": "admin", "permissions": ["*:*"]}],
					"endpoints": [{"path": "/api", "methods": ["GET"], "requiredPermission": "admin:*"}]
				}`),
			},
			expectError: false,
		},
		{
			desc: "returns error when source fetch fails",
			source: &mockHotReloadSourceForConfig{
				err: assert.AnError,
			},
			expectError: true,
		},
		{
			desc: "returns error for invalid json",
			source: &mockHotReloadSourceForConfig{
				data: []byte(`invalid json{`),
			},
			expectError: true,
		},
		{
			desc: "reloads config with yaml format",
			source: &mockHotReloadSourceForConfig{
				data: []byte(`roles:
  - name: admin
    permissions: ["*:*"]`),
			},
			expectError: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			config := &Config{
				Roles: []RoleDefinition{
					{Name: "viewer", Permissions: []string{"users:read"}},
				},
				Endpoints: []EndpointMapping{
					{Path: "/api", Methods: []string{"GET"}, RequiredPermission: "users:read"},
				},
				HotReloadConfig: &HotReloadConfig{
					Source: tc.source,
				},
				Logger: &mockLoggerForConfig{},
			}

			err := config.processUnifiedConfig()
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

			err = config.reloadConfig()

			if tc.expectError {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
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

	err := os.WriteFile(filename, []byte(content), 0644)
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

func TestConfig_hotReloadLoop(t *testing.T) {
	testCases := []struct {
		desc         string
		config       *Config
		source       *mockHotReloadSourceForConfig
		expectReload bool
	}{
		{
			desc: "reloads config successfully in loop",
			config: &Config{
				HotReloadConfig: &HotReloadConfig{
					Enabled:         true,
					IntervalSeconds: 1,
				},
				Logger: &mockLoggerForConfig{logs: []string{}},
			},
			source: &mockHotReloadSourceForConfig{
				data: []byte(`{"roles":[],"endpoints":[]}`),
			},
			expectReload: true,
		},
		{
			desc: "handles reload errors in loop",
			config: &Config{
				HotReloadConfig: &HotReloadConfig{
					Enabled:         true,
					IntervalSeconds: 1,
				},
				Logger: &mockLoggerForConfig{logs: []string{}},
			},
			source: &mockHotReloadSourceForConfig{
				err: assert.AnError,
			},
			expectReload: true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			tc.config.HotReloadConfig.Source = tc.source
			tc.config.processUnifiedConfig()

			go tc.config.hotReloadLoop(100 * time.Millisecond)

			time.Sleep(250 * time.Millisecond)

			if tc.expectReload {
				mockLog := tc.config.Logger.(*mockLoggerForConfig)
				assert.Greater(t, len(mockLog.logs), 0, "TEST[%d], Failed.\n%s", i, tc.desc)
			}
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
