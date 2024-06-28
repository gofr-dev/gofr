package migration

import (
	"context"
	"database/sql"
	"time"

	goRedis "github.com/redis/go-redis/v9"
	"gofr.dev/pkg/gofr/container"
)

type Redis interface {
	Get(ctx context.Context, key string) *goRedis.StringCmd
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *goRedis.StatusCmd
	Del(ctx context.Context, keys ...string) *goRedis.IntCmd
	Rename(ctx context.Context, key, newKey string) *goRedis.StatusCmd
}

type SQL interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

type PubSub interface {
	CreateTopic(context context.Context, name string) error
	DeleteTopic(context context.Context, name string) error
}

type Clickhouse interface {
	Exec(ctx context.Context, query string, args ...any) error
	Select(ctx context.Context, dest any, query string, args ...any) error
	AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error
}

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

// keeping the migrator interface unexported as, right now it is not being implemented directly, by the externalDB drivers.
// keeping the implementations for externalDB at one place such that if any change in migration logic, we would change directly here.
type migrator interface {
	checkAndCreateMigrationTable(c *container.Container) error
	getLastMigration(c *container.Container) int64

	beginTransaction(c *container.Container) transactionData

	commitMigration(c *container.Container, data transactionData) error
	rollback(c *container.Container, data transactionData)
}
