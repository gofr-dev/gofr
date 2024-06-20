package migration

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	goRedis "github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/container"
)

type redis struct {
	Redis
}

type redisData struct {
	Method    string    `json:"method"`
	StartTime time.Time `json:"startTime"`
	Duration  int64     `json:"duration"`
}

func newRedis(r Redis) redis {
	return redis{
		Redis: r,
	}
}

func (r redis) Get(ctx context.Context, key string) *goRedis.StringCmd {
	return r.Redis.Get(ctx, key)
}

func (r redis) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *goRedis.StatusCmd {
	return r.Redis.Set(ctx, key, value, expiration)
}

func (r redis) Del(ctx context.Context, keys ...string) *goRedis.IntCmd {
	return r.Redis.Del(ctx, keys...)
}

func (r redis) Rename(ctx context.Context, key, newKey string) *goRedis.StatusCmd {
	return r.Redis.Rename(ctx, key, newKey)
}

func (r redis) Apply(m Manager) Manager {
	return redisMigrator{
		Redis:   r.Redis,
		Manager: m,
	}
}

type redisMigrator struct {
	Redis

	Manager
}

func (m redisMigrator) GetLastMigration(c *container.Container) int64 {
	var lastMigration int64

	table, err := c.Redis.HGetAll(context.Background(), "gofr_migrations").Result()
	if err != nil {
		c.Logger.Errorf("failed to get migration record from Redis. err: %v", err)

		return -1
	}

	val := make(map[int64]redisData)

	for key, value := range table {
		integerValue, _ := strconv.ParseInt(key, 10, 64)

		if integerValue > lastMigration {
			lastMigration = integerValue
		}

		d := []byte(value)

		var data redisData

		err = json.Unmarshal(d, &data)
		if err != nil {
			c.Logger.Errorf("failed to unmarshal redis Migration data err: %v", err)

			return -1
		}

		val[integerValue] = data
	}

	c.Debugf("Redis last migration fetched value is: %v", lastMigration)

	last := m.Manager.GetLastMigration(c)
	if last > lastMigration {
		return last
	}

	return lastMigration
}

func (m redisMigrator) BeginTransaction(c *container.Container) transactionData {
	redisTx := c.Redis.TxPipeline()

	cmt := m.Manager.BeginTransaction(c)

	cmt.RedisTx = redisTx

	c.Debug("Redis Transaction begin successful")

	return cmt
}

func (m redisMigrator) CommitMigration(c *container.Container, data transactionData) error {
	migrationVersion := strconv.FormatInt(data.MigrationNumber, 10)

	jsonData, err := json.Marshal(redisData{
		Method:    "UP",
		StartTime: data.StartTime,
		Duration:  time.Since(data.StartTime).Milliseconds(),
	})
	if err != nil {
		c.Logger.Errorf("migration %v for Redis failed with err: %v", migrationVersion, err)

		return err
	}

	_, err = data.RedisTx.HSet(context.Background(), "gofr_migrations", map[string]string{migrationVersion: string(jsonData)}).Result()
	if err != nil {
		c.Logger.Errorf("migration %v for Redis failed with err: %v", migrationVersion, err)

		return err
	}

	_, err = data.RedisTx.Exec(context.Background())
	if err != nil {
		c.Logger.Errorf("migration %v for Redis failed with err: %v", migrationVersion, err)

		return err
	}

	return m.Manager.CommitMigration(c, data)
}

func (m redisMigrator) Rollback(c *container.Container, data transactionData) {
	data.RedisTx.Discard()

	c.Errorf("Migration %v for Redis failed and rolled back", data.MigrationNumber)
	m.Manager.Rollback(c, data)
}
