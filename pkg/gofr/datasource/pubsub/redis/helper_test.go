package redis

import (
	"crypto/tls"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gofr.dev/pkg/gofr/config"
)

func TestValidateConfigs(t *testing.T) { //nolint:funlen // Test function with many test cases
	tests := []struct {
		name        string
		cfg         *Config
		wantErr     bool
		validateErr func(t *testing.T, err error)
		validateCfg func(t *testing.T, cfg *Config)
	}{
		{
			name: "valid config",
			cfg: &Config{
				Addr:        "localhost:6379",
				DB:          0,
				PoolSize:    10,
				DialTimeout: 5 * time.Second,
				ReadTimeout: 3 * time.Second,
			},
			wantErr: false,
			validateCfg: func(t *testing.T, cfg *Config) {
				t.Helper()
				t.Helper()
				assert.Equal(t, "localhost:6379", cfg.Addr)
				assert.Equal(t, 0, cfg.DB)
			},
		},
		{
			name: "empty address",
			cfg: &Config{
				Addr: "",
				DB:   0,
			},
			wantErr: true,
			validateErr: func(t *testing.T, err error) {
				t.Helper()
				t.Helper()
				require.Equal(t, errAddrNotProvided, err)
			},
		},
		{
			name: "invalid DB - negative",
			cfg: &Config{
				Addr: "localhost:6379",
				DB:   -1,
			},
			wantErr: true,
			validateErr: func(t *testing.T, err error) {
				t.Helper()
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "database number must be non-negative")
			},
		},
		{
			name: "zero pool size - should default",
			cfg: &Config{
				Addr:     "localhost:6379",
				DB:       0,
				PoolSize: 0,
			},
			wantErr: false,
			validateCfg: func(t *testing.T, cfg *Config) {
				t.Helper()
				t.Helper()
				assert.Equal(t, 10, cfg.PoolSize)
			},
		},
		{
			name: "zero dial timeout - should default",
			cfg: &Config{
				Addr:        "localhost:6379",
				DB:          0,
				DialTimeout: 0,
			},
			wantErr: false,
			validateCfg: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, 5*time.Second, cfg.DialTimeout)
			},
		},
		{
			name: "zero read timeout - should default",
			cfg: &Config{
				Addr:        "localhost:6379",
				DB:          0,
				ReadTimeout: 0,
			},
			wantErr: false,
			validateCfg: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, 3*time.Second, cfg.ReadTimeout)
			},
		},
		{
			name: "zero write timeout - should default",
			cfg: &Config{
				Addr:         "localhost:6379",
				DB:           0,
				WriteTimeout: 0,
			},
			wantErr: false,
			validateCfg: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, 3*time.Second, cfg.WriteTimeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfigs(tt.cfg)

			require.Equal(t, tt.wantErr, err != nil, "error expectation mismatch")

			if tt.wantErr {
				require.Error(t, err)

				if tt.validateErr != nil {
					tt.validateErr(t, err)
				}

				return
			}

			require.NoError(t, err)

			if tt.validateCfg != nil {
				tt.validateCfg(t, tt.cfg)
			}
		})
	}
}

func TestCreateRedisOptions(t *testing.T) { //nolint:funlen // Test function with many test cases
	tests := []struct {
		name        string
		cfg         *Config
		wantErr     bool
		validateErr func(t *testing.T, err error)
		validate    func(t *testing.T, options *redis.Options)
	}{
		{
			name: "successful options creation",
			cfg: &Config{
				Addr:         "localhost:6379",
				Password:     "password",
				DB:           1,
				MaxRetries:   3,
				DialTimeout:  5 * time.Second,
				ReadTimeout:  3 * time.Second,
				WriteTimeout: 3 * time.Second,
				PoolSize:     10,
				MinIdleConns: 5,
				MaxIdleConns: 10,
			},
			wantErr: false,
			validate: func(t *testing.T, options *redis.Options) {
				t.Helper()
				assert.Equal(t, "localhost:6379", options.Addr)
				assert.Equal(t, "password", options.Password)
				assert.Equal(t, 1, options.DB)
				assert.Equal(t, 3, options.MaxRetries)
				assert.Equal(t, 10, options.PoolSize)
			},
		},
		{
			name: "options with TLS",
			cfg: &Config{
				Addr: "localhost:6379",
				DB:   0,
				TLS: &TLSConfig{
					InsecureSkipVerify: true,
				},
			},
			wantErr: false,
			validate: func(t *testing.T, options *redis.Options) {
				t.Helper()
				assert.NotNil(t, options.TLSConfig)
				assert.True(t, options.TLSConfig.InsecureSkipVerify)
			},
		},
		{
			name: "options with connection lifetime",
			cfg: &Config{
				Addr:            "localhost:6379",
				DB:              0,
				ConnMaxIdleTime: 5 * time.Minute,
				ConnMaxLifetime: 30 * time.Minute,
			},
			wantErr: false,
			validate: func(t *testing.T, options *redis.Options) {
				t.Helper()
				assert.Equal(t, 5*time.Minute, options.ConnMaxIdleTime)
				assert.Equal(t, 30*time.Minute, options.ConnMaxLifetime)
			},
		},
		{
			name: "options with TLS cert files",
			cfg: func(t *testing.T) *Config {
				t.Helper()
				// Create temporary cert files
				certFile, _ := os.CreateTemp(t.TempDir(), "cert.pem")
				keyFile, _ := os.CreateTemp(t.TempDir(), "key.pem")
				_ = certFile.Close()
				_ = keyFile.Close()

				return &Config{
					Addr: "localhost:6379",
					DB:   0,
					TLS: &TLSConfig{
						CertFile: certFile.Name(),
						KeyFile:  keyFile.Name(),
					},
				}
			}(t),
			wantErr: true, // Will fail because cert files are empty
			validateErr: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to create TLS config")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options, err := createRedisOptions(tt.cfg)

			require.Equal(t, tt.wantErr, err != nil, "error expectation mismatch")

			if tt.wantErr {
				require.Error(t, err)

				if tt.validateErr != nil {
					tt.validateErr(t, err)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, options)

			if tt.validate != nil {
				tt.validate(t, options)
			}
		})
	}
}

func TestCreateTLSConfig(t *testing.T) {
	tests := []struct {
		name        string
		tlsConfig   *TLSConfig
		wantErr     bool
		validateErr func(t *testing.T, err error)
		validate    func(t *testing.T, tlsConfig *tls.Config)
	}{
		{
			name: "basic TLS config",
			tlsConfig: &TLSConfig{
				InsecureSkipVerify: true,
			},
			wantErr: false,
			validate: func(t *testing.T, tlsConfig *tls.Config) {
				t.Helper()
				assert.NotNil(t, tlsConfig)
				assert.True(t, tlsConfig.InsecureSkipVerify)
			},
		},
		{
			name: "TLS config with invalid cert files",
			tlsConfig: &TLSConfig{
				CertFile: "nonexistent.pem",
				KeyFile:  "nonexistent.key",
			},
			wantErr: true,
			validateErr: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to load client certificate")
			},
		},
		{
			name: "TLS config with invalid CA cert file",
			tlsConfig: &TLSConfig{
				CACertFile: "nonexistent-ca.pem",
			},
			wantErr: true,
			validateErr: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to read CA certificate")
			},
		},
		{
			name: "TLS config with invalid CA cert content",
			tlsConfig: func(t *testing.T) *TLSConfig {
				t.Helper()
				// Create temp file with invalid content
				tmpFile, _ := os.CreateTemp(t.TempDir(), "invalid-ca.pem")
				_, _ = tmpFile.WriteString("invalid cert content")
				_ = tmpFile.Close()

				return &TLSConfig{
					CACertFile: tmpFile.Name(),
				}
			}(t),
			wantErr: true,
			validateErr: func(t *testing.T, err error) {
				t.Helper()
				require.Error(t, err)
				// Could be either parse error or file read error depending on timing
				assert.True(t, errors.Is(err, errFailedToParseCACert) ||
					strings.Contains(err.Error(), "failed to read CA certificate"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsConfig, err := createTLSConfig(tt.tlsConfig)

			require.Equal(t, tt.wantErr, err != nil, "error expectation mismatch")

			if tt.wantErr {
				require.Error(t, err)

				if tt.validateErr != nil {
					tt.validateErr(t, err)
				}

				return
			}

			require.NoError(t, err)
			require.NotNil(t, tlsConfig)

			if tt.validate != nil {
				tt.validate(t, tlsConfig)
			}
		})
	}
}

func TestIsConnected(t *testing.T) {
	tests := []struct {
		name        string
		setupClient func(t *testing.T) (*Client, func())
		want        bool
	}{
		{
			name: "connected client",
			setupClient: func(t *testing.T) (*Client, func()) {
				t.Helper()
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)

				return client, func() {
					_ = client.Close()
					s.Close()
				}
			},
			want: true,
		},
		{
			name: "nil pubConn",
			setupClient: func(_ *testing.T) (*Client, func()) {
				return New(DefaultConfig()), func() {}
			},
			want: false,
		},
		{
			name: "disconnected client",
			setupClient: func(t *testing.T) (*Client, func()) {
				t.Helper()
				s, err := miniredis.Run()
				require.NoError(t, err)

				cfg := DefaultConfig()
				cfg.Addr = s.Addr()

				client := New(cfg)
				client.Connect()
				time.Sleep(100 * time.Millisecond)
				s.Close() // Close Redis

				return client, func() {
					_ = client.Close()
				}
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, cleanup := tt.setupClient(t)
			defer cleanup()

			result := client.isConnected()
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestParseQueryArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []any
		wantTimeout time.Duration
		wantLimit   int
	}{
		{
			name:        "no args - defaults",
			args:        []any{},
			wantTimeout: 30 * time.Second,
			wantLimit:   10,
		},
		{
			name:        "with timeout only",
			args:        []any{5 * time.Second},
			wantTimeout: 5 * time.Second,
			wantLimit:   10,
		},
		{
			name:        "with timeout and limit",
			args:        []any{10 * time.Second, 5},
			wantTimeout: 10 * time.Second,
			wantLimit:   5,
		},
		{
			name:        "invalid timeout type",
			args:        []any{"invalid"},
			wantTimeout: 30 * time.Second,
			wantLimit:   10,
		},
		{
			name:        "invalid limit type",
			args:        []any{5 * time.Second, "invalid"},
			wantTimeout: 5 * time.Second,
			wantLimit:   10,
		},
		{
			name:        "zero timeout - uses default",
			args:        []any{0 * time.Second},
			wantTimeout: 30 * time.Second,
			wantLimit:   10,
		},
		{
			name:        "negative limit - uses default",
			args:        []any{5 * time.Second, -1},
			wantTimeout: 5 * time.Second,
			wantLimit:   10,
		},
		{
			name:        "zero limit - uses default",
			args:        []any{5 * time.Second, 0},
			wantTimeout: 5 * time.Second,
			wantLimit:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeout, limit := parseQueryArgs(tt.args...)
			assert.Equal(t, tt.wantTimeout, timeout)
			assert.Equal(t, tt.wantLimit, limit)
		})
	}
}

func TestGetRedisPubSubConfig(t *testing.T) { //nolint:funlen // Test function with many test cases
	tests := []struct {
		name        string
		setupConfig func() config.Config
		validate    func(t *testing.T, cfg *Config)
	}{
		{
			name: "with REDIS_PUBSUB_ADDR",
			setupConfig: func() config.Config {
				return config.NewMockConfig(map[string]string{
					"REDIS_PUBSUB_ADDR": "localhost:6380",
				})
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()

				assert.Equal(t, "localhost:6380", cfg.Addr)
			},
		},
		{
			name: "with REDIS_HOST and REDIS_PORT",
			setupConfig: func() config.Config {
				return config.NewMockConfig(map[string]string{
					"REDIS_HOST": "redis.example.com",
					"REDIS_PORT": "6381",
				})
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "redis.example.com:6381", cfg.Addr)
			},
		},
		{
			name: "with REDIS_HOST and default port",
			setupConfig: func() config.Config {
				return config.NewMockConfig(map[string]string{
					"REDIS_HOST": "redis.example.com",
				})
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()

				assert.Equal(t, "redis.example.com:6379", cfg.Addr)
			},
		},
		{
			name: "with REDIS_PASSWORD",
			setupConfig: func() config.Config {
				return config.NewMockConfig(map[string]string{
					"REDIS_PUBSUB_ADDR": "localhost:6379",
					"REDIS_PASSWORD":    "secret123",
				})
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()

				assert.Equal(t, "secret123", cfg.Password)
			},
		},
		{
			name: "with REDIS_PUBSUB_DB",
			setupConfig: func() config.Config {
				return config.NewMockConfig(map[string]string{
					"REDIS_PUBSUB_ADDR": "localhost:6379",
					"REDIS_PUBSUB_DB":   "5",
				})
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()

				assert.Equal(t, 5, cfg.DB)
			},
		},
		{
			name: "with REDIS_DB fallback",
			setupConfig: func() config.Config {
				return config.NewMockConfig(map[string]string{
					"REDIS_HOST": "localhost",
					"REDIS_DB":   "3",
				})
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, 3, cfg.DB)
			},
		},
		{
			name: "with timeout configurations",
			setupConfig: func() config.Config {
				return config.NewMockConfig(map[string]string{
					"REDIS_PUBSUB_ADDR":          "localhost:6379",
					"REDIS_PUBSUB_DIAL_TIMEOUT":  "10s",
					"REDIS_PUBSUB_READ_TIMEOUT":  "5s",
					"REDIS_PUBSUB_WRITE_TIMEOUT": "5s",
				})
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, 10*time.Second, cfg.DialTimeout)
				assert.Equal(t, 5*time.Second, cfg.ReadTimeout)
				assert.Equal(t, 5*time.Second, cfg.WriteTimeout)
			},
		},
		{
			name: "with pool settings",
			setupConfig: func() config.Config {
				return config.NewMockConfig(map[string]string{
					"REDIS_PUBSUB_ADDR":           "localhost:6379",
					"REDIS_PUBSUB_POOL_SIZE":      "20",
					"REDIS_PUBSUB_MIN_IDLE_CONNS": "10",
					"REDIS_PUBSUB_MAX_IDLE_CONNS": "15",
					"REDIS_PUBSUB_MAX_RETRIES":    "5",
				})
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, 20, cfg.PoolSize)
				assert.Equal(t, 10, cfg.MinIdleConns)
				assert.Equal(t, 15, cfg.MaxIdleConns)
				assert.Equal(t, 5, cfg.MaxRetries)
			},
		},
		{
			name: "with TLS enabled",
			setupConfig: func() config.Config {
				return config.NewMockConfig(map[string]string{
					"REDIS_PUBSUB_ADDR":              "localhost:6379",
					"REDIS_TLS_ENABLED":              "true",
					"REDIS_TLS_INSECURE_SKIP_VERIFY": "true",
					"REDIS_TLS_CA_CERT":              "/path/to/ca.pem",
					"REDIS_TLS_CERT":                 "/path/to/cert.pem",
					"REDIS_TLS_KEY":                  "/path/to/key.pem",
				})
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()
				require.NotNil(t, cfg.TLS)
				assert.True(t, cfg.TLS.InsecureSkipVerify)
				assert.Equal(t, "/path/to/ca.pem", cfg.TLS.CACertFile)
				assert.Equal(t, "/path/to/cert.pem", cfg.TLS.CertFile)
				assert.Equal(t, "/path/to/key.pem", cfg.TLS.KeyFile)
			},
		},
		{
			name: "with invalid timeout - uses default",
			setupConfig: func() config.Config {
				return config.NewMockConfig(map[string]string{
					"REDIS_PUBSUB_ADDR":         "localhost:6379",
					"REDIS_PUBSUB_DIAL_TIMEOUT": "invalid",
				})
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, 5*time.Second, cfg.DialTimeout) // Default
			},
		},
		{
			name: "with invalid DB - uses default",
			setupConfig: func() config.Config {
				return config.NewMockConfig(map[string]string{
					"REDIS_PUBSUB_ADDR": "localhost:6379",
					"REDIS_PUBSUB_DB":   "invalid",
				})
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, 0, cfg.DB) // Default
			},
		},
		{
			name: "with negative DB - uses default",
			setupConfig: func() config.Config {
				return config.NewMockConfig(map[string]string{
					"REDIS_PUBSUB_ADDR": "localhost:6379",
					"REDIS_PUBSUB_DB":   "-1",
				})
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, 0, cfg.DB) // Default
			},
		},
		{
			name: "empty config - uses defaults",
			setupConfig: func() config.Config {
				return config.NewMockConfig(map[string]string{})
			},
			validate: func(t *testing.T, cfg *Config) {
				t.Helper()
				assert.Equal(t, "localhost:6379", cfg.Addr) // Default
				assert.Equal(t, 0, cfg.DB)                  // Default
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockConfig := tt.setupConfig()
			cfg := getRedisPubSubConfig(mockConfig)
			require.NotNil(t, cfg)
			tt.validate(t, cfg)
		})
	}
}

func TestSanitizeRedisAddr(t *testing.T) {
	tests := []struct {
		name     string
		addr     string
		expected string
	}{
		{
			name:     "plain host:port - no sanitization needed",
			addr:     "localhost:6379",
			expected: "localhost:6379",
		},
		{
			name:     "Redis URI with credentials",
			addr:     "redis://user:password@localhost:6379",
			expected: "redis://localhost:6379",
		},
		{
			name:     "Redis URI with credentials and database",
			addr:     "redis://user:password@localhost:6379/0",
			expected: "redis://localhost:6379/0",
		},
		{
			name:     "Redis URI with credentials and path",
			addr:     "redis://user:password@localhost:6379/db1",
			expected: "redis://localhost:6379/db1",
		},
		{
			name:     "Rediss URI with credentials (TLS)",
			addr:     "rediss://user:password@localhost:6380",
			expected: "rediss://localhost:6380",
		},
		{
			name:     "address with @ but no scheme",
			addr:     "user:password@localhost:6379",
			expected: "localhost:6379",
		},
		{
			name:     "empty address",
			addr:     "",
			expected: "",
		},
		{
			name:     "address with multiple @ symbols - preserves scheme",
			addr:     "redis://user:pass@host1@host2:6379",
			expected: "redis://host2:6379",
		},
		{
			name:     "address without @ symbol",
			addr:     "127.0.0.1:6379",
			expected: "127.0.0.1:6379",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeRedisAddr(tt.addr)
			assert.Equal(t, tt.expected, result)
		})
	}
}
