package rbac

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/logging"
)

func TestNewProvider(t *testing.T) {
	testCases := []struct {
		desc     string
		expected *Provider
	}{
		{
			desc:     "creates new provider",
			expected: &Provider{},
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := NewProvider()

			require.NotNil(t, result, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.expected.config, result.config, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.expected.logger, result.logger, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestProvider_UseLogger(t *testing.T) {
	testCases := []struct {
		desc   string
		logger any
	}{
		{
			desc:   "sets logger",
			logger: &mockLogger{},
		},
		{
			desc:   "sets nil logger",
			logger: nil,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			p := NewProvider()
			p.UseLogger(tc.logger)

			assert.Equal(t, tc.logger, p.logger, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestProvider_LoadPermissions(t *testing.T) {
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
				"roles": [{"name": "admin", "permissions": ["admin:read", "admin:write"]}],
				"endpoints": [{"path": "/api", "methods": ["GET"], "requiredPermissions": ["admin:read"]}]
			}`,
			fileName:     "test_load.json",
			expectError:  false,
			expectConfig: true,
		},
		{
			desc: "loads valid yaml config",
			fileContent: `roles:
  - name: admin
    permissions: ["admin:read", "admin:write"]
endpoints:
  - path: /api
    methods: ["GET"]
    requiredPermissions: ["admin:read"]`,
			fileName:     "test_load.yaml",
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
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var filePath string

			if tc.fileContent != "" {
				path, err := createTestFile(tc.fileName, tc.fileContent)
				require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				filePath = path
				defer os.Remove(filePath)
			}

			p := NewProvider()
			config, err := p.LoadPermissions(tc.fileName)

			if tc.expectError {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				require.Nil(t, config, "TEST[%d], Failed.\n%s", i, tc.desc)
				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			require.NotNil(t, config, "TEST[%d], Failed.\n%s", i, tc.desc)

			rbacConfig, ok := config.(*Config)
			require.True(t, ok, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.NotNil(t, rbacConfig, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, rbacConfig, p.config, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestProvider_LoadPermissions_WithLogger(t *testing.T) {
	testCases := []struct {
		desc   string
		logger any
	}{
		{
			desc:   "sets logger on config when logger provided",
			logger: &mockLogger{},
		},
		{
			desc:   "does not set logger when nil",
			logger: nil,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			fileContent := `{
				"roles": [{"name": "admin", "permissions": ["admin:read", "admin:write"]}],
				"endpoints": [{"path": "/api", "methods": ["GET"], "requiredPermissions": ["admin:read"]}]
			}`

			path, err := createTestFile("test_logger.json", fileContent)
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			defer os.Remove(path)

			p := NewProvider()
			p.UseLogger(tc.logger)

			config, err := p.LoadPermissions("test_logger.json")
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

			rbacConfig, ok := config.(*Config)
			require.True(t, ok, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.logger, rbacConfig.Logger, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestProvider_GetMiddleware(t *testing.T) {
	testCases := []struct {
		desc              string
		config            any
		expectPassthrough bool
	}{
		{
			desc: "returns middleware for valid config",
			config: &Config{
				Roles: []RoleDefinition{
					{Name: "admin", Permissions: []string{"*:*"}},
				},
				Endpoints: []EndpointMapping{
					{Path: "/api", Methods: []string{"GET"}, RequiredPermissions: []string{"admin:read", "admin:write"}},
				},
			},
			expectPassthrough: false,
		},
		{
			desc:              "returns passthrough for invalid config type",
			config:            "invalid",
			expectPassthrough: true,
		},
		{
			desc:              "returns passthrough for nil config",
			config:            nil,
			expectPassthrough: true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			p := NewProvider()

			middlewareFunc := p.GetMiddleware(tc.config)

			require.NotNil(t, middlewareFunc, "TEST[%d], Failed.\n%s", i, tc.desc)

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			wrapped := middlewareFunc(handler)

			require.NotNil(t, wrapped, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func createTestFile(filename, content string) (string, error) {
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

type mockLogger struct {
	logs []string
}

func (m *mockLogger) Debug(args ...any)                 { m.logs = append(m.logs, "DEBUG") }
func (m *mockLogger) Debugf(format string, args ...any) { m.logs = append(m.logs, "DEBUGF") }
func (m *mockLogger) Log(args ...any)                   { m.logs = append(m.logs, "LOG") }
func (m *mockLogger) Logf(format string, args ...any)   { m.logs = append(m.logs, "LOGF") }
func (m *mockLogger) Info(args ...any)                  { m.logs = append(m.logs, "INFO") }
func (m *mockLogger) Infof(format string, args ...any)  { m.logs = append(m.logs, "INFOF") }
func (m *mockLogger) Notice(args ...any)                { m.logs = append(m.logs, "NOTICE") }
func (m *mockLogger) Noticef(format string, args ...any) { m.logs = append(m.logs, "NOTICEF") }
func (m *mockLogger) Error(args ...any)                 { m.logs = append(m.logs, "ERROR") }
func (m *mockLogger) Errorf(format string, args ...any) { m.logs = append(m.logs, "ERRORF") }
func (m *mockLogger) Warn(args ...any)                  { m.logs = append(m.logs, "WARN") }
func (m *mockLogger) Warnf(format string, args ...any)  { m.logs = append(m.logs, "WARNF") }
func (m *mockLogger) Fatal(args ...any)                 { m.logs = append(m.logs, "FATAL") }
func (m *mockLogger) Fatalf(format string, args ...any) { m.logs = append(m.logs, "FATALF") }
func (m *mockLogger) ChangeLevel(level logging.Level) {}
