package migration

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"gofr.dev/pkg/gofr/container"

	goRedis "github.com/redis/go-redis/v9"
)

type migration struct {
	Method    string    `json:"method"`
	StartTime time.Time `json:"startTime"`
	Duration  int64     `json:"duration"`
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

	_, err := tx.Exec(context.Background())
	if err != nil {
		c.Logger.Errorf("migration for Redis %v failed with err: %v", err)
	}
}

func getRedisLastMigration(c *container.Container) int64 {
	var lastMigration int64

	table, err := c.Redis.HGetAll(context.Background(), "gofr_migrations").Result()
	if err != nil {
		c.Logger.Errorf("failed to get migration record from Redis err: %v", err)

		return -1
	}

	val := make(map[int64]migration)

	for key, value := range table {
		integerValue, _ := strconv.ParseInt(key, 10, 64)

		if integerValue > lastMigration {
			lastMigration = integerValue
		}

		d := []byte(value)

		var migrationData migration

		err = json.Unmarshal(d, &migrationData)
		if err != nil {
			c.Logger.Errorf("failed to unmarshal redis Migration data err: %v", err)

			return -1
		}

		val[integerValue] = migrationData
	}

	return lastMigration
}
