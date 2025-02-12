package migration

import (
	"context"
	"time"

	"gofr.dev/pkg/gofr/container"
)

// arangoDS is our adapter struct that will implement both interfaces.
type arangoDS struct {
	client ArangoDB
}

// arangoMigrator struct remains the same but uses our adapter.
type arangoMigrator struct {
	ArangoDB
	migrator
}

const (
	arangoMigrationDB         = "_system"
	arangoMigrationCollection = "gofr_migrations"

	getLastArangoMigration = `
  FOR doc IN gofr_migrations
    SORT doc.version DESC
    LIMIT 1
    RETURN doc.version
`
	insertArangoMigrationRecord = `
  INSERT {
    version: @version,
    method: @method,
    start_time: @start_time,
    duration: @duration
  } INTO gofr_migrations
`
)

func (ds arangoDS) CreateDB(ctx context.Context, database string) error {
	return ds.client.CreateDB(ctx, database)
}

func (ds arangoDS) DropDB(ctx context.Context, database string) error {
	return ds.client.DropDB(ctx, database)
}

func (ds arangoDS) CreateCollection(ctx context.Context, database, collection string, isEdge bool) error {
	return ds.client.CreateCollection(ctx, database, collection, isEdge)
}

func (ds arangoDS) DropCollection(ctx context.Context, database, collection string) error {
	return ds.client.DropCollection(ctx, database, collection)
}

func (ds arangoDS) CreateGraph(ctx context.Context, database, graph string, edgeDefinitions any) error {
	return ds.client.CreateGraph(ctx, database, graph, edgeDefinitions)
}

func (ds arangoDS) DropGraph(ctx context.Context, database, graph string) error {
	return ds.client.DropGraph(ctx, database, graph)
}

func (ds arangoDS) apply(m migrator) migrator {
	return arangoMigrator{
		ArangoDB: ds,
		migrator: m,
	}
}

func (am arangoMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	err := am.CreateCollection(context.Background(), arangoMigrationDB, arangoMigrationCollection, false)
	if err != nil {
		c.Debug("Migration collection might already exist:", err)
	}

	return am.migrator.checkAndCreateMigrationTable(c)
}

func (am arangoMigrator) getLastMigration(c *container.Container) int64 {
	var lastMigrations []int64

	err := c.ArangoDB.Query(context.Background(), arangoMigrationDB, getLastArangoMigration, nil, &lastMigrations)
	if err != nil || len(lastMigrations) == 0 {
		return 0
	}

	c.Debugf("ArangoDB last migration fetched value is: %v", lastMigrations[0])

	lm2 := am.migrator.getLastMigration(c)
	if lm2 > lastMigrations[0] {
		return lm2
	}

	return lastMigrations[0]
}

func (am arangoMigrator) beginTransaction(c *container.Container) transactionData {
	data := am.migrator.beginTransaction(c)

	c.Debug("ArangoDB migrator begin successfully")

	return data
}

func (am arangoMigrator) commitMigration(c *container.Container, data transactionData) error {
	bindVars := map[string]any{
		"version":    data.MigrationNumber,
		"method":     "UP",
		"start_time": data.StartTime,
		"duration":   time.Since(data.StartTime).Milliseconds(),
	}

	var result []map[string]any

	err := c.ArangoDB.Query(context.Background(), arangoMigrationDB, insertArangoMigrationRecord, bindVars, &result)
	if err != nil {
		return err
	}

	c.Debugf("Inserted record for migration %v in ArangoDB gofr_migrations collection", data.MigrationNumber)

	return am.migrator.commitMigration(c, data)
}

func (am arangoMigrator) rollback(c *container.Container, data transactionData) {
	am.migrator.rollback(c, data)

	c.Fatalf("Migration %v failed and rolled back", data.MigrationNumber)
}
