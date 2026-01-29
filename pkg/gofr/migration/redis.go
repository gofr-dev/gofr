package migration

import (
	"context"
	"encoding/json"
	"fmt"

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
		ownerID:  fmt.Sprintf("%s-%d", getHostname(), time.Now().UnixNano()),
	}
}

type redisMigrator struct {
	Redis

	migrator
	ownerID string
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

	if len(table) == 0 {
		lm2 := m.migrator.getLastMigration(c)
		if lm2 == -1 {
			return -1
		}
		return lm2
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
	if last == -1 {
		return -1
	}

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

func (m *redisMigrator) rollback(c *container.Container, data transactionData) {
	data.RedisTx.Discard()

	m.migrator.rollback(c, data)

	c.Fatalf("Migration %v for Redis failed and rolled back", data.MigrationNumber)
}

const (
	redisLockKey = "gofr_migration_lock"
	redisLockTTL = 15 * time.Second
)

func (m *redisMigrator) lock(c *container.Container) error {
	for {
		// Try to acquire lock
		success, err := c.Redis.SetNX(context.Background(), redisLockKey, m.ownerID, redisLockTTL).Result()
		if err != nil {
			return err
		}

		if success {
			c.Debugf("Acquired Redis migration lock with ownerID: %s", m.ownerID)
			return nil
		}

		// Lock held by someone else, check if it's me (re-entrant? no, just wait)
		// Or check if expired? Redis handles expiration.
		// Just wait.
		c.Infof("Redis migration lock held, waiting...")
		time.Sleep(2 * time.Second)
	}
}

func (m *redisMigrator) unlock(c *container.Container) {
	// Only delete if we own it
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`
	_, err := c.Redis.Eval(context.Background(), script, []string{redisLockKey}, m.ownerID).Result()
	if err != nil {
		c.Errorf("failed to release Redis migration lock: %v", err)
	} else {
		c.Debugf("Released Redis migration lock for ownerID: %s", m.ownerID)
	}

	m.migrator.unlock(c)
}

func (m *redisMigrator) refreshLock(c *container.Container) error {
	// Only refresh if we own it
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("expire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`
	res, err := c.Redis.Eval(context.Background(), script, []string{redisLockKey}, m.ownerID, int(redisLockTTL.Seconds())).Result()
	if err != nil {
		return err
	}

	if res == int64(0) {
		return fmt.Errorf("failed to refresh Redis lock, lock lost or stolen")
	}

	c.Debugf("Refreshed Redis migration lock for ownerID: %s", m.ownerID)
	return nil
}
