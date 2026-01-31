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

	if len(table) == 0 {
		var lm2 int64

		lm2, err = m.migrator.getLastMigration(c)
		if err != nil {
			return -1, err
		}

		return lm2, nil
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

	if last > lastMigration {
		return last, nil
	}

	return lastMigration, nil
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
