package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"

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

	mockMetrics.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(), "hostname", gomock.Any(), "type", "ping")

	client := NewClient(mockConfig, mockLogger, mockMetrics)
	assert.Nil(t, client.Client, "Test_NewClient_InvalidPort Failed! Expected redis client to be nil")
}

func TestRedis_QueryLogging(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock Redis server setup
	s, err := miniredis.Run()
	assert.Nil(t, err)

	defer s.Close()

	mockMetric := NewMockMetrics(ctrl)
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(), "hostname", gomock.Any(), "type", "ping")
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(), "hostname", gomock.Any(), "type", "set")

	result := testutil.StdoutOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.DEBUG)
		client := NewClient(config.NewMockConfig(map[string]string{
			"REDIS_HOST": s.Host(),
			"REDIS_PORT": s.Port(),
		}), mockLogger, mockMetric)

		assert.Nil(t, err)

		result, err := client.Set(context.TODO(), "key", "value", 1*time.Minute).Result()

		assert.Nil(t, err)
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
	assert.Nil(t, err)

	defer s.Close()

	mockMetric := NewMockMetrics(ctrl)
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(), "hostname", gomock.Any(), "type", "ping")
	mockMetric.EXPECT().RecordHistogram(gomock.Any(), "app_redis_stats", gomock.Any(), "hostname", gomock.Any(), "type", "pipeline")

	// Execute Redis pipeline
	result := testutil.StdoutOutputForFunc(func() {
		mockLogger := logging.NewMockLogger(logging.DEBUG)
		client := NewClient(config.NewMockConfig(map[string]string{
			"REDIS_HOST": s.Host(),
			"REDIS_PORT": s.Port(),
		}), mockLogger, mockMetric)

		assert.Nil(t, err)

		// Pipeline execution
		pipe := client.Pipeline()
		setCmd := pipe.Set(context.TODO(), "key1", "value1", 1*time.Minute)
		getCmd := pipe.Get(context.TODO(), "key1")

		// Pipeline Exec should return a non-nil error
		_, err = pipe.Exec(context.TODO())
		assert.Nil(t, err)

		// Retrieve results
		setResult, err := setCmd.Result()
		assert.Nil(t, err)
		assert.Equal(t, "OK", setResult)

		getResult, err := getCmd.Result()
		assert.Nil(t, err)
		assert.Equal(t, "value1", getResult)
	})

	// Assertions
	assert.Contains(t, result, "ping")
	assert.Contains(t, result, "set key1 value1 ex 60: OK")
}
