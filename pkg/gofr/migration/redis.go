package migration

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"gofr.dev/pkg/gofr/container"
)

type redisDS struct {
	Redis
}

func (r redisDS) apply(m migrator) migrator {
	return &redisMigrator{
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

func (m *redisMigrator) getLastMigration(c *container.Container) int64 {
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

func (m *redisMigrator) beginTransaction(c *container.Container) transactionData {
	redisTx := c.Redis.TxPipeline()

	cmt := m.migrator.beginTransaction(c)

	cmt.RedisTx = redisTx

	c.Debug("Redis Transaction begin successful")

	return cmt
}

func (m *redisMigrator) commitMigration(c *container.Container, data transactionData) error {
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

func (m *redisMigrator) rollback(c *container.Container, data transactionData) {
	data.RedisTx.Discard()

	m.migrator.rollback(c, data)

	c.Fatalf("Migration %v for Redis failed and rolled back", data.MigrationNumber)
}

func (*redisMigrator) Lock(c *container.Container) error {
	// SETNX gofr_migrations_lock 1 EX 60
	// We use a TTL of 60 seconds to ensure the lock is released if the process crashes.
	ttl := 60 * time.Second
	retryInterval := 500 * time.Millisecond
	maxRetries := 140 // Total wait time ~70 seconds

	for i := 0; i < maxRetries; i++ {
		status, err := c.Redis.SetNX(context.Background(), lockKey, "1", ttl).Result()
		if err == nil && status {
			c.Debug("Redis lock acquired successfully")

			return nil
		}

		if err != nil {
			c.Errorf("error while acquiring redis lock: %v", err)

			return ErrLockAcquisitionFailed
		}

		c.Debugf("Redis lock already held, retrying in %v... (attempt %d/%d)", retryInterval, i+1, maxRetries)
		time.Sleep(retryInterval)
	}

	return ErrLockAcquisitionFailed
}

func (*redisMigrator) Unlock(c *container.Container) error {
	err := c.Redis.Del(context.Background(), lockKey).Err()
	if err != nil {
		c.Errorf("unable to release redis lock: %v", err)

		return ErrLockReleaseFailed
	}

	c.Debug("Redis lock released successfully")

	return nil
}

func (*redisMigrator) Name() string {
	return "Redis"
}
