package migration

import (
	"context"
	"fmt"
	"strings"
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
	mongoLockCollection      = "migration_locks"
	mongoLockDocumentID      = "migration_lock"
)

func isMongoCollectionExistsError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())

	return strings.Contains(msg, "already exists") || strings.Contains(msg, "namespaceexists")
}

// checkAndCreateMigrationTable initializes a MongoDB collection if it doesn't exist.
func (mg mongoMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	err := mg.Mongo.CreateCollection(context.Background(), mongoMigrationCollection)

	if err != nil && !isMongoCollectionExistsError(err) {
		return err
	}

	err = mg.Mongo.CreateCollection(context.Background(), mongoLockCollection)
	if err != nil && !isMongoCollectionExistsError(err) {
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

func (mg mongoMigrator) startRefresh(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) {
	ticker := time.NewTicker(defaultRefresh)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()

			filter := map[string]any{
				"_id":      mongoLockDocumentID,
				"lockedBy": ownerID,
			}

			update := map[string]any{
				"$set": map[string]any{
					"lockedAt":  now,
					"expiresAt": now.Add(defaultLockTTL),
				},
			}

			if err := mg.Mongo.UpdateOne(ctx, mongoLockCollection, filter, update); err != nil {
				c.Errorf("failed to refresh mongo lock: %v", err)
				cancel()

				return
			}

			c.Debug("Mongo lock refreshed successfully")
		case <-ctx.Done():
			return
		}
	}
}

func (mg mongoMigrator) lock(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) error {
	for i := 0; ; i++ {
		now := time.Now()

		staleFilter := map[string]any{
			"_id": mongoLockDocumentID,
			"expiresAt": map[string]any{
				"$lte": now,
			},
		}

		if _, err := mg.Mongo.DeleteOne(ctx, mongoLockCollection, staleFilter); err != nil {
			c.Errorf("failed to cleanup stale MongoDB lock: %v", err)
		}

		lockDoc := map[string]any{
			"_id":       mongoLockDocumentID,
			"lockedAt":  now,
			"lockedBy":  ownerID,
			"expiresAt": now.Add(defaultLockTTL),
		}

		_, err := mg.Mongo.InsertOne(ctx, mongoLockCollection, lockDoc)
		if err == nil {
			c.Debug("Mongo lock acquired successfully")

			go mg.startRefresh(ctx, cancel, c, ownerID)

			return mg.migrator.lock(ctx, cancel, c, ownerID)
		}

		if !isDuplicateKeyError(err) {
			c.Errorf("error while acquiring mongodb lock: %v", err)

			return errLockAcquisitionFailed
		}

		c.Debugf("MongoDB lock already held, retrying in %v... (attempt %d)", defaultRetry, i+1)

		select {
		case <-time.After(defaultRetry):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (mg mongoMigrator) unlock(c *container.Container, ownerID string) error {
	deleted, err := mg.Mongo.DeleteOne(context.Background(), mongoLockCollection, map[string]any{
		"_id":      mongoLockDocumentID,
		"lockedBy": ownerID,
	})
	if err != nil {
		c.Errorf("unable to release MongoDB lock: %v", err)
		return errLockReleaseFailed
	}

	if deleted == 0 {
		c.Errorf("failed to release MongoDB lock: lock already released or owned by another instance")
		return errLockReleaseFailed
	}

	c.Debug("Mongo lock released successfully")

	return mg.migrator.unlock(c, ownerID)
}

func (mongoMigrator) name() string {
	return "Mongo"
}
