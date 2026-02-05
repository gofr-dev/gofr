package migration

import (
	"context"
	"fmt"
	"time"

	"gofr.dev/pkg/gofr/container"
)

type mongoDS struct {
	container.Mongo
}

type mongoMigrator struct {
	container.Mongo
	migrator
}

// apply initializes mongoMigrator using the Mongo interface.
func (ds mongoDS) apply(m migrator) migrator {
	return mongoMigrator{
		Mongo:    ds.Mongo,
		migrator: m,
	}
}

const (
	mongoMigrationCollection = "gofr_migrations"
)

// checkAndCreateMigrationTable initializes a MongoDB collection if it doesn't exist.
func (mg mongoMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	err := mg.Mongo.CreateCollection(context.Background(), mongoMigrationCollection)
	if err != nil {
		c.Debug("Migration collection might already exist:", err)

		return err
	}

	return mg.migrator.checkAndCreateMigrationTable(c)
}

func (mg mongoMigrator) getLastMigration(c *container.Container) (int64, error) {
	var (
		lastMigration int64
		migrations    []struct {
			Version int64 `bson:"version"`
		}
	)

	filter := make(map[string]any)

	err := mg.Mongo.Find(context.Background(), mongoMigrationCollection, filter, &migrations)
	if err != nil {
		return -1, fmt.Errorf("mongo: %w", err)
	}

	// Identify the highest migration version.
	for _, migration := range migrations {
		lastMigration = max(lastMigration, migration.Version)
	}

	c.Debugf("MongoDB last migration fetched value is: %v", lastMigration)

	lm2, err := mg.migrator.getLastMigration(c)
	if err != nil {
		return -1, err
	}

	return max(lastMigration, lm2), nil
}

func (mg mongoMigrator) beginTransaction(c *container.Container) transactionData {
	return mg.migrator.beginTransaction(c)
}

func (mg mongoMigrator) commitMigration(c *container.Container, data transactionData) error {
	migrationDoc := map[string]any{
		"version":    data.MigrationNumber,
		"method":     "UP",
		"start_time": data.StartTime,
		"duration":   time.Since(data.StartTime).Milliseconds(),
	}

	_, err := mg.Mongo.InsertOne(context.Background(), mongoMigrationCollection, migrationDoc)
	if err != nil {
		return err
	}

	c.Debugf("Inserted record for migration %v in MongoDB gofr_migrations collection", data.MigrationNumber)

	return mg.migrator.commitMigration(c, data)
}

func (mg mongoMigrator) rollback(c *container.Container, data transactionData) {
	mg.migrator.rollback(c, data)
	c.Fatalf("Migration %v failed.", data.MigrationNumber)
}

func (mg mongoMigrator) lock(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) error {
	return mg.migrator.lock(ctx, cancel, c, ownerID)
}

func (mg mongoMigrator) unlock(c *container.Container, ownerID string) error {
	return mg.migrator.unlock(c, ownerID)
}

func (mongoMigrator) name() string {
	return "Mongo"
}
