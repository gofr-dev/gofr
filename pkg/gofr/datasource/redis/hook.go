package redis

import (
	"context"
	"time"

	"gofr.dev/pkg/gofr/datasource"

	"github.com/redis/go-redis/v9"
)

type redisHook struct {
	logger datasource.Logger
}

type QueryLog struct {
	Query     string      `json:"query"`
	Duration  int64       `json:"duration"`
	StartTime time.Time   `json:"-"`
	Args      interface{} `json:"args,omitempty"`
}

func (r *redisHook) logQuery(start time.Time, query string, args ...interface{}) {
	r.logger.Debug(QueryLog{
		Query:     query,
		Duration:  time.Since(start).Microseconds(),
		StartTime: start,
		Args:      args,
	})
}

func (r *redisHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (r *redisHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		r.logQuery(start, cmd.Name(), cmd.Args()...)

		return err
	}
}

func (r *redisHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		r.logQuery(start, "pipeline", cmds[:len(cmds)-1])

		return err
	}
}
