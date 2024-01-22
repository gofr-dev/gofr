package redis

import (
	"context"
	"time"

	"gofr.dev/pkg/gofr/datasource"

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

// logQuery logs the Redis query information.
func (r *redisHook) logQuery(start time.Time, query string, args ...interface{}) {
	r.logger.Debug(QueryLog{
		Query:    query,
		Duration: time.Since(start).Microseconds(),
		Args:     args,
	})
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
