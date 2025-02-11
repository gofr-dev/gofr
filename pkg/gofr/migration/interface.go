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
	Set(ctx context.Context, key string, value any, expiration time.Duration) *goRedis.StatusCmd
	Del(ctx context.Context, keys ...string) *goRedis.IntCmd
	Rename(ctx context.Context, key, newKey string) *goRedis.StatusCmd
}

type SQL interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type PubSub interface {
	CreateTopic(context context.Context, name string) error
	DeleteTopic(context context.Context, name string) error
}

type Clickhouse interface {
	Exec(ctx context.Context, query string, args ...any) error
	Select(ctx context.Context, dest any, query string, args ...any) error
	AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error

	HealthCheck(ctx context.Context) (any, error)
}

type Cassandra interface {
	Exec(query string, args ...any) error
	NewBatch(name string, batchType int) error
	BatchQuery(name, stmt string, values ...any) error
	ExecuteBatch(name string) error

	HealthCheck(ctx context.Context) (any, error)
}

// Mongo is an interface representing a MongoDB database client with common CRUD operations.
type Mongo interface {
	Find(ctx context.Context, collection string, filter any, results any) error
	FindOne(ctx context.Context, collection string, filter any, result any) error
	InsertOne(ctx context.Context, collection string, document any) (any, error)
	InsertMany(ctx context.Context, collection string, documents []any) ([]any, error)
	DeleteOne(ctx context.Context, collection string, filter any) (int64, error)
	DeleteMany(ctx context.Context, collection string, filter any) (int64, error)
	UpdateByID(ctx context.Context, collection string, id any, update any) (int64, error)
	UpdateOne(ctx context.Context, collection string, filter any, update any) error
	UpdateMany(ctx context.Context, collection string, filter any, update any) (int64, error)
	Drop(ctx context.Context, collection string) error
	CreateCollection(ctx context.Context, name string) error
	StartSession() (any, error)
}

// ArangoDB is an interface representing an ArangoDB database client with common CRUD operations.
type ArangoDB interface {
	// CreateDB creates a new database in ArangoDB.
	CreateDB(ctx context.Context, database string) error
	// DropDB deletes an existing database in ArangoDB.
	DropDB(ctx context.Context, database string) error

	// CreateCollection creates a new collection in a database with specified type.
	CreateCollection(ctx context.Context, database, collection string, isEdge bool) error
	// DropCollection deletes an existing collection from a database.
	DropCollection(ctx context.Context, database, collection string) error

	// CreateGraph creates a new graph in a database.
	CreateGraph(ctx context.Context, database, graph string, edgeDefinitions any) error
	// DropGraph deletes an existing graph from a database.
	DropGraph(ctx context.Context, database, graph string) error
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
