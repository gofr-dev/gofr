package migration

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gofr.dev/pkg/gofr/container"
)

type pubsubDS struct {
	client PubSub
}

const (
	pubsubMigrationTopic = "gofr_migrations"
	migrationTimeout     = 30 * time.Second // Increased timeout
	maxRetries           = 3
)

type migrationRecord struct {
	Version   int64  `json:"version"`
	Method    string `json:"method"`
	StartTime int64  `json:"start_time"`
	Duration  int64  `json:"duration"`
}

// pubsubMigrator struct remains the same but uses our adapter.
type pubsubMigrator struct {
	PubSub
	migrator
}

func (ds pubsubDS) CreateTopic(ctx context.Context, name string) error {
	return ds.client.CreateTopic(ctx, name)
}

func (ds pubsubDS) DeleteTopic(ctx context.Context, name string) error {
	return ds.client.DeleteTopic(ctx, name)
}

func (ds pubsubDS) apply(m migrator) migrator {
	return pubsubMigrator{
		PubSub:   ds,
		migrator: m,
	}
}

func (pm pubsubMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	err := pm.CreateTopic(context.Background(), pubsubMigrationTopic)
	if err != nil {
		c.Debug("Migration topic might already exist:", err)
	}

	return pm.migrator.checkAndCreateMigrationTable(c)
}

func (pm pubsubMigrator) getLastMigration(c *container.Container) int64 {
	var lastVersion int64

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for {
		msg, err := c.PubSub.Subscribe(ctx, pubsubMigrationTopic)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				c.Debug("No new messages in migration topic within timeout")
				break
			}

			c.Debug("Error subscribing to migration topic:", err)
			continue
		}

		var record migrationRecord
		if err := json.Unmarshal(msg.Value, &record); err != nil {
			c.Debug("Error unmarshalling migration record:", err)
			continue
		}

		if lastVersion > 0 {
			c.Debugf("PubSub last migration fetched value is: %v", lastVersion)
		}

		if record.Version > lastVersion {
			lastVersion = record.Version
		}
	}

	return lastVersion
}

func (pm pubsubMigrator) commitMigration(c *container.Container, data transactionData) error {
	record := migrationRecord{
		Version:   data.MigrationNumber,
		Method:    "UP",
		StartTime: data.StartTime.UnixMilli(),
		Duration:  time.Since(data.StartTime).Milliseconds(),
	}

	recordBytes, err := json.Marshal(record)
	if err != nil {
		return err
	}

	err = c.PubSub.Publish(context.Background(), pubsubMigrationTopic, recordBytes)
	if err != nil {
		return err
	}

	c.Debugf("Inserted record for migration %v in PubSub gofr_migrations topic", data.MigrationNumber)
	return pm.migrator.commitMigration(c, data)
}
