package migration

import (
	"context"

	"gofr.dev/pkg/gofr/container"
)

type Datasource struct {
	// TODO Logger should not be embedded rather it should be a field.
	// Need to think it through as it will bring breaking changes.
	Logger

	SQL        SQL
	Redis      Redis
	PubSub     PubSub
	Clickhouse Clickhouse
	Cassandra  Cassandra
	Mongo      Mongo
}

// It is a base implementation for migration manager, on this other database drivers have been wrapped.

func (*Datasource) checkAndCreateMigrationTable(context.Context, *container.Container) error {
	return nil
}

func (*Datasource) getLastMigration(context.Context, *container.Container) int64 {
	return 0
}

func (*Datasource) beginTransaction(context.Context, *container.Container) transactionData {
	return transactionData{}
}

func (*Datasource) commitMigration(ctx context.Context, c *container.Container, data transactionData) error {
	c.Infof("Migration %v ran successfully", data.MigrationNumber)

	return nil
}

func (*Datasource) rollback(context.Context, *container.Container, transactionData) {}
