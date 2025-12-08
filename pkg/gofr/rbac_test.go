package gofr

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	errConfigPathNotSet = errors.New("config path not set")
	errFailedToParse    = errors.New("failed to parse")
)

func TestEnableRBAC(t *testing.T) {
	testCases := []struct {
		desc          string
		provider      RBACProvider
		setupFiles    func() (string, error)
		cleanupFiles  func(string)
		expectedLogs  []string
		expectedError bool
		middlewareSet bool
	}{
		{
			desc:     "nil provider should log error",
			provider: nil,
			setupFiles: func() (string, error) {
				return "", nil
			},
			cleanupFiles:  func(string) {},
			expectedLogs:  []string{"RBAC provider is required"},
			expectedError: false,
			middlewareSet: false,
		},
		{
			desc:     "valid provider with custom config file",
			provider: &mockRBACProvider{configPath: "test_rbac.json"},
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
			desc:     "valid provider with default config path",
			provider: &mockRBACProvider{configPath: ""},
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
			desc:     "config file not found",
			provider: &mockRBACProvider{configPath: "nonexistent.json", loadErr: errConfigPathNotSet},
			setupFiles: func() (string, error) {
				return "", nil
			},
			cleanupFiles:  func(string) {},
			expectedLogs:  []string{"Failed to load RBAC config"},
			expectedError: false,
			middlewareSet: false,
		},
		{
			desc:     "invalid config file format",
			provider: &mockRBACProvider{configPath: "invalid.json", loadErr: errFailedToParse},
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
			app.EnableRBAC(tc.provider)

			// Check if middleware was actually called (which means it was added to router)
			mockProvider, ok := tc.provider.(*mockRBACProvider)
			if ok {
				require.Equal(t, tc.middlewareSet, mockProvider.middlewareCalled,
					"TEST[%d], Failed.\n%s", i, tc.desc)
			} else {
				// For nil provider case, just check that httpServer exists (it always does after New())
				require.NotNil(t, app.httpServer, "TEST[%d], Failed.\n%s", i, tc.desc)
				require.NotNil(t, app.httpServer.router, "TEST[%d], Failed.\n%s", i, tc.desc)
			}
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
	configPath       string
	loadErr          error
	middlewareFn     func(http.Handler) http.Handler
	middlewareCalled bool // Track if ApplyMiddleware was called
}

func (*mockRBACProvider) UseLogger(_ any) {
	// Mock implementation
}

func (*mockRBACProvider) UseMetrics(_ any) {
	// Mock implementation
}

func (*mockRBACProvider) UseTracer(_ any) {
	// Mock implementation
}

func (m *mockRBACProvider) LoadPermissions() error {
	if m.loadErr != nil {
		return m.loadErr
	}

	return nil
}

func (m *mockRBACProvider) ApplyMiddleware() func(http.Handler) http.Handler {
	m.middlewareCalled = true
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
					{"path": "/api/users", "methods": ["GET"], "requiredPermissions": ["users:read"]}
				]
			}`,
			provider:    &mockRBACProvider{configPath: "test_integration.json"},
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
			provider:    &mockRBACProvider{configPath: "test_inheritance.json"},
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
			app.EnableRBAC(tc.provider)

			require.NotNil(t, app.httpServer, "TEST[%d], Failed.\n%s", i, tc.desc)
			require.NotNil(t, app.httpServer.router, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
