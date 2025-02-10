package migration

import (
	"context"
	"fmt"
	"time"

	"gofr.dev/pkg/gofr/container"
)

// arangoDS is our adapter struct that will implement both interfaces.
type arangoDS struct {
	client container.ArangoDB
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

func (ds arangoDS) CreateDocument(ctx context.Context, dbName, collectionName string, document any) (string, error) {
	return ds.client.CreateDocument(ctx, dbName, collectionName, document)
}

func (ds arangoDS) GetDocument(ctx context.Context, dbName, collectionName, documentID string, result any) error {
	return ds.client.GetDocument(ctx, dbName, collectionName, documentID, result)
}

func (ds arangoDS) UpdateDocument(ctx context.Context, dbName, collectionName, documentID string, document any) error {
	return ds.client.UpdateDocument(ctx, dbName, collectionName, documentID, document)
}

func (ds arangoDS) DeleteDocument(ctx context.Context, dbName, collectionName, documentID string) error {
	return ds.client.DeleteDocument(ctx, dbName, collectionName, documentID)
}

func (ds arangoDS) GetEdges(ctx context.Context, dbName, graphName, edgeCollection, vertexID string, resp any) error {
	return ds.client.GetEdges(ctx, dbName, graphName, edgeCollection, vertexID, resp)
}

func (ds arangoDS) Query(ctx context.Context, dbName, query string, bindVars map[string]any, result any) error {
	return ds.client.Query(ctx, dbName, query, bindVars, result)
}

func (ds arangoDS) CreateDB(ctx context.Context, database string) error {
	query := fmt.Sprintf("CREATE DATABASE %s", database)
	return ds.client.Query(ctx, "_system", query, nil, nil)
}

func (ds arangoDS) DropDB(ctx context.Context, database string) error {
	query := fmt.Sprintf("DROP DATABASE %s", database)
	return ds.client.Query(ctx, "_system", query, nil, nil)
}

func (ds arangoDS) CreateCollection(ctx context.Context, database, collection string, isEdge bool) error {
	var collType string
	if isEdge {
		collType = "edge"
	} else {
		collType = "document"
	}

	query := fmt.Sprintf(`
		CREATE COLLECTION %s TYPE %s
	`, collection, collType)

	return ds.client.Query(ctx, database, query, nil, nil)
}

func (ds arangoDS) DropCollection(ctx context.Context, database, collection string) error {
	query := fmt.Sprintf("DROP COLLECTION %s", collection)
	return ds.client.Query(ctx, database, query, nil, nil)
}

func (ds arangoDS) CreateGraph(ctx context.Context, database, graph string, edgeDefinitions any) error {
	query := fmt.Sprintf(`
		CREATE GRAPH %s WITH edgeDefinitions: @edgeDefs
	`, graph)

	bindVars := map[string]any{
		"edgeDefs": edgeDefinitions,
	}

	return ds.client.Query(ctx, database, query, bindVars, nil)
}

func (ds arangoDS) DropGraph(ctx context.Context, database, graph string) error {
	query := fmt.Sprintf("DROP GRAPH %s", graph)
	return ds.client.Query(ctx, database, query, nil, nil)
}

func (ds arangoDS) HealthCheck(ctx context.Context) (any, error) {
	return ds.client.HealthCheck(ctx)
}

func (ds arangoDS) apply(m migrator) migrator {
	return arangoMigrator{
		ArangoDB: ds,
		migrator: m,
	}
}

func (am arangoMigrator) checkAndCreateMigrationTable(c *container.Container) error {
	// Create migration collection in _system database
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

	err := c.ArangoDB.Query(context.Background(), arangoMigrationDB, insertArangoMigrationRecord, bindVars, nil)
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
