package redis

import (
	"crypto/tls"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
)

func TestGetRedisConfig_Defaults(t *testing.T) {
	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockConfig := config.NewMockConfig(map[string]string{
		"PUBSUB_BACKEND": "REDIS", // Required to trigger PubSub config parsing
		"REDIS_HOST":     "localhost",
	})

	conf := getRedisConfig(mockConfig, mockLogger)

	assert.Equal(t, "localhost", conf.HostName)
	assert.Equal(t, defaultRedisPort, conf.Port)
	assert.Equal(t, 0, conf.DB)
	assert.Nil(t, conf.TLS)
	// PubSubStreamsConfig is initialized when mode is streams (default)
	assert.NotNil(t, conf.PubSubStreamsConfig)
	assert.Equal(t, "streams", conf.PubSubMode) // Default mode is now streams
}

func TestGetRedisConfig_InvalidPortAndDB(t *testing.T) {
	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockConfig := config.NewMockConfig(map[string]string{
		"REDIS_HOST": "localhost",
		"REDIS_PORT": "invalid",
		"REDIS_DB":   "invalid",
	})

	conf := getRedisConfig(mockConfig, mockLogger)

	assert.Equal(t, defaultRedisPort, conf.Port)
	assert.Equal(t, 0, conf.DB)
}

func TestGetRedisConfig_TLS(t *testing.T) {
	// Create temporary cert files
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

	// Write dummy content (not valid PEM, but enough to trigger file read)
	_, _ = certFile.WriteString("-----BEGIN CERTIFICATE-----\nMIID\n-----END CERTIFICATE-----")
	_, _ = keyFile.WriteString("-----BEGIN PRIVATE KEY-----\nMIIE\n-----END PRIVATE KEY-----")
	_, _ = caFile.WriteString("-----BEGIN CERTIFICATE-----\nMIID\n-----END CERTIFICATE-----")

	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockConfig := config.NewMockConfig(map[string]string{
		"REDIS_HOST":        "localhost",
		"REDIS_TLS_ENABLED": "true",
		"REDIS_TLS_CERT":    certFile.Name(),
		"REDIS_TLS_KEY":     keyFile.Name(),
		"REDIS_TLS_CA_CERT": caFile.Name(),
	})

	// This will log errors because dummy content is not valid PEM, but it tests the path
	conf := getRedisConfig(mockConfig, mockLogger)

	assert.NotNil(t, conf.TLS)
	assert.Equal(t, uint16(tls.VersionTLS12), conf.TLS.MinVersion)
}

func TestGetRedisConfig_TLS_InvalidFiles(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logging.NewMockLogger(logging.ERROR)

	mockConfig := config.NewMockConfig(map[string]string{
		"REDIS_HOST":        "localhost",
		"REDIS_TLS_ENABLED": "true",
		"REDIS_TLS_CERT":    "nonexistent_cert.pem",
		"REDIS_TLS_KEY":     "nonexistent_key.pem",
		"REDIS_TLS_CA_CERT": "nonexistent_ca.pem",
	})

	conf := getRedisConfig(mockConfig, mockLogger)

	assert.NotNil(t, conf.TLS)
	// Should be empty as files failed to load
	assert.Empty(t, conf.TLS.Certificates)
	assert.Nil(t, conf.TLS.RootCAs)
}

func TestGetRedisConfig_PubSubStreams(t *testing.T) {
	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockConfig := config.NewMockConfig(map[string]string{
		"PUBSUB_BACKEND":               "REDIS",
		"REDIS_HOST":                   "localhost",
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "mygroup",
		"REDIS_STREAMS_CONSUMER_NAME":  "myconsumer",
		"REDIS_STREAMS_MAXLEN":         "1000",
		"REDIS_STREAMS_BLOCK_TIMEOUT":  "2s",
	})

	conf := getRedisConfig(mockConfig, mockLogger)

	assert.Equal(t, "streams", conf.PubSubMode)

	if assert.NotNil(t, conf.PubSubStreamsConfig) {
		assert.Equal(t, "mygroup", conf.PubSubStreamsConfig.ConsumerGroup)
		assert.Equal(t, "myconsumer", conf.PubSubStreamsConfig.ConsumerName)
		assert.Equal(t, int64(1000), conf.PubSubStreamsConfig.MaxLen)
		assert.Equal(t, 2*time.Second, conf.PubSubStreamsConfig.Block)
	}
}

func TestGetRedisConfig_PubSubStreams_Defaults(t *testing.T) {
	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockConfig := config.NewMockConfig(map[string]string{
		"PUBSUB_BACKEND":               "REDIS",
		"REDIS_HOST":                   "localhost",
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "mygroup",
	})

	conf := getRedisConfig(mockConfig, mockLogger)

	assert.Equal(t, "streams", conf.PubSubMode)

	if assert.NotNil(t, conf.PubSubStreamsConfig) {
		assert.Equal(t, "mygroup", conf.PubSubStreamsConfig.ConsumerGroup)
		assert.Empty(t, conf.PubSubStreamsConfig.ConsumerName)
		assert.Equal(t, int64(0), conf.PubSubStreamsConfig.MaxLen)
		assert.Equal(t, 5*time.Second, conf.PubSubStreamsConfig.Block) // Default block
	}
}

func TestGetRedisConfig_PubSubStreams_InvalidValues(t *testing.T) {
	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockConfig := config.NewMockConfig(map[string]string{
		"PUBSUB_BACKEND":               "REDIS",
		"REDIS_HOST":                   "localhost",
		"REDIS_PUBSUB_MODE":            "streams",
		"REDIS_STREAMS_CONSUMER_GROUP": "mygroup",
		"REDIS_STREAMS_MAXLEN":         "invalid",
		"REDIS_STREAMS_BLOCK_TIMEOUT":  "invalid",
	})

	conf := getRedisConfig(mockConfig, mockLogger)

	assert.Equal(t, "streams", conf.PubSubMode)

	if assert.NotNil(t, conf.PubSubStreamsConfig) {
		// Should use defaults
		assert.Equal(t, int64(0), conf.PubSubStreamsConfig.MaxLen)
		assert.Equal(t, 5*time.Second, conf.PubSubStreamsConfig.Block)
	}
}
