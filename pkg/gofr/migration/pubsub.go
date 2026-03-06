package migration

import (
	"context"

	"gofr.dev/pkg/gofr/container"
)

type pubsubDS struct {
	client PubSub
}

// pubsubMigrator wraps the next migrator in the chain.
// It is kept for structural consistency but no longer manages its own migration state
// to prevent "ghost data" conflicts on persistent PubSub backends.
type pubsubMigrator struct {
	PubSub
	migrator migrator
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
	return pm.migrator.checkAndCreateMigrationTable(c)
}

func (pm pubsubMigrator) getLastMigration(c *container.Container) (int64, error) {
	// PubSub no longer participates in version tracking.
	// We delegate directly to the next migrator in the chain (SQL, Redis, etc.).
	return pm.migrator.getLastMigration(c)
}

func (pm pubsubMigrator) beginTransaction(c *container.Container) transactionData {
	return pm.migrator.beginTransaction(c)
}

func (pm pubsubMigrator) commitMigration(c *container.Container, data transactionData) error {
	// No migration entry is added to PubSub anymore.
	// We only commit the migration in the primary data source.
	return pm.migrator.commitMigration(c, data)
}

func (pm pubsubMigrator) rollback(c *container.Container, data transactionData) {
	pm.migrator.rollback(c, data)
}

func (pm pubsubMigrator) lock(ctx context.Context, cancel context.CancelFunc, c *container.Container, ownerID string) error {
	return pm.migrator.lock(ctx, cancel, c, ownerID)
}

func (pm pubsubMigrator) unlock(c *container.Container, ownerID string) error {
	return pm.migrator.unlock(c, ownerID)
}

func (pubsubMigrator) name() string {
	return "PubSub"
}
