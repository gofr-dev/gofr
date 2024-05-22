package datasource

import (
	"context"
)

// Mongo is an interface representing a MongoDB database client with common CRUD operations.
type Mongo interface {
	// Find executes a query to find documents in a collection based on a filter and stores the results
	// into the provided results interface.
	Find(ctx context.Context, collection string, filter interface{}, results interface{}) error

	// FindOne executes a query to find a single document in a collection based on a filter and stores the result
	// into the provided result interface.
	FindOne(ctx context.Context, collection string, filter interface{}, result interface{}) error

	// InsertOne inserts a single document into a collection.
	// It returns the identifier of the inserted document and an error, if any.
	InsertOne(ctx context.Context, collection string, document interface{}) (interface{}, error)

	// InsertMany inserts multiple documents into a collection.
	// It returns the identifiers of the inserted documents and an error, if any.
	InsertMany(ctx context.Context, collection string, documents []interface{}) ([]interface{}, error)

	// DeleteOne deletes a single document from a collection based on a filter.
	// It returns the number of documents deleted and an error, if any.
	DeleteOne(ctx context.Context, collection string, filter interface{}) (int64, error)

	// DeleteMany deletes multiple documents from a collection based on a filter.
	// It returns the number of documents deleted and an error, if any.
	DeleteMany(ctx context.Context, collection string, filter interface{}) (int64, error)

	// UpdateByID updates a document in a collection by its ID.
	// It returns the number of documents updated and an error if any.
	UpdateByID(ctx context.Context, collection string, id interface{}, update interface{}) (int64, error)

	// UpdateOne updates a single document in a collection based on a filter.
	// It returns an error if any.
	UpdateOne(ctx context.Context, collection string, filter interface{}, update interface{}) error

	// UpdateMany updates multiple documents in a collection based on a filter.
	// It returns the number of documents updated and an error if any.
	UpdateMany(ctx context.Context, collection string, filter interface{}, update interface{}) (int64, error)

	// CountDocuments counts the number of documents in a collection based on a filter.
	// It returns the count and an error if any.
	CountDocuments(ctx context.Context, collection string, filter interface{}) (int64, error)

	// Drop an entire collection from the database.
	// It returns an error if any.
	Drop(ctx context.Context, collection string) error
}

// MongoProvider is an interface that extends Mongo with additional methods for logging, metrics, and connection management.
// Which is used for initializing datasource.
type MongoProvider interface {
	Mongo

	// UseLogger sets the logger for the MongoDB client.
	UseLogger(logger interface{})

	// UseMetrics sets the metrics for the MongoDB client.
	UseMetrics(metrics interface{})

	// Connect establishes a connection to MongoDB and registers metrics using the provided configuration when the client was Created.
	Connect()
}
