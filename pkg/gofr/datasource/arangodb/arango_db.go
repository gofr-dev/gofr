package arangodb

import (
	"context"
	"time"

	"github.com/arangodb/go-driver/v2/arangodb"
)

type DB struct {
	client *Client
}

// ListDBs returns a list of all databases in ArangoDB.
func (d *DB) ListDBs(ctx context.Context) ([]string, error) {
	tracerCtx, span := d.client.addTrace(ctx, "listDBs", nil)
	startTime := time.Now()

	defer d.client.sendOperationStats(&QueryLog{Operation: "listDBs"}, startTime, "listDBs", span)

	dbs, err := d.client.client.Databases(tracerCtx)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(dbs))

	for _, db := range dbs {
		if db.Name() != "" {
			names = append(names, db.Name())
		}
	}

	return names, nil
}

// CreateDB creates a new database in ArangoDB.
func (d *DB) CreateDB(ctx context.Context, database string) error {
	tracerCtx, span := d.client.addTrace(ctx, "createDB", map[string]string{"DB": database})
	startTime := time.Now()

	defer d.client.sendOperationStats(&QueryLog{Operation: "createDB", Database: database}, startTime, "createDB", span)

	_, err := d.client.client.CreateDatabase(tracerCtx, database, nil)

	return err
}

// DropDB deletes a database from ArangoDB.
func (d *DB) DropDB(ctx context.Context, database string) error {
	tracerCtx, span := d.client.addTrace(ctx, "dropDB", map[string]string{"DB": database})
	startTime := time.Now()

	defer d.client.sendOperationStats(&QueryLog{Operation: "dropDB", Database: database}, startTime, "dropDB", span)

	db, err := d.client.client.Database(tracerCtx, database)
	if err != nil {
		return err
	}

	err = db.Remove(tracerCtx)
	if err != nil {
		return err
	}

	return err
}

// CreateCollection creates a new collection in a database with specified type.
func (d *DB) CreateCollection(ctx context.Context, database, collection string, isEdge bool) error {
	tracerCtx, span := d.client.addTrace(ctx, "createCollection", map[string]string{"collection": collection})
	startTime := time.Now()

	defer d.client.sendOperationStats(&QueryLog{Operation: "createCollection", Database: database,
		Collection: collection, Filter: isEdge}, startTime, "createCollection", span)

	db, err := d.client.client.Database(tracerCtx, database)
	if err != nil {
		return err
	}

	options := arangodb.CreateCollectionProperties{Type: arangodb.CollectionTypeDocument}
	if isEdge {
		options.Type = arangodb.CollectionTypeEdge
	}

	_, err = db.CreateCollection(tracerCtx, collection, &options)

	return err
}

// DropCollection deletes an existing collection from a database.
func (d *DB) DropCollection(ctx context.Context, database, collectionName string) error {
	return d.handleCollectionOperation(ctx, "dropCollection", database, collectionName, func(collection arangodb.Collection) error {
		return collection.Remove(ctx)
	})
}

// TruncateCollection truncates a collection in a database.
func (d *DB) TruncateCollection(ctx context.Context, database, collectionName string) error {
	return d.handleCollectionOperation(ctx, "truncateCollection", database, collectionName, func(collection arangodb.Collection) error {
		return collection.Truncate(ctx)
	})
}

// ListCollections lists all collections in a database.
func (d *DB) ListCollections(ctx context.Context, database string) ([]string, error) {
	tracerCtx, span := d.client.addTrace(ctx, "listCollections", map[string]string{"DB": database})
	startTime := time.Now()

	defer d.client.sendOperationStats(&QueryLog{Operation: "listCollections",
		Database: database}, startTime, "listCollections", span)

	db, err := d.client.client.Database(tracerCtx, database)
	if err != nil {
		return nil, err
	}

	collections, err := db.Collections(tracerCtx)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(collections))
	for _, coll := range collections {
		names = append(names, coll.Name())
	}

	return names, nil
}

func (d *DB) getCollection(ctx context.Context, dbName, collectionName string) (arangodb.Collection, error) {
	db, err := d.client.client.Database(ctx, dbName)
	if err != nil {
		return nil, err
	}

	collection, err := db.Collection(ctx, collectionName)
	if err != nil {
		return nil, err
	}

	return collection, nil
}

// handleCollectionOperation handles common logic for collection operations.
func (d *DB) handleCollectionOperation(ctx context.Context, operation, database, collectionName string,
	action func(arangodb.Collection) error) error {
	tracerCtx, span := d.client.addTrace(ctx, operation, map[string]string{"collection": collectionName})
	startTime := time.Now()

	defer d.client.sendOperationStats(&QueryLog{Operation: operation, Database: database,
		Collection: collectionName}, startTime, operation, span)

	collection, err := d.getCollection(tracerCtx, database, collectionName)
	if err != nil {
		return err
	}

	return action(collection)
}
