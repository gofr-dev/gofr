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
	Query(ctx context.Context, query string, args ...any) ([]byte, error)
	CreateTopic(context context.Context, name string) error
	DeleteTopic(context context.Context, name string) error
}

type Clickhouse interface {
	Exec(ctx context.Context, query string, args ...any) error
	Select(ctx context.Context, dest any, query string, args ...any) error
	AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error

	HealthCheck(ctx context.Context) (any, error)
}

type Oracle interface {
	Select(ctx context.Context, dest any, query string, args ...any) error
	Exec(ctx context.Context, query string, args ...any) error
	Begin() (container.OracleTx, error)
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

type SurrealDB interface {
	// Query executes a Surreal query with the provided variables and returns the query results as a slice of interfaces{}.
	// It returns an error if the query execution fails.
	Query(ctx context.Context, query string, vars map[string]any) ([]any, error)

	// CreateNamespace creates a new namespace in the SurrealDB instance.
	CreateNamespace(ctx context.Context, namespace string) error

	// CreateDatabase creates a new database in the SurrealDB instance.
	CreateDatabase(ctx context.Context, database string) error

	// DropNamespace deletes a namespace from the SurrealDB instance.
	DropNamespace(ctx context.Context, namespace string) error

	// DropDatabase deletes a database from the SurrealDB instance.
	DropDatabase(ctx context.Context, database string) error
}

type DGraph interface {
	// ApplySchema applies or updates the complete database schema.
	// Parameters:
	// - ctx: Context for request cancellation and timeouts
	// - schema: Schema definition in Dgraph Schema Definition Language (SDL) format
	// Returns:
	// - error: An error if the schema application fails
	ApplySchema(ctx context.Context, schema string) error

	// AddOrUpdateField atomically creates or updates a single field definition.
	// Parameters:
	// - ctx: Context for request cancellation and timeouts
	// - fieldName: Name of the field/predicate to create or update
	// - fieldType: Dgraph data type (e.g., string, int, datetime)
	// - directives: Space-separated Dgraph directives (e.g., "@index(hash) @upsert")
	// Returns:
	// - error: An error if the field operation fails
	AddOrUpdateField(ctx context.Context, fieldName, fieldType, directives string) error

	// DropField permanently removes a field/predicate and all its associated data.
	// Parameters:
	// - ctx: Context for request cancellation and timeouts
	// - fieldName: Name of the field/predicate to remove
	// Returns:
	// - error: An error if the field removal fails
	DropField(ctx context.Context, fieldName string) error
}

type ScyllaDB interface {
	Query(dest any, stmt string, values ...any) error
	QueryWithCtx(ctx context.Context, dest any, stmt string, values ...any) error

	Exec(stmt string, values ...any) error
	ExecWithCtx(ctx context.Context, stmt string, values ...any) error

	ExecCAS(dest any, stmt string, values ...any) (bool, error)

	NewBatch(name string, batchType int) error
	NewBatchWithCtx(ctx context.Context, name string, batchType int) error

	BatchQuery(name, stmt string, values ...any) error
	BatchQueryWithCtx(ctx context.Context, name, stmt string, values ...any) error

	ExecuteBatchWithCtx(ctx context.Context, name string) error
}

// Elasticsearch is an interface representing an Elasticsearch client for migration operations.
// It includes only the essential methods needed for schema changes and migrations.
type Elasticsearch interface {
	// CreateIndex creates a new index with optional mapping/settings.
	CreateIndex(ctx context.Context, index string, settings map[string]any) error

	// DeleteIndex deletes an existing index.
	DeleteIndex(ctx context.Context, index string) error

	// IndexDocument indexes (creates or replaces) a single document.
	// Useful for seeding data or adding configuration documents during migrations.
	IndexDocument(ctx context.Context, index, id string, document any) error

	// DeleteDocument removes a document by ID.
	// Useful for removing specific documents during migrations.
	DeleteDocument(ctx context.Context, index, id string) error

	// Bulk executes multiple indexing/updating/deleting operations in one request.
	// Each entry in `operations` should be a JSON‑serializable object
	// following the Elasticsearch bulk API format.
	// Useful for bulk operations during migrations.
	Bulk(ctx context.Context, operations []map[string]any) (map[string]any, error)

	Search(ctx context.Context, indices []string, query map[string]any) (map[string]any, error)

	GetDocument(ctx context.Context, index, id string) (map[string]any, error)

	UpdateDocument(ctx context.Context, index, id string, update map[string]any) error

	HealthCheck(ctx context.Context) (any, error)
}

// keeping the migrator interface unexported as, right now it is not being implemented directly, by the externalDB drivers.
// keeping the implementations for externalDB at one place such that if any change in migration logic, we would change directly here.
type migrator interface {
	checkAndCreateMigrationTable(c *container.Container) error
	getLastMigration(c *container.Container) int64

	beginTransaction(c *container.Container) transactionData

	commitMigration(c *container.Container, data transactionData) error
	rollback(c *container.Container, data transactionData)

	Locker
}

type OpenTSDB interface {
	// PutDataPoints can be used for seeding initial metrics during migration
	PutDataPoints(ctx context.Context, data any, queryParam string, res any) error
	// PostAnnotation creates or updates an annotation in OpenTSDB using the 'POST /api/annotation' endpoint.
	PostAnnotation(ctx context.Context, annotation any, res any) error
	// PutAnnotation creates or replaces an annotation in OpenTSDB using the 'PUT /api/annotation' endpoint.
	PutAnnotation(ctx context.Context, annotation any, res any) error
	// DeleteAnnotation removes an annotation from OpenTSDB using the 'DELETE /api/annotation' endpoint.
	DeleteAnnotation(ctx context.Context, annotation any, res any) error
}

type Locker interface {
	AcquireLock(c *container.Container) error
	ReleaseLock(c *container.Container) error
	Name() string
}
