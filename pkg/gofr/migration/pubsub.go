package migration

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"gofr.dev/pkg/gofr/container"
)

type pubsubDS struct {
	client PubSub
}

const (
	pubsubMigrationTopic = "gofr_migrations"
	migrationTimeout     = 10 * time.Second // Increased timeout
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

func (ds pubsubDS) Query(ctx context.Context, query string, args ...any) ([]byte, error) {
	return ds.client.Query(ctx, query, args...)
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

	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout)
	defer cancel()

	result, err := c.PubSub.Query(ctx, pubsubMigrationTopic, int64(0), 100)
	if err != nil {
		c.Errorf("Error querying migration topic: %v", err)

		return lastVersion
	}

	if len(result) == 0 {
		c.Debug("No previous migrations found - this appears to be the first run")

		return lastVersion
	}

	var records []migrationRecord
	decoder := json.NewDecoder(bytes.NewReader(result))

	for decoder.More() {
		var rec migrationRecord
		if err := decoder.Decode(&rec); err != nil {
			c.Errorf("Error decoding JSON stream: %v", err)
			break
		}
		records = append(records, rec)
	}

	// Process the records
	for _, record := range records {
		if record.Method == "UP" && record.Version > lastVersion {
			lastVersion = record.Version
		}
	}

	c.Debugf("Last completed migration version: %d", lastVersion)

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
