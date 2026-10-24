package redis

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func testGetRedisConfig(t *testing.T, configMap map[string]string) *Config {
	t.Helper()

	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockConfig := config.NewMockConfig(configMap)

	return getRedisConfig(mockConfig, mockLogger)
}

func TestGetRedisConfig_Defaults(t *testing.T) {
	conf := testGetRedisConfig(t, map[string]string{
		"PUBSUB_BACKEND": "REDIS", // Required to trigger PubSub config parsing
		"REDIS_HOST":     "localhost",
	})

	assert.Equal(t, "localhost", conf.HostName)
	assert.Equal(t, defaultRedisPort, conf.Port)
	assert.Equal(t, 0, conf.DB)
	assert.Nil(t, conf.TLS)
	// PubSubStreamsConfig is initialized when mode is streams (default)
	assert.NotNil(t, conf.PubSubStreamsConfig)
	assert.Equal(t, "streams", conf.PubSubMode) // Default mode is now streams
}

func TestGetRedisConfig_InvalidPortAndDB(t *testing.T) {
	conf := testGetRedisConfig(t, map[string]string{
		"REDIS_HOST": "localhost",
		"REDIS_PORT": "invalid",
		"REDIS_DB":   "invalid",
	})

	assert.Equal(t, defaultRedisPort, conf.Port)
	assert.Equal(t, 0, conf.DB)
}

func TestGetRedisConfig_TLS(t *testing.T) {
	certFile, err := os.CreateTemp(t.TempDir(), "cert-*.pem")
	require.NoError(t, err)

	defer os.Remove(certFile.Name())
	defer certFile.Close()

	keyFile, err := os.CreateTemp(t.TempDir(), "key-*.pem")
	require.NoError(t, err)

	defer os.Remove(keyFile.Name())
	defer keyFile.Close()

	caFile, err := os.CreateTemp(t.TempDir(), "ca-*.pem")
	require.NoError(t, err)

	defer os.Remove(caFile.Name())
	defer caFile.Close()

	_, _ = certFile.WriteString("-----BEGIN CERTIFICATE-----\nMIID\n-----END CERTIFICATE-----")
	_, _ = keyFile.WriteString("-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----")
	_, _ = caFile.WriteString("-----BEGIN CERTIFICATE-----\nMIID\n-----END CERTIFICATE-----")

	conf := testGetRedisConfig(t, map[string]string{
		"REDIS_HOST":        "localhost",
		"REDIS_TLS_ENABLED": "true",
		"REDIS_TLS_CERT":    certFile.Name(),
		"REDIS_TLS_KEY":     keyFile.Name(),
		"REDIS_TLS_CA_CERT": caFile.Name(),
	})

	assert.NotNil(t, conf.TLS)
	assert.Equal(t, uint16(tls.VersionTLS12), conf.TLS.MinVersion)
}

func TestGetRedisConfig_TLS_InvalidFiles(t *testing.T) {
	conf := testGetRedisConfig(t, map[string]string{
		"REDIS_HOST":        "localhost",
		"REDIS_TLS_ENABLED": "true",
		"REDIS_TLS_CERT":    "nonexistent_cert.pem",
		"REDIS_TLS_KEY":     "nonexistent_key.pem",
		"REDIS_TLS_CA_CERT": "nonexistent_ca.pem",
	})

	assert.NotNil(t, conf.TLS)
	assert.Empty(t, conf.TLS.Certificates)
	assert.Nil(t, conf.TLS.RootCAs)
}

func TestGetRedisConfig_PubSubStreams(t *testing.T) {
	conf := testGetRedisConfig(t, map[string]string{
		"PUBSUB_BACKEND":               "REDIS",
		"REDIS_HOST":                   "localhost",
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "mygroup",
		"REDIS_STREAMS_CONSUMER_NAME":  "myconsumer",
		"REDIS_STREAMS_MAXLEN":         "1000",
		"REDIS_STREAMS_BLOCK_TIMEOUT":  "2s",
	})

	assert.Equal(t, "streams", conf.PubSubMode)
	require.NotNil(t, conf.PubSubStreamsConfig)

	assert.Equal(t, "mygroup", conf.PubSubStreamsConfig.ConsumerGroup)
	assert.Equal(t, "myconsumer", conf.PubSubStreamsConfig.ConsumerName)
	assert.Equal(t, int64(1000), conf.PubSubStreamsConfig.MaxLen)
	assert.Equal(t, 2*time.Second, conf.PubSubStreamsConfig.Block)
}

func TestGetRedisConfig_PubSubStreams_Defaults(t *testing.T) {
	conf := testGetRedisConfig(t, map[string]string{
		"PUBSUB_BACKEND":               "REDIS",
		"REDIS_HOST":                   "localhost",
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "mygroup",
	})

	assert.Equal(t, "streams", conf.PubSubMode)
	require.NotNil(t, conf.PubSubStreamsConfig)

	assert.Equal(t, "mygroup", conf.PubSubStreamsConfig.ConsumerGroup)
	assert.Empty(t, conf.PubSubStreamsConfig.ConsumerName)
	assert.Equal(t, int64(0), conf.PubSubStreamsConfig.MaxLen)
	assert.Equal(t, 1*time.Second, conf.PubSubStreamsConfig.Block)     // Default block (reduced from 5s for better responsiveness)
	assert.InEpsilon(t, 0.7, conf.PubSubStreamsConfig.PELRatio, 0.001) // Default PEL ratio
}

func TestGetRedisConfig_PubSubStreams_InvalidValues(t *testing.T) {
	conf := testGetRedisConfig(t, map[string]string{
		"PUBSUB_BACKEND":               "REDIS",
		"REDIS_HOST":                   "localhost",
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "mygroup",
		"REDIS_STREAMS_MAXLEN":         "invalid",
		"REDIS_STREAMS_BLOCK_TIMEOUT":  "invalid",
	})

	assert.Equal(t, "streams", conf.PubSubMode)
	require.NotNil(t, conf.PubSubStreamsConfig)

	assert.Equal(t, int64(0), conf.PubSubStreamsConfig.MaxLen)
	assert.Equal(t, 1*time.Second, conf.PubSubStreamsConfig.Block)     // Falls back to default when invalid
	assert.InEpsilon(t, 0.7, conf.PubSubStreamsConfig.PELRatio, 0.001) // Falls back to default when invalid
}

func TestGetRedisConfig_PubSubStreams_PELRatio(t *testing.T) {
	conf := testGetRedisConfig(t, map[string]string{
		"PUBSUB_BACKEND":               "REDIS",
		"REDIS_HOST":                   "localhost",
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "mygroup",
		"REDIS_STREAMS_PEL_RATIO":      "0.5",
	})

	assert.Equal(t, "streams", conf.PubSubMode)
	require.NotNil(t, conf.PubSubStreamsConfig)
	assert.InEpsilon(t, 0.5, conf.PubSubStreamsConfig.PELRatio, 0.001)
}

func TestGetRedisConfig_PubSubStreams_PELRatio_Invalid(t *testing.T) {
	// Test invalid ratio (out of range) - should fall back to default
	conf := testGetRedisConfig(t, map[string]string{
		"PUBSUB_BACKEND":               "REDIS",
		"REDIS_HOST":                   "localhost",
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "mygroup",
		"REDIS_STREAMS_PEL_RATIO":      "1.5", // Invalid: > 1.0
	})

	assert.Equal(t, "streams", conf.PubSubMode)
	require.NotNil(t, conf.PubSubStreamsConfig)
	assert.InEpsilon(t, 0.7, conf.PubSubStreamsConfig.PELRatio, 0.001) // Should fall back to default

	// Test invalid ratio (negative) - should fall back to default
	conf2 := testGetRedisConfig(t, map[string]string{
		"PUBSUB_BACKEND":               "REDIS",
		"REDIS_HOST":                   "localhost",
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "mygroup",
		"REDIS_STREAMS_PEL_RATIO":      "-0.1", // Invalid: < 0.0
	})

	require.NotNil(t, conf2.PubSubStreamsConfig)
	assert.InEpsilon(t, 0.7, conf2.PubSubStreamsConfig.PELRatio, 0.001) // Should fall back to default

	// Test invalid ratio (non-numeric) - should fall back to default
	conf3 := testGetRedisConfig(t, map[string]string{
		"PUBSUB_BACKEND":               "REDIS",
		"REDIS_HOST":                   "localhost",
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "mygroup",
		"REDIS_STREAMS_PEL_RATIO":      "invalid",
	})

	require.NotNil(t, conf3.PubSubStreamsConfig)
	assert.InEpsilon(t, 0.7, conf3.PubSubStreamsConfig.PELRatio, 0.001) // Should fall back to default
}

func TestGetRedisConfig_PubSubMode_InvalidFallsBackToStreams(t *testing.T) {
	conf := testGetRedisConfig(t, map[string]string{
		"PUBSUB_BACKEND":    "REDIS",
		"REDIS_HOST":        "localhost",
		"REDIS_PUBSUB_MODE": "invalid-mode",
	})

	assert.Equal(t, "streams", conf.PubSubMode)
	require.NotNil(t, conf.PubSubStreamsConfig)
}

// writeTestCACert writes a self-signed CA certificate to a temporary file and returns its path.
func writeTestCACert(t *testing.T) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "ca.pem")
	require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600))

	return path
}

// getRedisConfigWithLogs runs getRedisConfig and returns the config with the WARN (stdout) and ERROR (stderr) output.
func getRedisConfigWithLogs(t *testing.T, configMap map[string]string) (conf *Config, stdout, stderr string) {
	t.Helper()

	stderr = testutil.StderrOutputForFunc(func() {
		stdout = testutil.StdoutOutputForFunc(func() {
			conf = getRedisConfig(config.NewMockConfig(configMap), logging.NewMockLogger(logging.WARN))
		})
	})

	return conf, stdout, stderr
}

const redisURLTestSecret = "s3cr3tP4ss"

type redisURLCase struct {
	name           string
	env            map[string]string
	wantHost       string
	wantPort       int
	wantUser       string
	wantPassword   string
	wantDB         int
	wantTLS        bool
	wantServerName string
	wantRootCAs    bool
	wantStdout     []string
	wantStderr     []string
	rejectStderr   []string
}

func TestGetRedisConfig_URL(t *testing.T) {
	for _, tc := range redisURLCases(writeTestCACert(t)) {
		t.Run(tc.name, func(t *testing.T) {
			conf, stdout, stderr := getRedisConfigWithLogs(t, tc.env)

			assert.NotContains(t, stdout+stderr, redisURLTestSecret, "logs must never contain the password")

			for _, want := range tc.wantStdout {
				assert.Contains(t, stdout, want)
			}

			for _, want := range tc.wantStderr {
				assert.Contains(t, stderr, want)
			}

			for _, reject := range tc.rejectStderr {
				assert.NotContains(t, stderr, reject)
			}

			assert.Equal(t, tc.wantHost, conf.HostName)

			if tc.wantHost != "" {
				assertRedisURLConfig(t, &tc, conf)
			}
		})
	}
}

func assertRedisURLConfig(t *testing.T, tc *redisURLCase, conf *Config) {
	t.Helper()

	assert.Equal(t, tc.wantPort, conf.Port)
	assert.Equal(t, tc.wantUser, conf.Username)
	assert.Equal(t, tc.wantPassword, conf.Password)
	assert.Equal(t, tc.wantDB, conf.DB)

	require.NotNil(t, conf.Options)
	assert.Equal(t, tc.wantUser, conf.Options.Username)
	assert.Equal(t, tc.wantPassword, conf.Options.Password)
	assert.Equal(t, tc.wantDB, conf.Options.DB)

	if !tc.wantTLS {
		assert.Nil(t, conf.TLS)
		assert.Nil(t, conf.Options.TLSConfig)

		return
	}

	require.NotNil(t, conf.TLS)
	assert.Same(t, conf.TLS, conf.Options.TLSConfig)
	assert.Equal(t, tc.wantServerName, conf.TLS.ServerName)
	assert.Equal(t, uint16(tls.VersionTLS12), conf.TLS.MinVersion)
	assert.Equal(t, tc.wantRootCAs, conf.TLS.RootCAs != nil)
}

func redisURLCases(caPath string) []redisURLCase {
	const secret = redisURLTestSecret

	return []redisURLCase{
		{
			name:         "redis scheme with user, password and db",
			env:          map[string]string{"REDIS_URL": "redis://app:" + secret + "@cache.internal:6380/3"},
			wantHost:     "cache.internal",
			wantPort:     6380,
			wantUser:     "app",
			wantPassword: secret,
			wantDB:       3,
		},
		{
			name:           "rediss scheme turns TLS on with the host as server name",
			env:            map[string]string{"REDIS_URL": "rediss://cache.example.com:6390"},
			wantHost:       "cache.example.com",
			wantPort:       6390,
			wantTLS:        true,
			wantServerName: "cache.example.com",
		},
		{
			name:     "missing port defaults to 6379",
			env:      map[string]string{"REDIS_URL": "redis://cache.internal"},
			wantHost: "cache.internal",
			wantPort: defaultRedisPort,
		},
		{
			name: "URL wins over discrete variables and the warning names them",
			env: map[string]string{
				"REDIS_URL":      "redis://cache.internal:6380",
				"REDIS_HOST":     "other-host",
				"REDIS_PASSWORD": "other-" + secret,
			},
			wantHost:   "cache.internal",
			wantPort:   6380,
			wantStdout: []string{"REDIS_HOST", "REDIS_PASSWORD"},
		},
		{
			name: "rediss with a CA cert keeps the server name and adds root CAs",
			env: map[string]string{
				"REDIS_URL":         "rediss://cache.example.com",
				"REDIS_TLS_CA_CERT": caPath,
			},
			wantHost:       "cache.example.com",
			wantPort:       defaultRedisPort,
			wantTLS:        true,
			wantServerName: "cache.example.com",
			wantRootCAs:    true,
		},
		{
			name:       "invalid port",
			env:        map[string]string{"REDIS_URL": "redis://app:" + secret + "@cache.internal:port"},
			wantStderr: []string{"invalid REDIS_URL"},
		},
		{
			name:       "bad db path",
			env:        map[string]string{"REDIS_URL": "redis://app:" + secret + "@cache.internal:6379/abc"},
			wantStderr: []string{"invalid REDIS_URL"},
		},
		{
			name:       "unix scheme is rejected",
			env:        map[string]string{"REDIS_URL": "unix://app:" + secret + "@/var/run/redis.sock"},
			wantStderr: []string{"invalid REDIS_URL", "unix"},
		},
		{
			name:       "unknown query option",
			env:        map[string]string{"REDIS_URL": "redis://app:" + secret + "@cache.internal:6379?bogus=1"},
			wantStderr: []string{"invalid REDIS_URL", "bogus"},
		},
		{
			// url.Error quotes the URL with %q, so a control character defeats a plain replace of the raw URL.
			name:       "control character in the password",
			env:        map[string]string{"REDIS_URL": "redis://app:" + secret + "\x01@cache.internal:6379"},
			wantStderr: []string{"invalid REDIS_URL", "invalid control character"},
		},
		{
			// url.EscapeError quotes the bad sequence, which is part of the password.
			name:         "bad escape in the password",
			env:          map[string]string{"REDIS_URL": "redis://app:" + secret + "%zz@cache.internal:6379"},
			wantStderr:   []string{"invalid REDIS_URL", "invalid URL escape"},
			rejectStderr: []string{"%zz"},
		},
	}
}

func TestGetRedisConfig_PoolAndTimeouts(t *testing.T) {
	tests := []struct {
		name             string
		env              map[string]string
		wantPoolSize     int
		wantMinIdleConns int
		wantDialTimeout  time.Duration
		wantReadTimeout  time.Duration
		wantWriteTimeout time.Duration
		wantStdout       []string
	}{
		{
			name: "each variable is applied",
			env: map[string]string{
				"REDIS_HOST":           "localhost",
				"REDIS_POOL_SIZE":      "20",
				"REDIS_MIN_IDLE_CONNS": "2",
				"REDIS_DIAL_TIMEOUT":   "2s",
				"REDIS_READ_TIMEOUT":   "500ms",
				"REDIS_WRITE_TIMEOUT":  "750ms",
			},
			wantPoolSize:     20,
			wantMinIdleConns: 2,
			wantDialTimeout:  2 * time.Second,
			wantReadTimeout:  500 * time.Millisecond,
			wantWriteTimeout: 750 * time.Millisecond,
		},
		{
			name: "env variable beats the URL query parameter",
			env: map[string]string{
				"REDIS_URL":          "redis://cache.internal?pool_size=5&read_timeout=1s",
				"REDIS_POOL_SIZE":    "30",
				"REDIS_READ_TIMEOUT": "250ms",
			},
			wantPoolSize:    30,
			wantReadTimeout: 250 * time.Millisecond,
		},
		{
			name: "unset leaves the URL query value",
			env: map[string]string{
				"REDIS_URL": "redis://cache.internal?pool_size=5&min_idle_conns=1&dial_timeout=3s",
			},
			wantPoolSize:     5,
			wantMinIdleConns: 1,
			wantDialTimeout:  3 * time.Second,
		},
		{
			name: "bad or negative values warn and keep the default",
			env: map[string]string{
				"REDIS_HOST":           "localhost",
				"REDIS_POOL_SIZE":      "many",
				"REDIS_MIN_IDLE_CONNS": "-1",
				"REDIS_DIAL_TIMEOUT":   "soon",
				"REDIS_READ_TIMEOUT":   "-1s",
				"REDIS_WRITE_TIMEOUT":  "10",
			},
			wantStdout: []string{
				"REDIS_POOL_SIZE", "REDIS_MIN_IDLE_CONNS", "REDIS_DIAL_TIMEOUT", "REDIS_READ_TIMEOUT", "REDIS_WRITE_TIMEOUT",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			conf, stdout, _ := getRedisConfigWithLogs(t, tc.env)

			for _, want := range tc.wantStdout {
				assert.Contains(t, stdout, want)
			}

			require.NotNil(t, conf.Options)
			assert.Equal(t, tc.wantPoolSize, conf.Options.PoolSize)
			assert.Equal(t, tc.wantMinIdleConns, conf.Options.MinIdleConns)
			assert.Equal(t, tc.wantDialTimeout, conf.Options.DialTimeout)
			assert.Equal(t, tc.wantReadTimeout, conf.Options.ReadTimeout)
			assert.Equal(t, tc.wantWriteTimeout, conf.Options.WriteTimeout)
		})
	}
}
