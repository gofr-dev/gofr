package migration

import (
	"context"
	"encoding/json"
	"errors"
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

func (m redisMigrator) getLastMigration(c *container.Container) int64 {
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

	last := m.migrator.getLastMigration(c)
	if last > lastMigration {
		return last
	}

	return lastMigration
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

	return m.migrator.commitMigration(c, data)
}

func (m redisMigrator) rollback(c *container.Container, data transactionData) {
	data.RedisTx.Discard()

	m.migrator.rollback(c, data)

	c.Fatalf("Migration %v for Redis failed and rolled back", data.MigrationNumber)
}

func (m redisMigrator) lock(c *container.Container, ownerID string, stop <-chan struct{}, fail chan<- error) error {
	for i := 0; ; i++ {
		status, err := c.Redis.SetNX(context.Background(), lockKey, ownerID, defaultLockTTL).Result()
		if err == nil && status {
			c.Debug("Redis lock acquired successfully")

			// Start refresh goroutine
			go m.startRefresh(c, ownerID, stop, fail)

			return m.migrator.lock(c, ownerID, stop, fail)
		}

		if err != nil {
			c.Errorf("error while acquiring redis lock: %v", err)

			return errLockAcquisitionFailed
		}

		c.Debugf("Redis lock already held, retrying in %v... (attempt %d)", defaultRetry, i+1)
		time.Sleep(defaultRetry)
	}
}

func (redisMigrator) startRefresh(c *container.Container, ownerID string, stop <-chan struct{}, fail chan<- error) {
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

			val, err := c.Redis.Eval(context.Background(), script, []string{lockKey}, ownerID, int(defaultLockTTL.Seconds())).Result()
			if err != nil {
				c.Errorf("failed to refresh Redis lock: %v", err)

				fail <- err

				return
			}

			if val == int64(0) {
				c.Errorf("%v", errRedisLockRefreshFailed)

				fail <- errRedisLockRefreshFailed

				return
			}

			c.Debug("Redis lock refreshed successfully")
		case <-stop:
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

	_, err := c.Redis.Eval(context.Background(), script, []string{lockKey}, ownerID).Result()
	if err != nil {
		c.Errorf("unable to release redis lock: %v", err)

		return errLockReleaseFailed
	}

	c.Debug("Redis lock released successfully")

	return m.migrator.unlock(c, ownerID)
}

func (redisMigrator) name() string {
	return "Redis"
}
