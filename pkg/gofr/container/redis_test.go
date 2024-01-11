package container

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_QueryLoggerSingleCommand(t *testing.T) {
	db, mock := redismock.NewClientMock()

	mock.ExpectSet("test_key", "test_value", 1*time.Minute).SetVal("Success")

	ctx := context.Background()

	outputLog := testutil.StdoutOutputForFunc(func() {
		logger := logging.NewLogger(logging.DEBUG)

		queryLogger := &queryLogger{Logger: logger}

		newCtx := queryLogger.BeforeRedisCommand(ctx)

		_, err := db.Set(ctx, "test_key", "test_value", 1*time.Minute).Result()
		assert.Nil(t, err, "Redis QueryLogger MockRedis Set Failed!")

		err = queryLogger.AfterRedisCommand(newCtx, redis.NewCmd(newCtx, "test_args"))
		assert.Nil(t, err, "Redis QueryLogger AfterRedisCommand Failed!")
	})

	assert.Contains(t, outputLog, "DEBUG", "Test_QueryLoggerSingleCommand Failed")
	assert.Contains(t, outputLog, "\"message\":\"Redis query: test_args",
		"Test_QueryLoggerSingleCommand Failed")
}

func Test_QueryLoggerPipeline(t *testing.T) {
	db, mock := redismock.NewClientMock()

	mock.ExpectTxPipelineExec().SetVal([]interface{}{"test_string_1", "test_string_2"})

	ctx := context.Background()

	outputLog := testutil.StdoutOutputForFunc(func() {
		logger := logging.NewLogger(logging.DEBUG)

		queryLogger := &queryLogger{Logger: logger}

		newCtx, err := queryLogger.BeforeProcessPipeline(ctx)
		assert.Nil(t, err, "Redis QueryLogger BeforeProcessPipeline Failed!")

		cmds := []redis.Cmder{
			db.Pipeline().Set(ctx, "test_key_1", "test_string_1", 1*time.Minute),
			db.Pipeline().Set(ctx, "test_key_2", "test_string_2", 1*time.Minute),
		}

		err = queryLogger.AfterProcessPipeline(newCtx, cmds)
		assert.Nil(t, err, "Redis QueryLogger AfterProcessPipeline Failed!")
	})

	output := "Redis pipeline: 2 queries : [ set test_key_1 test_string_1 ex 60 , set test_key_2 test_string_2 ex 60 ]"
	assert.Contains(t, outputLog, output)
}
