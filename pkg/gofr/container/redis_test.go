package container

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redismock/v9"
	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

func Test_QueryLoggerSingleCommand(t *testing.T) {
	db, mock := redismock.NewClientMock()

	mock.ExpectSet("test_key", "test_value", 1*time.Minute).SetVal("Success")

	ctx := context.Background()

	infoLog := testutil.StdoutOutputForFunc(func() {
		logger := logging.NewLogger(logging.DEBUG)

		queryLogger := &queryLogger{Logger: logger}

		newCtx := queryLogger.BeforeRedisCommand(ctx)

		_, err := db.Set(ctx, "test_key", "test_value", 1*time.Minute).Result()
		if err != nil {
			t.Errorf("MockRedis Set Failed!")
		}

		err = queryLogger.AfterRedisCommand(newCtx, redis.NewCmd(newCtx, "test_args"))
		if err != nil {
			t.Errorf("After  Failed!")
		}
	})

	if !(strings.Contains(infoLog, "DEBUG") &&
		strings.Contains(infoLog, "\"message\":\"Redis query: test_args")) {
		t.Errorf("Test Failed")
	}
}
