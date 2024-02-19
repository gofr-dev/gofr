package migration

import (
	"context"
	"encoding/json"
	"fmt"
	"gofr.dev/pkg/gofr/container"
	"strconv"
	"time"

	goRedis "github.com/redis/go-redis/v9"
)

type migration struct {
	Method    string
	StartTime time.Time
	Duration  int64
}

type redis struct {
	commands
}

func newRedis(c commands) redis {
	return redis{
		commands: c,
	}
}

type commands interface {
	Get(ctx context.Context, key string) *goRedis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *goRedis.StatusCmd
	Del(ctx context.Context, keys ...string) *goRedis.IntCmd
	Rename(ctx context.Context, key, newKey string) *goRedis.StatusCmd
}

func (r redis) Get(ctx context.Context, key string) *goRedis.StringCmd {
	return r.commands.Get(ctx, key)
}

func (r redis) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *goRedis.StatusCmd {
	return r.commands.Set(ctx, key, value, expiration)
}

func (r redis) Del(ctx context.Context, keys ...string) *goRedis.IntCmd {
	return r.commands.Del(ctx, keys...)
}

func (r redis) Rename(ctx context.Context, key, newKey string) *goRedis.StatusCmd {
	return r.commands.Rename(ctx, key, newKey)
}

func redisPostRun(c *container.Container, tx goRedis.Pipeliner, currentMigration int64, start time.Time) {

	data, _ := json.Marshal(migration{
		Method:    "UP",
		StartTime: start,
		Duration:  time.Since(start).Milliseconds(),
	})

	migrationVersion := strconv.FormatInt(currentMigration, 10)

	_, _ = c.Redis.HSet(context.Background(), "gofr_migrations", map[string]string{migrationVersion: string(data)}).Result()

	cmd, err := tx.Exec(context.Background())
	if err != nil {
		c.Logger.Errorf("Migration for Redis %v failed with err : %v", err)
	}

	fmt.Println(cmd)
}
