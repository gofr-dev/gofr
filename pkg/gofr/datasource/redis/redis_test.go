package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"

	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/testutil"
)

func TestRedis_QueryLogging(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock Redis server setup
	s, err := miniredis.Run()
	assert.Nil(t, err)

	defer s.Close()

	result := testutil.StdoutOutputForFunc(func() {
		mockLogger := testutil.NewMockLogger(testutil.DEBUGLOG)
		client := NewClient(testutil.NewMockConfig(map[string]string{
			"REDIS_HOST": s.Host(),
			"REDIS_PORT": s.Port(),
		}), mockLogger, mockMetrics{})
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

	// Execute Redis pipeline
	result := testutil.StdoutOutputForFunc(func() {
		mockLogger := testutil.NewMockLogger(testutil.DEBUGLOG)
		client := NewClient(testutil.NewMockConfig(map[string]string{
			"REDIS_HOST": s.Host(),
			"REDIS_PORT": s.Port(),
		}), mockLogger, mockMetrics{})
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

type mockMetrics struct {
}

func (m mockMetrics) IncrementCounter(ctx context.Context, name string, labels ...string) {
}

func (m mockMetrics) DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string) {
}

func (m mockMetrics) RecordHistogram(ctx context.Context, name string, value float64, labels ...string) {
}

func (m mockMetrics) SetGauge(name string, value float64) {
}
