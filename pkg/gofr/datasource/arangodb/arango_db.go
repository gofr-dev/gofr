package arangodb

import (
	"context"

	"github.com/arangodb/go-driver/v2/arangodb"
)

type DB struct {
	client *Client
}

// CreateDB creates a new database in ArangoDB.
// It first checks if the database already exists before attempting to create it.
// Returns ErrDatabaseExists if the database already exists.
func (d *DB) CreateDB(ctx context.Context, database string) error {
	ctx, done := d.client.instrumentOp(ctx, &QueryLog{Operation: "createDB", Database: database})
	defer done()

	// Check if the database already exists
	exists, err := d.client.client.DatabaseExists(ctx, database)
	if err != nil {
		return err
	}

	if exists {
		d.client.logger.Debugf("database %s already exists", database)
		return ErrDatabaseExists
	}

	_, err = d.client.client.CreateDatabase(ctx, database, nil)

	return err
}

// DropDB deletes a database from ArangoDB.
func (d *DB) DropDB(ctx context.Context, database string) error {
	ctx, done := d.client.instrumentOp(ctx, &QueryLog{Operation: "dropDB", Database: database})
	defer done()

	db, err := d.client.client.GetDatabase(ctx, database, &arangodb.GetDatabaseOptions{})
	if err != nil {
		return err
	}

	err = db.Remove(ctx)
	if err != nil {
		return err
	}

	return err
}

// CreateCollection creates a new collection in a database with specified type.
// It first checks if the collection already exists before attempting to create it.
// Returns ErrCollectionExists if the collection already exists.
func (d *DB) CreateCollection(ctx context.Context, database, collection string, isEdge bool) error {
	ctx, done := d.client.instrumentOp(ctx, &QueryLog{Operation: "createCollection", Database: database,
		Collection: collection, Filter: isEdge})
	defer done()

	db, err := d.client.client.GetDatabase(ctx, database, nil)
	if err != nil {
		return err
	}

	// Check if the collection already exists
	exists, err := db.CollectionExists(ctx, collection)
	if err != nil {
		return err
	}

	if exists {
		d.client.logger.Debugf("collection %s already exists in database %s", collection, database)
		return ErrCollectionExists
	}

	collectionType := arangodb.CollectionTypeDocument
	if isEdge {
		collectionType = arangodb.CollectionTypeEdge
	}

	options := arangodb.CreateCollectionPropertiesV2{Type: &collectionType}

	_, err = db.CreateCollectionV2(ctx, collection, &options)

	return err
}

// DropCollection deletes an existing collection from a database.
func (d *DB) DropCollection(ctx context.Context, database, collectionName string) error {
	ctx, done := d.client.instrumentOp(ctx, &QueryLog{Operation: "dropCollection", Database: database,
		Collection: collectionName})
	defer done()

	collection, err := d.getCollection(ctx, database, collectionName)
	if err != nil {
		return err
	}

	return collection.Remove(ctx)
}

func (d *DB) getCollection(ctx context.Context, dbName, collectionName string) (arangodb.Collection, error) {
	db, err := d.client.client.GetDatabase(ctx, dbName, nil)
	if err != nil {
		return nil, err
	}

	collection, err := db.GetCollection(ctx, collectionName, nil)
	if err != nil {
		return nil, err
	}

	return collection, nil
}
