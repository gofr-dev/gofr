package migration

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"gofr.dev/pkg/gofr/container"
)

const (
	pubsubMigrationTopic = "gofr_migrations"
	migrationTimeout     = 10 * time.Second
	defaultQueryLimit    = 100
)

type pubsubDS struct {
	client PubSub
}

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
	return &pubsubMigrator{
		PubSub:   ds,
		migrator: m,
	}
}

func (pm *pubsubMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	err := pm.CreateTopic(context.Background(), pubsubMigrationTopic)
	if err != nil {
		c.Debug("Migration topic might already exist:", err)
	}

	return pm.migrator.checkAndCreateMigrationTable(c)
}

func (pm *pubsubMigrator) getLastMigration(c *container.Container) int64 {
	queryTopic := resolveMigrationTopic(c)

	ctx, cancel := context.WithTimeout(context.Background(), migrationTimeout)
	defer cancel()

	result, err := c.PubSub.Query(ctx, queryTopic, int64(0), defaultQueryLimit)
	if err != nil {
		c.Errorf("Error querying migration topic: %v", err)
	}

	pubsubLastMigration := extractLastVersion(c, result)

	nextMigratorLastMigration := pm.migrator.getLastMigration(c)

	if nextMigratorLastMigration > pubsubLastMigration {
		return nextMigratorLastMigration
	}

	return pubsubLastMigration
}

func resolveMigrationTopic(c *container.Container) string {
	// Check if the PubSub client provides a GetTopicName method
	if topicResolver, ok := c.PubSub.(interface{ GetEventHubName() string }); ok {
		topicName := topicResolver.GetEventHubName()
		if topicName != "" {
			return topicName
		}
	}

	return pubsubMigrationTopic
}

func extractLastVersion(c *container.Container, data []byte) int64 {
	if len(data) == 0 {
		return 0
	}

	lines := bytes.Split(data, []byte("\n"))

	var lastVersion int64

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}

		var rec migrationRecord

		if err := json.Unmarshal(line, &rec); err != nil {
			c.Errorf("Error decoding JSON: %v for line: %s", err, string(line))

			continue
		}

		if rec.Method == "UP" && rec.Version > lastVersion {
			lastVersion = rec.Version
		}
	}

	c.Debugf("Last completed migration version: %d", lastVersion)

	return lastVersion
}

func (pm *pubsubMigrator) commitMigration(c *container.Container, data transactionData) error {
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

	publishTopic := resolveMigrationTopic(c)

	err = c.PubSub.Publish(context.Background(), publishTopic, recordBytes)
	if err != nil {
		return err
	}

	c.Debugf("Inserted record for migration %v in PubSub gofr_migrations topic", data.MigrationNumber)

	return pm.migrator.commitMigration(c, data)
}

func (pm *pubsubMigrator) beginTransaction(c *container.Container) transactionData {
	return pm.migrator.beginTransaction(c)
}

func (pm *pubsubMigrator) rollback(c *container.Container, data transactionData) {
	pm.migrator.rollback(c, data)
}

func (*pubsubMigrator) Lock(*container.Container, string) error {
	return nil
}

func (*pubsubMigrator) Unlock(*container.Container, string) error {
	return nil
}

func (*pubsubMigrator) Refresh(*container.Container, string) error {
	return nil
}

func (pm *pubsubMigrator) Next() migrator {
	return pm.migrator
}

func (*pubsubMigrator) Name() string {
	return "PubSub"
}
