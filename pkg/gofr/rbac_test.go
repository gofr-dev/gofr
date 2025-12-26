package gofr

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

func TestEnableRBAC(t *testing.T) {
	testCases := []struct {
		desc          string
		configPath    string
		setupFiles    func() (string, error)
		cleanupFiles  func(string)
		expectedLogs  []string
		expectedError bool
		middlewareSet bool
	}{
		{
			desc:       "valid config with custom config file",
			configPath: "test_rbac.json",
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
			desc:       "config file not found",
			configPath: "nonexistent.json",
			setupFiles: func() (string, error) {
				return "", nil
			},
			cleanupFiles:  func(string) {},
			expectedLogs:  []string{"Failed to load RBAC config"},
			expectedError: false,
			middlewareSet: false,
		},
		{
			desc:       "invalid config file format",
			configPath: "invalid.json",
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
			// Set up free ports for HTTP, Metrics, and gRPC
			_ = testutil.NewServerConfigs(t)

			filePath, err := tc.setupFiles()
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

			defer tc.cleanupFiles(filePath)

			app := New()
			app.EnableRBAC(tc.configPath)

			// Check that httpServer and router exist
			require.NotNil(t, app.httpServer, "TEST[%d], Failed.\n%s", i, tc.desc)
			require.NotNil(t, app.httpServer.router, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestEnableRBAC_WithDefaultConfigPath(t *testing.T) {
	testCases := []struct {
		desc          string
		setupFiles    func() (string, error)
		cleanupFiles  func(string)
		expectedLogs  []string
		expectedError bool
		middlewareSet bool
	}{
		{
			desc: "valid config with default config path (no args)",
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
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Set up free ports for HTTP, Metrics, and gRPC
			_ = testutil.NewServerConfigs(t)

			filePath, err := tc.setupFiles()
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

			defer tc.cleanupFiles(filePath)

			app := New()
			app.EnableRBAC()

			// Check that httpServer and router exist
			require.NotNil(t, app.httpServer, "TEST[%d], Failed.\n%s", i, tc.desc)
			require.NotNil(t, app.httpServer.router, "TEST[%d], Failed.\n%s", i, tc.desc)
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

func TestApp_EnableRBAC_Integration(t *testing.T) {
	testCases := []struct {
		desc          string
		configContent string
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
					{"path": "/api/users", "methods": ["GET"], "requiredPermissions": ["users:read"]}
				]
			}`,
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
					{"path": "/api/users", "methods": ["GET"], "requiredPermissions": ["users:read"]}
				]
			}`,
			configFile:  "test_inheritance.json",
			expectError: false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Set up free ports for HTTP, Metrics, and gRPC
			_ = testutil.NewServerConfigs(t)

			path, err := createTestConfigFile(tc.configFile, tc.configContent)
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

			defer os.Remove(path)

			app := New()
			app.EnableRBAC(tc.configFile)

			require.NotNil(t, app.httpServer, "TEST[%d], Failed.\n%s", i, tc.desc)
			require.NotNil(t, app.httpServer.router, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
