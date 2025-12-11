package redis

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_NewClient_HostNameMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockMetrics := NewMockMetrics(ctrl)
	mockConfig := config.NewMockConfig(map[string]string{"REDIS_HOST": ""})

	client := NewClient(mockConfig, mockLogger, mockMetrics)
	assert.Nil(t, client, "Test_NewClient_HostNameMissing Failed! Expected redis client to be nil")
}

func Test_NewClient_InvalidPort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockMetrics := NewMockMetrics(ctrl)
	mockConfig := config.NewMockConfig(map[string]string{"REDIS_HOST": "localhost", "REDIS_PORT": "&&^%%^&*"})

	// Redis client may send "hello" (RESP3 handshake) or "ping" during connection
	// Allow any type of call since we're just verifying the client object is created
	mockMetrics.EXPECT().RecordHistogram(
		gomock.Any(), "app_redis_stats", gomock.Any(),
		"hostname", gomock.Any(), "type", gomock.Any(),
	).AnyTimes()

	client := NewClient(mockConfig, mockLogger, mockMetrics)
	assert.NotNil(t, client.Client, "Test_NewClient_InvalidPort Failed! Expected redis client not to be nil")
}

func TestRedis_QueryLogging(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock Redis server setup
	s, err := miniredis.Run()
	require.NoError(t, err)

	defer s.Close()

	mockMetric := NewMockMetrics(ctrl)
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(),
		"hostname", gomock.Any(), "type", gomock.Any()).AnyTimes()

	result := testutil.StdoutOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.DEBUG)
		client := NewClient(config.NewMockConfig(map[string]string{
			"REDIS_HOST": s.Host(),
			"REDIS_PORT": s.Port(),
			"REDIS_DB":   "1",
		}), mockLogger, mockMetric)

		require.NoError(t, err)

		result, err := client.Set(t.Context(), "key", "value", 1*time.Minute).Result()

		require.NoError(t, err)
		assert.Equal(t, "OK", result)
	})

	// Assertions
	assert.Contains(t, result, "ping")
	assert.Contains(t, result, "set key value ex 60")
}

func TestRedis_PipelineQueryLogging(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock Redis server setup
	s, err := miniredis.Run()
	require.NoError(t, err)

	defer s.Close()

	mockMetric := NewMockMetrics(ctrl)
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(),
		"hostname", gomock.Any(), "type", gomock.Any()).AnyTimes()

	// Execute Redis pipeline
	result := testutil.StdoutOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.DEBUG)
		client := NewClient(config.NewMockConfig(map[string]string{
			"REDIS_HOST": s.Host(),
			"REDIS_PORT": s.Port(),
		}), mockLogger, mockMetric)

		require.NoError(t, err)

		// Pipeline execution
		pipe := client.Pipeline()
		setCmd := pipe.Set(t.Context(), "key1", "value1", 1*time.Minute)
		getCmd := pipe.Get(t.Context(), "key1")

		// Pipeline Exec should return a non-nil error
		_, err = pipe.Exec(t.Context())
		require.NoError(t, err)

		// Retrieve results
		setResult, err := setCmd.Result()
		require.NoError(t, err)
		assert.Equal(t, "OK", setResult)

		getResult, err := getCmd.Result()
		require.NoError(t, err)
		assert.Equal(t, "value1", getResult)
	})

	// Assertions
	assert.Contains(t, result, "ping")
	assert.Contains(t, result, "set key1 value1 ex 60: OK")
}

func TestRedis_Close(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock Redis server setup
	s, err := miniredis.Run()
	require.NoError(t, err)

	defer s.Close()

	// Mock metrics setup
	mockMetric := NewMockMetrics(ctrl)
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(), "hostname",
		gomock.Any(), "type", gomock.Any()).AnyTimes()

	mockLogger := logging.NewMockLogger(logging.DEBUG)
	client := NewClient(config.NewMockConfig(map[string]string{
		"REDIS_HOST": s.Host(),
		"REDIS_PORT": s.Port(),
	}), mockLogger, mockMetric)

	err = client.Close()

	require.NoError(t, err)
}

func Test_TLSConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockConfig := config.NewMockConfig(map[string]string{
		"REDIS_HOST":        "localhost",
		"REDIS_TLS_ENABLED": "true",
	})

	conf := getRedisConfig(mockConfig, mockLogger)
	assert.NotNil(t, conf.TLS, "Expected TLS config to be set")
	assert.EqualValues(t, tls.VersionTLS12, conf.TLS.MinVersion, "Expected TLS 1.2")
}

func Test_TLSConfigWithDummyPEM(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockPEM := getMockPEM()
	mockKey := getMockKey()

	mockLogger := logging.NewMockLogger(logging.ERROR)
	mockConfig := config.NewMockConfig(map[string]string{
		"REDIS_HOST":        "localhost",
		"REDIS_TLS_ENABLED": "true",
		"REDIS_TLS_CA_CERT": mockPEM,
		"REDIS_TLS_CERT":    mockPEM,
		"REDIS_TLS_KEY":     mockKey,
	})

	conf := getRedisConfig(mockConfig, mockLogger)
	assert.NotNil(t, conf.TLS, "Expected TLS config to be set")
	assert.EqualValues(t, tls.VersionTLS12, conf.TLS.MinVersion, "Expected TLS 1.2")
}

func getMockPEM() string {
	const mockPEM = `-----BEGIN CERTIFICATE-----\nMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAnzQw\n-----END CERTIFICATE-----`

	return mockPEM
}

func getMockKey() string {
	//nolint:gosec // dummy private key for test only, not used in production
	const mockKey = `-----BEGIN RSA PRIVATE KEY-----\nMIIEpAIBAAKCAQEAnzQw\n-----END RSA PRIVATE KEY-----`

	return mockKey
}
