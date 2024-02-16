package redis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/metrics"

	"github.com/redis/go-redis/v9"
)

// redisHook is a custom Redis hook for logging queries and their durations.
type redisHook struct {
	logger datasource.Logger
}

// QueryLog represents a logged Redis query.
type QueryLog struct {
	Query    string      `json:"query"`
	Duration int64       `json:"duration"`
	Args     interface{} `json:"args,omitempty"`
}

func (ql QueryLog) String() string {
	if ql.Args == nil {
		return ""
	}

	switch args := ql.Args.(type) {
	case []interface{}:
		strArgs := make([]string, len(args))
		for i, arg := range args {
			strArgs[i] = fmt.Sprint(arg)
		}

		return strings.Join(strArgs, " ")
	default:
		return fmt.Sprint(ql.Args)
	}
}

// logQuery logs the Redis query information.
func (r *redisHook) logQuery(start time.Time, query string, args ...interface{}) {
	duration := time.Since(start)

	r.logger.Debug(QueryLog{
		Query:    query,
		Duration: duration.Microseconds(),
		Args:     args,
	})

	metrics.GetMetricsManager().RecordHistogram(context.Background(), "app_redis_stats",
		duration.Seconds(), "type", query)
}

// DialHook implements the redis.DialHook interface.
func (r *redisHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

// ProcessHook implements the redis.ProcessHook interface.
func (r *redisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		r.logQuery(start, cmd.Name(), cmd.Args()...)

		return err
	}
}

// ProcessPipelineHook implements the redis.ProcessPipelineHook interface.
func (r *redisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		r.logQuery(start, "pipeline", cmds[:len(cmds)-1])

		return err
	}
}
