package container

import (
	"bytes"
	"context"
	"database/sql"

	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
)

//go:generate go run go.uber.org/mock/mockgen -source=datasources.go -destination=mock_datasources.go -package=container
type DB interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Begin() (*gofrSQL.Tx, error)
	Select(ctx context.Context, data any, query string, args ...any)
	HealthCheck() *datasource.Health
	Dialect() string
	Close() error
}

type Redis interface {
	redis.Cmdable
	redis.HashCmdable
	HealthCheck() datasource.Health
	Close() error
}

// Cassandra is an interface representing a cassandra database
// Deprecated: Cassandra interface is deprecated and will be removed in future releases, users must use CassandraWithContext.
type Cassandra interface {
	// Deprecated: Query method is deprecated and will be removed in future releases, users must use QueryWithCtx.
	// Query executes the query and binds the result into dest parameter.
	// Returns error if any error occurs while binding the result.
	// Can be used to single as well as multiple rows.
	// Accepts pointer to struct or slice as dest parameter for single and multiple rows retrieval respectively.
	//
	// Example:
	//
	//	// Get multiple rows with only one column
	//	   ids := make([]int, 0)
	//	   err := c.Query(&ids, "SELECT id FROM users")
	//
	//	// Get a single object from database
	//	   type user struct {
	//	   	ID    int
	//	   	Name string
	//	   }
	//	   u := user{}
	//	   err := c.Query(&u, "SELECT * FROM users WHERE id=?", 1)
	//
	//	// Get array of objects from multiple rows
	//	   type user struct {
	//	   	ID    int
	//	   	Name string `db:"name"`
	//	   }
	//	   users := []user{}
	//	   err := c.Query(&users, "SELECT * FROM users")
	Query(dest any, stmt string, values ...any) error

	// Deprecated: Exec method is deprecated and will be removed in future releases, users must use ExecWithCtx.
	// Exec executes the query without returning any rows.
	// Return error if any error occurs while executing the query.
	// Can be used to execute UPDATE or INSERT.
	//
	// Example:
	//
	//	// Without values
	//	   err := c.Exec("INSERT INTO users VALUES(1, 'John Doe')")
	//
	//	// With Values
	//	   id := 1
	//	   name := "John Doe"
	//	   err := c.Exec("INSERT INTO users VALUES(?, ?)", id, name)
	Exec(stmt string, values ...any) error

	// Deprecated: ExecCAS method is deprecated and will be removed in future releases, users must use ExecCASWithCtx.
	// ExecCAS executes a lightweight transaction (i.e. an UPDATE or INSERT statement containing an IF clause).
	// If the transaction fails because the existing values did not match, the previous values will be stored in dest.
	// Returns true if the query is applied otherwise false.
	// Returns false and error if any error occur while executing the query.
	// Accepts only pointer to struct and built-in types as the dest parameter.
	//
	// Example:
	//
	//	type user struct {
	//		ID    int
	//		Name string
	//	}
	//	u := user{}
	//	applied, err := c.ExecCAS(&user, "INSERT INTO users VALUES(1, 'John Doe') IF NOT EXISTS")
	ExecCAS(dest any, stmt string, values ...any) (bool, error)

	// Deprecated: NewBatch method is deprecated and will be removed in future releases, users must use NewBatchWithCtx.
	// NewBatch creates a new Cassandra batch with the specified name and batch type.
	// This method initializes a new Cassandra batch operation. It sets up the batch
	// with the given name and type, allowing you to execute multiple queries in
	// a single batch operation. The `batchType` determines the type of batch operation
	// and can be one of `LoggedBatch`, `UnloggedBatch`, or `CounterBatch`.
	// These constants have been defined in gofr.dev/pkg/gofr/datasource/cassandra
	//
	// Example:
	//	err := client.NewBatch("myBatch", cassandra.LoggedBatch)
	NewBatch(name string, batchType int) error

	CassandraBatch

	HealthChecker
}

type CassandraBatch interface {
	// Deprecated: BatchQuery method is deprecated and will be removed in future releases, users must use BatchQueryWithCtx.
	// BatchQuery adds the query to the batch operation
	//
	// Example:
	//
	//	// Without values
	//	   c.BatchQuery("INSERT INTO users VALUES(1, 'John Doe')")
	//	   c.BatchQuery("INSERT INTO users VALUES(2, 'Jane Smith')")
	//
	//	// With Values
	//	   id1 := 1
	//	   name1 := "John Doe"
	//	   id2 := 2
	//	   name2 := "Jane Smith"
	//	   c.BatchQuery("INSERT INTO users VALUES(?, ?)", id1, name1)
	//	   c.BatchQuery("INSERT INTO users VALUES(?, ?)", id2, name2)
	BatchQuery(name, stmt string, values ...any) error

	// Deprecated: ExecuteBatch method is deprecated and will be removed in future releases, users must use ExecuteBatchWithCtx.
	// ExecuteBatch executes a batch operation and returns nil if successful otherwise an error is returned describing the failure.
	//
	// Example:
	//
	//	err := c.ExecuteBatch("myBatch")
	ExecuteBatch(name string) error

	// Deprecated: ExecuteBatchCAS method is deprecated and will be removed in future releases, users must use ExecuteBatchCASWithCtx.
	// ExecuteBatchCAS executes a batch operation and returns true if successful.
	// Returns true if the query is applied otherwise false.
	// Returns false and error if any error occur while executing the query.
	// Accepts only pointer to struct and built-in types as the dest parameter.
	//
	// Example:
	//
	//  applied, err := c.ExecuteBatchCAS("myBatch");
	ExecuteBatchCAS(name string, dest ...any) (bool, error)
}

type CassandraWithContext interface {
	// QueryWithCtx executes the query with a context and binds the result into dest parameter.
	// Accepts pointer to struct or slice as dest parameter for single and multiple rows retrieval respectively.
	QueryWithCtx(ctx context.Context, dest any, stmt string, values ...any) error

	// ExecWithCtx executes the query with a context, without returning any rows.
	ExecWithCtx(ctx context.Context, stmt string, values ...any) error

	// ExecCASWithCtx executes a lightweight transaction with a context.
	ExecCASWithCtx(ctx context.Context, dest any, stmt string, values ...any) (bool, error)

	// NewBatchWithCtx creates a new Cassandra batch with context.
	NewBatchWithCtx(ctx context.Context, name string, batchType int) error

	Cassandra
	CassandraBatchWithContext
}

type CassandraBatchWithContext interface {
	// BatchQueryWithCtx adds the query to the batch operation with a context.
	BatchQueryWithCtx(ctx context.Context, name, stmt string, values ...any) error

	// ExecuteBatchWithCtx executes a batch operation with a context.
	ExecuteBatchWithCtx(ctx context.Context, name string) error

	// ExecuteBatchCASWithCtx executes a batch operation with context and returns the result.
	ExecuteBatchCASWithCtx(ctx context.Context, name string, dest ...any) (bool, error)
}

type CassandraProvider interface {
	CassandraWithContext

	provider
}

type Clickhouse interface {
	Exec(ctx context.Context, query string, args ...any) error
	Select(ctx context.Context, dest any, query string, args ...any) error
	AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error

	HealthChecker
}

type ClickhouseProvider interface {
	Clickhouse

	provider
}

// Mongo is an interface representing a MongoDB database client with common CRUD operations.
type Mongo interface {
	// Find executes a query to find documents in a collection based on a filter and stores the results
	// into the provided results interface.
	Find(ctx context.Context, collection string, filter any, results any) error

	// FindOne executes a query to find a single document in a collection based on a filter and stores the result
	// into the provided result interface.
	FindOne(ctx context.Context, collection string, filter any, result any) error

	// InsertOne inserts a single document into a collection.
	// It returns the identifier of the inserted document and an error, if any.
	InsertOne(ctx context.Context, collection string, document any) (any, error)

	// InsertMany inserts multiple documents into a collection.
	// It returns the identifiers of the inserted documents and an error, if any.
	InsertMany(ctx context.Context, collection string, documents []any) ([]any, error)

	// DeleteOne deletes a single document from a collection based on a filter.
	// It returns the number of documents deleted and an error, if any.
	DeleteOne(ctx context.Context, collection string, filter any) (int64, error)

	// DeleteMany deletes multiple documents from a collection based on a filter.
	// It returns the number of documents deleted and an error, if any.
	DeleteMany(ctx context.Context, collection string, filter any) (int64, error)

	// UpdateByID updates a document in a collection by its ID.
	// It returns the number of documents updated and an error if any.
	UpdateByID(ctx context.Context, collection string, id any, update any) (int64, error)

	// UpdateOne updates a single document in a collection based on a filter.
	// It returns an error if any.
	UpdateOne(ctx context.Context, collection string, filter any, update any) error

	// UpdateMany updates multiple documents in a collection based on a filter.
	// It returns the number of documents updated and an error if any.
	UpdateMany(ctx context.Context, collection string, filter any, update any) (int64, error)

	// CountDocuments counts the number of documents in a collection based on a filter.
	// It returns the count and an error if any.
	CountDocuments(ctx context.Context, collection string, filter any) (int64, error)

	// Drop an entire collection from the database.
	// It returns an error if any.
	Drop(ctx context.Context, collection string) error

	// CreateCollection creates a new collection with specified name and default options.
	CreateCollection(ctx context.Context, name string) error

	// StartSession starts a session and provide methods to run commands in a transaction.
	StartSession() (any, error)

	HealthChecker
}

type Transaction interface {
	StartTransaction() error
	AbortTransaction(context.Context) error
	CommitTransaction(context.Context) error
	EndSession(context.Context)
}

// MongoProvider is an interface that extends Mongo with additional methods for logging, metrics, and connection management.
// Which is used for initializing datasource.
type MongoProvider interface {
	Mongo

	provider
}

type provider interface {
	// UseLogger sets the logger for the Cassandra client.
	UseLogger(logger any)

	// UseMetrics sets the metrics for the Cassandra client.
	UseMetrics(metrics any)

	// UseTracer sets the tracer for the Cassandra client.
	UseTracer(tracer any)

	// Connect establishes a connection to a DB and registers metrics using the provided configuration when the client was Created.
	Connect(ctx context.Context) error
}

type HealthChecker interface {
	// HealthCheck returns an interface rather than a struct as externalDB's are part of different module.
	// It is done to avoid adding packages which are not being used.
	HealthCheck(context.Context) (any, error)
}

type KVStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error

	HealthChecker
}

type KVStoreProvider interface {
	KVStore

	provider
}

type PubSubProvider interface {
	pubsub.Client

	provider
}

type Solr interface {
	Search(ctx context.Context, collection string, params map[string]any) (any, error)
	Create(ctx context.Context, collection string, document *bytes.Buffer, params map[string]any) (any, error)
	Update(ctx context.Context, collection string, document *bytes.Buffer, params map[string]any) (any, error)
	Delete(ctx context.Context, collection string, document *bytes.Buffer, params map[string]any) (any, error)

	Retrieve(ctx context.Context, collection string, params map[string]any) (any, error)
	ListFields(ctx context.Context, collection string, params map[string]any) (any, error)
	AddField(ctx context.Context, collection string, document *bytes.Buffer) (any, error)
	UpdateField(ctx context.Context, collection string, document *bytes.Buffer) (any, error)
	DeleteField(ctx context.Context, collection string, document *bytes.Buffer) (any, error)

	HealthChecker
}

type SolrProvider interface {
	Solr

	provider
}

// Dgraph defines the methods for interacting with a Dgraph database.
type Dgraph interface {
	// Query executes a read-only query in the Dgraph database and returns the result.
	// Parameters:
	// - ctx: The context for the query, used for controlling timeouts, cancellation, etc.
	// - query: The Dgraph query string in GraphQL+- format.
	// Returns:
	// - any: The result of the query, usually of type *api.Response.
	// - error: An error if the query execution fails.
	Query(ctx context.Context, query string) (any, error)

	// QueryWithVars executes a read-only query with variables in the Dgraph database.
	// Parameters:
	// - ctx: The context for the query.
	// - query: The Dgraph query string in GraphQL+- format.
	// - vars: A map of variables to be used within the query.
	// Returns:
	// - any: The result of the query with variables, usually of type *api.Response.
	// - error: An error if the query execution fails.
	QueryWithVars(ctx context.Context, query string, vars map[string]string) (any, error)

	// Mutate executes a write operation (mutation) in the Dgraph database and returns the result.
	// Parameters:
	// - ctx: The context for the mutation.
	// - mu: The mutation operation, usually of type *api.Mutation.
	// Returns:
	// - any: The result of the mutation, usually of type *api.Assigned.
	// - error: An error if the mutation execution fails.
	Mutate(ctx context.Context, mu any) (any, error)

	// Alter applies schema or other changes to the Dgraph database.
	// Parameters:
	// - ctx: The context for the alter operation.
	// - op: The alter operation, usually of type *api.Operation.
	// Returns:
	// - error: An error if the operation fails.
	Alter(ctx context.Context, op any) error

	// NewTxn creates a new transaction (read-write) for interacting with the Dgraph database.
	// Returns:
	// - any: A new transaction, usually of type *api.Txn.
	NewTxn() any

	// NewReadOnlyTxn creates a new read-only transaction for querying the Dgraph database.
	// Returns:
	// - any: A new read-only transaction, usually of type *api.Txn.
	NewReadOnlyTxn() any

	// HealthChecker checks the health of the Dgraph instance, ensuring it is up and running.
	// Returns:
	// - error: An error if the health check fails.
	HealthChecker
}

// DgraphProvider extends Dgraph with connection management capabilities.
type DgraphProvider interface {
	Dgraph
	provider
}
