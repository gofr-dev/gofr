package migration

import (
	"context"
	"gofr.dev/pkg/gofr/container"
	"time"

	goRedis "github.com/redis/go-redis/v9"
)

type migration struct {
	Version   int64
	StartTime time.Time
	Duration  int64
}

type redis struct {
	commands
	usageTracker
}

func newRedis(c commands, s usageTracker) redis {
	return redis{
		commands:     c,
		usageTracker: s,
	}
}

type commands interface {
	Get(ctx context.Context, key string) *goRedis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *goRedis.StatusCmd
	Del(ctx context.Context, keys ...string) *goRedis.IntCmd
	Rename(ctx context.Context, key, newKey string) *goRedis.StatusCmd
}

func (r redis) Get(ctx context.Context, key string) *goRedis.StringCmd {
	r.set()
	return r.commands.Get(ctx, key)
}

func (r redis) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *goRedis.StatusCmd {
	r.set()
	return r.commands.Set(ctx, key, value, expiration)
}

func (r redis) Del(ctx context.Context, keys ...string) *goRedis.IntCmd {
	r.set()
	return r.commands.Del(ctx, keys...)
}

func (r redis) Rename(ctx context.Context, key, newKey string) *goRedis.StatusCmd {
	r.set()
	return r.commands.Rename(ctx, key, newKey)
}

func redisPostRun(c *container.Container, tx goRedis.Pipeliner, currentMigration int64, start time.Time, s usageTracker) {
	tx.HSet(context.Background(), "gofr_migrations", tx.SetArgs(), string(migBytes))

}
