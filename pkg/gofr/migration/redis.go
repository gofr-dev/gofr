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

type redisMigratorObject struct {
	commands
}

type redisMigrator struct {
	commands

	Migrator
}

func (s redisMigratorObject) apply(m Migrator) Migrator {
	return redisMigrator{
		commands: s.commands,
		Migrator: m,
	}
}

func (d redisMigrator) getLastMigration(c *container.Container) int64 {
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

	lm2 := d.Migrator.getLastMigration(c)
	if lm2 > lastMigration {
		return lm2
	}

	return lastMigration
}

func (d redisMigrator) commitMigration(c *container.Container, data migrationData) error {
	jsonData, err := json.Marshal(migration{
		Method:    "UP",
		StartTime: data.StartTime,
		Duration:  time.Since(data.StartTime).Milliseconds(),
	})
	if err != nil {
		c.Logger.Errorf("migration for Redis %v failed with err: %v", err)

		return err
	}

	migrationVersion := strconv.FormatInt(data.MigrationNumber, 10)

	_, err = data.RedisTx.HSet(context.Background(), "gofr_migrations", map[string]string{migrationVersion: string(jsonData)}).Result()
	if err != nil {
		c.Logger.Errorf("migration for Redis %v failed with err: %v", err)

		return err
	}

	_, err = data.RedisTx.Exec(context.Background())
	if err != nil {
		c.Logger.Errorf("migration for Redis %v failed with err: %v", err)

		return err
	}

	return d.Migrator.commitMigration(c, data)
}

func (d redisMigrator) rollback(c *container.Container, data migrationData) {
	data.RedisTx.Discard()

	d.Migrator.rollback(c, data)
}

func (d redisMigrator) beginTransaction(c *container.Container) migrationData {
	redisTx := c.Redis.TxPipeline()

	cmt := d.Migrator.beginTransaction(c)

	cmt.RedisTx = redisTx

	c.Debug("Redis Transaction begin successful")

	return cmt
}
