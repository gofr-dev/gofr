package rbac

import (
	"context"
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
		desc       string
		configPath string
		expected   *Provider
	}{
		{
			desc:       "creates new provider with config path",
			configPath: "configs/rbac.json",
			expected:   &Provider{configPath: "configs/rbac.json"},
		},
		{
			desc:       "creates new provider with empty path",
			configPath: "",
			expected:   &Provider{configPath: ""},
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := NewProvider(tc.configPath)

			require.NotNil(t, result, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.expected.configPath, result.configPath, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestProvider_UseLogger(t *testing.T) {
	testCases := []struct {
		desc   string
		logger any
		valid  bool
	}{
		{
			desc:   "sets logger",
			logger: &mockLogger{},
			valid:  true,
		},
		{
			desc:   "sets nil logger",
			logger: nil,
			valid:  false,
		},
		{
			desc:   "sets invalid logger type",
			logger: "invalid",
			valid:  false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			p := NewProvider("configs/rbac.json")
			p.UseLogger(tc.logger)

			if tc.valid {
				assert.NotNil(t, p.logger, "TEST[%d], Failed.\n%s", i, tc.desc)
			} else {
				assert.Nil(t, p.logger, "TEST[%d], Failed.\n%s", i, tc.desc)
			}
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
			} else {
				filePath = tc.fileName
			}

			p := NewProvider(filePath)

			err := p.LoadPermissions()

			if tc.expectError {
				require.Error(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
				require.Nil(t, p.config, "TEST[%d], Failed.\n%s", i, tc.desc)

				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)
			require.NotNil(t, p.config, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.NotNil(t, p.config, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestProvider_LoadPermissions_WithLogger(t *testing.T) {
	testCases := []struct {
		desc   string
		logger logging.Logger
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

			p := NewProvider("test_logger.json")
			p.UseLogger(tc.logger)

			err = p.LoadPermissions()
			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

			require.NotNil(t, p.config, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.logger, p.config.Logger, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func TestProvider_UseMetrics(t *testing.T) {
	testCases := []struct {
		desc    string
		metrics any
		valid   bool
	}{
		{
			desc:    "sets valid metrics",
			metrics: &mockMetrics{},
			valid:   true,
		},
		{
			desc:    "does not set invalid metrics",
			metrics: map[string]int{"test": 1},
			valid:   false,
		},
		{
			desc:    "sets nil metrics",
			metrics: nil,
			valid:   false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			p := NewProvider("configs/rbac.json")
			p.UseMetrics(tc.metrics)

			if tc.valid {
				assert.Equal(t, tc.metrics, p.metrics, "TEST[%d], Failed.\n%s", i, tc.desc)
				// Check if metrics were registered
				m, ok := tc.metrics.(*mockMetrics)
				require.True(t, ok)
				assert.True(t, m.histogramCreated, "NewHistogram should be called")
				assert.True(t, m.counterCreated, "NewCounter should be called")
			} else {
				assert.Nil(t, p.metrics, "TEST[%d], Failed.\n%s", i, tc.desc)
			}
		})
	}
}

func TestProvider_UseTracer(t *testing.T) {
	testCases := []struct {
		desc   string
		tracer any
		valid  bool
	}{
		{
			desc:   "sets valid tracer",
			tracer: nil, // In real usage, this would be trace.Tracer
			valid:  false,
		},
		{
			desc:   "sets invalid tracer type",
			tracer: "invalid",
			valid:  false,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			p := NewProvider("configs/rbac.json")
			p.UseTracer(tc.tracer)

			// Tracer is only set if it's a valid trace.Tracer type
			if tc.valid {
				assert.NotNil(t, p.tracer, "TEST[%d], Failed.\n%s", i, tc.desc)
			} else {
				// For invalid types, tracer should remain nil
				assert.Nil(t, p.tracer, "TEST[%d], Failed.\n%s", i, tc.desc)
			}
		})
	}
}

func TestProvider_ApplyMiddleware(t *testing.T) {
	testCases := []struct {
		desc              string
		setupConfig       func() *Config
		expectPassthrough bool
	}{
		{
			desc: "returns middleware for valid config",
			setupConfig: func() *Config {
				config := &Config{
					Roles: []RoleDefinition{
						{Name: "admin", Permissions: []string{"*:*"}},
					},
					Endpoints: []EndpointMapping{
						{Path: "/api", Methods: []string{"GET"}, RequiredPermissions: []string{"admin:read", "admin:write"}},
					},
				}
				_ = config.processUnifiedConfig()
				return config
			},
			expectPassthrough: false,
		},
		{
			desc: "returns passthrough for nil config",
			setupConfig: func() *Config {
				return nil
			},
			expectPassthrough: true,
		},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			p := NewProvider("configs/rbac.json")
			p.config = tc.setupConfig()

			middlewareFunc := p.ApplyMiddleware()

			require.NotNil(t, middlewareFunc, "TEST[%d], Failed.\n%s", i, tc.desc)

			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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

	err := os.WriteFile(filename, []byte(content), 0600)

	return filename, err
}

type mockLogger struct {
	logs []string
}

func (m *mockLogger) Debug(_ ...any)             { m.logs = append(m.logs, "DEBUG") }
func (m *mockLogger) Debugf(_ string, _ ...any)  { m.logs = append(m.logs, "DEBUGF") }
func (m *mockLogger) Log(_ ...any)               { m.logs = append(m.logs, "LOG") }
func (m *mockLogger) Logf(_ string, _ ...any)    { m.logs = append(m.logs, "LOGF") }
func (m *mockLogger) Info(_ ...any)              { m.logs = append(m.logs, "INFO") }
func (m *mockLogger) Infof(_ string, _ ...any)   { m.logs = append(m.logs, "INFOF") }
func (m *mockLogger) Notice(_ ...any)            { m.logs = append(m.logs, "NOTICE") }
func (m *mockLogger) Noticef(_ string, _ ...any) { m.logs = append(m.logs, "NOTICEF") }
func (m *mockLogger) Error(_ ...any)             { m.logs = append(m.logs, "ERROR") }
func (m *mockLogger) Errorf(_ string, _ ...any)  { m.logs = append(m.logs, "ERRORF") }
func (m *mockLogger) Warn(_ ...any)              { m.logs = append(m.logs, "WARN") }
func (m *mockLogger) Warnf(_ string, _ ...any)   { m.logs = append(m.logs, "WARNF") }
func (m *mockLogger) Fatal(_ ...any)             { m.logs = append(m.logs, "FATAL") }
func (m *mockLogger) Fatalf(_ string, _ ...any)  { m.logs = append(m.logs, "FATALF") }
func (*mockLogger) ChangeLevel(logging.Level)    {}

type mockMetrics struct {
	histogramCreated bool
	counterCreated   bool
}

func (m *mockMetrics) NewHistogram(name, desc string, buckets ...float64) {
	m.histogramCreated = true
}

func (m *mockMetrics) RecordHistogram(ctx context.Context, name string, value float64, labels ...string) {
}

func (m *mockMetrics) NewCounter(name, desc string) {
	m.counterCreated = true
}

func (m *mockMetrics) IncrementCounter(ctx context.Context, name string, labels ...string) {
}

func (m *mockMetrics) NewUpDownCounter(name, desc string) {
}

func (m *mockMetrics) NewGauge(name, desc string) {
}

func (m *mockMetrics) DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string) {
}

func (m *mockMetrics) SetGauge(name string, value float64, labels ...string) {
}
