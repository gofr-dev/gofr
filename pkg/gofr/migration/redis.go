package migration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"gofr.dev/pkg/gofr/container"
)

var errRedisLockRefreshFailed = errors.New("failed to refresh Redis lock: lock lost or stolen")

type redisDS struct {
	Redis
}

func (r redisDS) apply(m migrator) migrator {
	return redisMigrator{
		Redis:    r.Redis,
		migrator: m,
	}
}

type redisMigrator struct {
	Redis

	migrator
}

type redisData struct {
	Method    string    `json:"method"`
	StartTime time.Time `json:"startTime"`
	Duration  int64     `json:"duration"`
}

func (m redisMigrator) getLastMigration(c *container.Container) (int64, error) {
	var lastMigration int64

	table, err := c.Redis.HGetAll(context.Background(), "gofr_migrations").Result()
	if err != nil {
		return -1, fmt.Errorf("redis: %w", err)
	}

	for key, value := range table {
		integerValue, _ := strconv.ParseInt(key, 10, 64)

		if integerValue > lastMigration {
			lastMigration = integerValue
		}

		var data redisData

		err = json.Unmarshal([]byte(value), &data)
		if err != nil {
			return -1, fmt.Errorf("redis: %w", err)
		}
	}

	c.Debugf("Redis last migration fetched value is: %v", lastMigration)

	last, err := m.migrator.getLastMigration(c)
	if err != nil {
		return -1, err
	}

	return max(lastMigration, last), nil
}

func (m redisMigrator) beginTransaction(c *container.Container) transactionData {
	redisTx := c.Redis.TxPipeline()

	cmt := m.migrator.beginTransaction(c)

	cmt.RedisTx = redisTx

	c.Debug("Redis Transaction begin successful")

	return cmt
}

func (m redisMigrator) commitMigration(c *container.Container, data transactionData) error {
	migrationVersion := strconv.FormatInt(data.MigrationNumber, 10)

	if data.UsedDatasources[dsRedis] {
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
	}

	// Always execute the pipeline to avoid leaving dangling transactions.
	_, err := data.RedisTx.Exec(context.Background())
	if err != nil {
		c.Logger.Errorf("migration %v for Redis failed with err: %v", migrationVersion, err)

		return err
	}

	return m.migrator.commitMigration(c, data)
}

func (m redisMigrator) rollback(c *container.Container, data transactionData) {
	data.RedisTx.Discard()

	m.migrator.rollback(c, data)

	c.Fatalf("Migration %v for Redis failed and rolled back", data.MigrationNumber)
}

func (m redisMigrator) lock(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) error {
	for i := 0; ; i++ {
		status, err := c.Redis.SetNX(ctx, lockKey, ownerID, defaultLockTTL).Result()
		if err == nil && status {
			c.Debug("Redis lock acquired successfully")

			go m.startRefresh(ctx, cancel, c, ownerID)

			return m.migrator.lock(ctx, cancel, c, ownerID)
		}

		if err != nil {
			c.Errorf("error while acquiring redis lock: %v", err)

			return errLockAcquisitionFailed
		}

		c.Debugf("Redis lock already held, retrying in %v... (attempt %d)", defaultRetry, i+1)

		select {
		case <-time.After(defaultRetry):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (redisMigrator) startRefresh(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) {
	ticker := time.NewTicker(defaultRefresh)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Use Lua script to ensure we only refresh the lock if we own it
			script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`

			val, err := c.Redis.Eval(ctx, script, []string{lockKey}, ownerID, int(defaultLockTTL.Seconds())).Result()
			if err != nil {
				c.Errorf("failed to refresh Redis lock: %v", err)

				cancel()

				return
			}

			if val == int64(0) {
				c.Errorf("%v", errRedisLockRefreshFailed)

				cancel()

				return
			}

			c.Debug("Redis lock refreshed successfully")
		case <-ctx.Done():
			return
		}
	}
}

func (m redisMigrator) unlock(c *container.Container, ownerID string) error {
	// Use Lua script to ensure we only delete the lock if we own it
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := c.Redis.Eval(context.Background(), script, []string{lockKey}, ownerID).Result()
	if err != nil {
		c.Errorf("unable to release redis lock: %v", err)

		return errLockReleaseFailed
	}

	// Check if the lock was actually deleted (result should be 1)
	deleted, ok := result.(int64)
	if !ok || deleted == 0 {
		c.Errorf("failed to release Redis lock: lock was already released or stolen")
		return errLockReleaseFailed
	}

	c.Debug("Redis lock released successfully")

	return m.migrator.unlock(c, ownerID)
}

func (redisMigrator) name() string {
	return "Redis"
}
