package redis

import (
	"context"
	"os"
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

	// Set the necessary env
	t.Setenv("REDIS_HOST", s.Host())
	t.Setenv("REDIS_PORT", s.Port())

	defer s.Close()

	result := testutil.StdoutOutputForFunc(func() {
		mockLogger := testutil.NewMockLogger(testutil.DEBUGLOG)
		client := NewClient(&mockConfig{}, mockLogger)
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

	// Set the necessary env
	t.Setenv("REDIS_HOST", s.Host())
	t.Setenv("REDIS_PORT", s.Port())

	defer s.Close()

	// Execute Redis pipeline
	result := testutil.StdoutOutputForFunc(func() {
		mockLogger := testutil.NewMockLogger(testutil.DEBUGLOG)
		client := NewClient(&mockConfig{}, mockLogger)
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

type mockConfig struct {
}

func (m *mockConfig) Get(s string) string {
	return os.Getenv(s)
}

func (m *mockConfig) GetOrDefault(s, d string) string {
	res := os.Getenv(s)
	if res == "" {
		res = d
	}

	return res
}
