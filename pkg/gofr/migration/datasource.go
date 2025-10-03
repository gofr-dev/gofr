package migration

import "gofr.dev/pkg/gofr/container"

type Datasource struct {
	// TODO Logger should not be embedded rather it should be a field.
	// Need to think it through as it will bring breaking changes.
	Logger

	SQL           SQL
	Redis         Redis
	PubSub        PubSub
	Clickhouse    Clickhouse
	Oracle        Oracle
	Cassandra     Cassandra
	Mongo         Mongo
	ArangoDB      ArangoDB
	SurrealDB     SurrealDB
	DGraph        DGraph
	ScyllaDB      ScyllaDB
	Elasticsearch Elasticsearch
	OpenTSDB      OpenTSDB
}

// It is a base implementation for migration manager, on this other database drivers have been wrapped.

func (*Datasource) checkAndCreateMigrationTable(*container.Container) error {
	return nil
}

func (*Datasource) getLastMigration(*container.Container) int64 {
	return 0
}

func (*Datasource) beginTransaction(*container.Container) transactionData {
	return transactionData{}
}

func (*Datasource) commitMigration(c *container.Container, data transactionData) error {
	c.Infof("Migration %v ran successfully", data.MigrationNumber)

	return nil
}

func (*Datasource) rollback(*container.Container, transactionData) {}
