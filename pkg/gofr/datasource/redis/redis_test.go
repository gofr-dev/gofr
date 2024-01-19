package redis

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"

	"go.uber.org/mock/gomock"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/testutil"
)

func TestRedis_QueryLogging(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock Redis server setup
	s, err := miniredis.Run()
	assert.Nil(t, err)

	defer s.Close()

	// Convert port to integer
	port, err := strconv.Atoi(s.Port())
	assert.Nil(t, err)

	// Config for  miniRedis server
	config := Config{
		HostName: s.Host(),
		Port:     port,
	}

	result := testutil.StdoutOutputForFunc(func() {
		mockLogger := datasource.NewMockLogger(0)
		client, err := NewClient(config, mockLogger)
		assert.Nil(t, err)

		result, err := client.Set(context.TODO(), "key", "value", 1*time.Minute).Result()
		assert.Nil(t, err)
		assert.Equal(t, "OK", result)
	})

	// Assertions
	assert.Contains(t, result, "[ping]")
	assert.Contains(t, result, "[set key value ex 60]")
}

func TestRedis_PipelineQueryLogging(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Mock Redis server setup
	s, err := miniredis.Run()
	assert.Nil(t, err)

	s.Close()

	// Convert port to integer
	port, err := strconv.Atoi(s.Port())
	assert.Nil(t, err)

	// Config for Redis client
	config := Config{
		HostName: s.Host(),
		Port:     port,
	}

	// Execute Redis pipeline
	result := testutil.StdoutOutputForFunc(func() {
		mockLogger := datasource.NewMockLogger(0)
		client, err := NewClient(config, mockLogger)
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
	assert.Contains(t, result, "[ping]")
	assert.Contains(t, result, "pipeline")
	assert.Contains(t, result, "[set key1 value1 ex 60: OK]")
}
