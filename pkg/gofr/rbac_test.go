package gofr

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnableRBAC(t *testing.T) {
	testCases := []struct {
		desc          string
		provider      RBACProvider
		configFile    string
		setupFiles    func() (string, error)
		cleanupFiles  func(string)
		expectedLogs  []string
		expectedError bool
		middlewareSet bool
	}{
		{
			desc:       "nil provider should log error",
			provider:   nil,
			configFile: "",
			setupFiles: func() (string, error) {
				return "", nil
			},
			cleanupFiles:  func(string) {},
			expectedLogs:  []string{"RBAC provider is required"},
			expectedError: false,
			middlewareSet: false,
		},
		{
			desc:       "valid provider with custom config file",
			provider:   &mockRBACProvider{},
			configFile: "test_rbac.json",
			setupFiles: func() (string, error) {
				content := `{"roles":[{"name":"admin","permissions":["admin:read"]}],` +
					`"endpoints":[{"path":"/api","methods":["GET"],"requiredPermissions":["admin:read"]}]}`
				return createTestConfigFile("test_rbac.json", content)
			},
			cleanupFiles: func(path string) {
				os.Remove(path)
			},
			expectedLogs:  []string{"Loaded RBAC config"},
			expectedError: false,
			middlewareSet: true,
		},
		{
			desc:       "valid provider with default config path",
			provider:   &mockRBACProvider{},
			configFile: DefaultRBACConfig,
			setupFiles: func() (string, error) {
				content := `{"roles":[{"name":"viewer","permissions":["users:read"]}],"endpoints":[{"path":"/health","methods":["GET"],"public":true}]}`
				return createTestConfigFile("configs/rbac.json", content)
			},
			cleanupFiles: func(path string) {
				os.Remove(path)
				os.Remove("configs")
			},
			expectedLogs:  []string{"Loaded RBAC config"},
			expectedError: false,
			middlewareSet: true,
		},
		{
			desc:       "config file not found",
			provider:   &mockRBACProvider{},
			configFile: "nonexistent.json",
			setupFiles: func() (string, error) {
				return "", nil
			},
			cleanupFiles:  func(string) {},
			expectedLogs:  []string{"RBAC config file not found"},
			expectedError: false,
			middlewareSet: false,
		},
		{
			desc:       "invalid config file format",
			provider:   &mockRBACProvider{},
			configFile: "invalid.json",
			setupFiles: func() (string, error) {
				content := `invalid json content{`
				return createTestConfigFile("invalid.json", content)
			},
			cleanupFiles: func(path string) {
				os.Remove(path)
			},
			expectedLogs:  []string{"Failed to load RBAC config"},
			expectedError: false,
			middlewareSet: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			filePath, err := tc.setupFiles()
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

			defer tc.cleanupFiles(filePath)

			app := New()
			app.EnableRBAC(tc.provider, tc.configFile)

			require.Equal(t, tc.middlewareSet, app.httpServer != nil && app.httpServer.router != nil,
				"TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestResolveRBACConfigPath(t *testing.T) {
	testCases := []struct {
		desc         string
		configFile   string
		setupFiles   func() ([]string, error)
		cleanupFiles func([]string)
		expectedPath string
	}{
		{
			desc:       "custom path provided",
			configFile: "custom.json",
			setupFiles: func() ([]string, error) {
				content := `{}`
				path, err := createTestConfigFile("custom.json", content)
				return []string{path}, err
			},
			cleanupFiles: func(paths []string) {
				for _, p := range paths {
					os.Remove(p)
				}
			},
			expectedPath: "custom.json",
		},
		{
			desc:       "empty path uses default json",
			configFile: "",
			setupFiles: func() ([]string, error) {
				content := `{}`
				path, err := createTestConfigFile("configs/rbac.json", content)
				return []string{path}, err
			},
			cleanupFiles: func(paths []string) {
				for _, p := range paths {
					os.Remove(p)
				}
				os.Remove("configs")
			},
			expectedPath: "configs/rbac.json",
		},
		{
			desc:       "empty path uses default yaml when json not found",
			configFile: "",
			setupFiles: func() ([]string, error) {
				content := `roles: []`
				path, err := createTestConfigFile("configs/rbac.yaml", content)
				return []string{path}, err
			},
			cleanupFiles: func(paths []string) {
				for _, p := range paths {
					os.Remove(p)
				}
				os.Remove("configs")
			},
			expectedPath: "configs/rbac.yaml",
		},
		{
			desc:       "empty path uses default yml when json and yaml not found",
			configFile: "",
			setupFiles: func() ([]string, error) {
				content := `roles: []`
				path, err := createTestConfigFile("configs/rbac.yml", content)
				return []string{path}, err
			},
			cleanupFiles: func(paths []string) {
				for _, p := range paths {
					os.Remove(p)
				}
				os.Remove("configs")
			},
			expectedPath: "configs/rbac.yml",
		},
		{
			desc:       "empty path returns empty when no defaults found",
			configFile: "",
			setupFiles: func() ([]string, error) {
				return []string{}, nil
			},
			cleanupFiles: func([]string) {},
			expectedPath: "",
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			paths, err := tc.setupFiles()
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

			defer tc.cleanupFiles(paths)

			result := resolveRBACConfigPath(tc.configFile)

			assert.Equal(t, tc.expectedPath, result, "TEST[%d], Failed.\n%s", i, tc.desc)
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

// mockRBACProvider is a mock implementation of RBACProvider for testing.
// This avoids import cycle by not importing rbac package.
type mockRBACProvider struct {
	config       any
	loadErr      error
	middlewareFn func(http.Handler) http.Handler
}

func (m *mockRBACProvider) LoadPermissions(file string) (any, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	// For testing, we'll use a simple struct that satisfies the interface
	// In real usage, this would be *rbac.Config
	return map[string]any{"file": file}, nil
}

func (m *mockRBACProvider) GetMiddleware(config any) func(http.Handler) http.Handler {
	m.config = config
	if m.middlewareFn != nil {
		return m.middlewareFn
	}

	return func(handler http.Handler) http.Handler {
		return handler
	}
}

func TestApp_EnableRBAC_Integration(t *testing.T) {
	testCases := []struct {
		desc          string
		configContent string
		provider      RBACProvider
		configFile    string
		expectError   bool
	}{
		{
			desc: "valid config with roles and endpoints",
			configContent: `{
				"roles": [
					{"name": "admin", "permissions": ["*:*"]},
					{"name": "viewer", "permissions": ["users:read"]}
				],
				"endpoints": [
					{"path": "/health", "methods": ["GET"], "public": true},
					{"path": "/api/users", "methods": ["GET"], "requiredPermission": "users:read"}
				]
			}`,
			provider:    &mockRBACProvider{},
			configFile:  "test_integration.json",
			expectError: false,
		},
		{
			desc: "config with role inheritance",
			configContent: `{
				"roles": [
					{"name": "viewer", "permissions": ["users:read"]},
					{"name": "editor", "permissions": ["users:write"], "inheritsFrom": ["viewer"]}
				],
				"endpoints": [
					{"path": "/api/users", "methods": ["GET"], "requiredPermission": "users:read"}
				]
			}`,
			provider:    &mockRBACProvider{},
			configFile:  "test_inheritance.json",
			expectError: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			path, err := createTestConfigFile(tc.configFile, tc.configContent)
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

			defer os.Remove(path)

			app := New()
			app.EnableRBAC(tc.provider, tc.configFile)

			require.NotNil(t, app.httpServer, "TEST[%d], Failed.\n%s", i, tc.desc)
			require.NotNil(t, app.httpServer.router, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
