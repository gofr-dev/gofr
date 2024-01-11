package datasource

import (
	"context"

	"go.uber.org/mock/gomock"

	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

func Test_QueryLoggerSingleCommand(t *testing.T) {
	ctrl := gomock.NewController(t)

	db, mock := redismock.NewClientMock()
	logger := NewMockLogger(ctrl)

	redisClient := Redis{db, logger, queryLogger{}}

	mock.ExpectSet("test_key", "test_value", 1*time.Minute).SetVal("Success")
	logger.EXPECT().Debug(gomock.Any())

	ctx := context.Background()

	newCtx := redisClient.BeforeRedisCommand(ctx)

	_, err := db.Set(ctx, "test_key", "test_value", 1*time.Minute).Result()
	assert.Nil(t, err, "Redis QueryLogger MockRedis Set Failed!")

	err = redisClient.AfterRedisCommand(newCtx, redis.NewCmd(newCtx, "test_args"))
	assert.Nil(t, err, "Redis QueryLogger AfterRedisCommand Failed!")

	err = mock.ExpectationsWereMet()
	if err != nil {
		t.Error(err)
	}
}

func Test_QueryLoggerPipeline(t *testing.T) {
	ctrl := gomock.NewController(t)

	db, mock := redismock.NewClientMock()
	logger := NewMockLogger(ctrl)

	redisClient := Redis{db, logger, queryLogger{}}

	logger.EXPECT().Debug(gomock.Any())

	ctx := context.Background()

	newCtx, err := redisClient.BeforeProcessPipeline(ctx)
	assert.Nil(t, err, "Redis QueryLogger BeforeProcessPipeline Failed!")

	cmds := []redis.Cmder{
		db.Pipeline().Set(ctx, "test_key_1", "test_string_1", 1*time.Minute),
		db.Pipeline().Set(ctx, "test_key_2", "test_string_2", 1*time.Minute),
	}

	err = redisClient.AfterProcessPipeline(newCtx, cmds)
	assert.Nil(t, err, "Redis QueryLogger AfterProcessPipeline Failed!")

	err = mock.ExpectationsWereMet()
	if err != nil {
		t.Error(err)
	}
}
