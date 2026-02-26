package container

import (
	"bytes"
	"context"
	"database/sql"
	"time"

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

type OracleDB interface {
	Exec(ctx context.Context, query string, args ...any) error
	Select(ctx context.Context, dest any, query string, args ...any) error
	Begin() (OracleTx, error)

	HealthChecker
}

type OracleTx interface {
	ExecContext(ctx context.Context, query string, args ...any) error
	SelectContext(ctx context.Context, dest any, query string, args ...any) error
	Commit() error
	Rollback() error
}

type OracleProvider interface {
	OracleDB

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

// SurrealDB defines an interface representing a SurrealDB client with common database operations.
type SurrealDB interface {
	// CreateNamespace creates a new namespace in the SurrealDB instance.
	CreateNamespace(ctx context.Context, namespace string) error

	// CreateDatabase creates a new database in the SurrealDB instance.
	CreateDatabase(ctx context.Context, database string) error

	// DropNamespace deletes a namespace from the SurrealDB instance.
	DropNamespace(ctx context.Context, namespace string) error

	// DropDatabase deletes a database from the SurrealDB instance.
	DropDatabase(ctx context.Context, database string) error

	// Query executes a Surreal query with the provided variables and returns the query results as a slice of interfaces{}.
	// It returns an error if the query execution fails.
	Query(ctx context.Context, query string, vars map[string]any) ([]any, error)

	// Create inserts a new record into the specified table and returns the created record as a map.
	// It returns an error if the operation fails.
	Create(ctx context.Context, table string, data any) (map[string]any, error)

	// Update modifies an existing record in the specified table by its ID with the provided data.
	// It returns the updated record as an interface and an error if the operation fails.
	Update(ctx context.Context, table string, id string, data any) (any, error)

	// Delete removes a record from the specified table by its ID.
	// It returns the result of the delete operation as an interface and an error if the operation fails.
	Delete(ctx context.Context, table string, id string) (any, error)

	// Select retrieves all records from the specified table.
	// It returns a slice of maps representing the records and an error if the operation fails.
	Select(ctx context.Context, table string) ([]map[string]any, error)

	HealthChecker
}

// SurrealBDProvider is an interface that extends SurrealDB with additional methods for logging, metrics, or connection management.
// It is typically used for initializing and managing SurrealDB-based data sources.
type SurrealBDProvider interface {
	SurrealDB

	provider
}

type provider interface {
	// UseLogger sets the logger for the Cassandra client.
	UseLogger(logger any)

	// UseMetrics sets the metrics for the Cassandra client.
	UseMetrics(metrics any)

	// UseTracer sets the tracer for the Cassandra client.
	UseTracer(tracer any)

	// Connect establishes a connection to Cassandra and registers metrics using the provided configuration when the client was Created.
	Connect()
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

type OpenTSDBProvider interface {
	OpenTSDB
	provider
}

// OpenTSDB provides methods for GoFr applications to communicate with OpenTSDB
// through its REST APIs. Each method corresponds to an API endpoint defined in the
// OpenTSDB documentation (http://opentsdb.net/docs/build/html/api_http/index.html#api-endpoints).
type OpenTSDB interface {
	// HealthChecker verifies if the OpenTSDB server is reachable.
	// Returns an error if the server is unreachable, otherwise nil.
	HealthChecker

	// PutDataPoints sends data to the 'POST /api/put' endpoint to store metrics in OpenTSDB.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - data: A slice of DataPoint objects; must contain at least one entry.
	// - queryParam: Specifies the response format:
	//   - client.PutRespWithSummary: Requests a summary response.
	//   - client.PutRespWithDetails: Requests detailed response information.
	//   - Empty string (""): No additional response details.
	// - res: A pointer to PutResponse, where the server's response will be stored.
	//
	// Returns:
	// - Error if parameters are invalid, response parsing fails, or if connectivity issues occur.
	PutDataPoints(ctx context.Context, data any, queryParam string, res any) error

	// QueryDataPoints retrieves data using the 'GET /api/query' endpoint based on the specified parameters.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - param: An instance of QueryParam with query parameters for filtering data.
	// - res: A pointer to QueryResponse, where the server's response will be stored.
	//
	// Returns:
	// - Error if parameters are invalid, response parsing fails, or if connectivity issues occur.
	QueryDataPoints(ctx context.Context, param any, res any) error

	// QueryLatestDataPoints fetches the latest data point(s) using the 'GET /api/query/last' endpoint,
	// supported in OpenTSDB v2.2 and later.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - param: An instance of QueryLastParam with query parameters for the latest data point.
	// - res: A pointer to QueryLastResponse, where the server's response will be stored.
	//
	// Returns:
	// - Error if parameters are invalid, response parsing fails, or if connectivity issues occur.
	QueryLatestDataPoints(ctx context.Context, param any, res any) error

	// GetAggregators retrieves available aggregation functions using the 'GET /api/aggregators' endpoint.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - res: A pointer to AggregatorsResponse, where the server's response will be stored.
	//
	// Returns:
	// - Error if response parsing fails or if connectivity issues occur.
	GetAggregators(ctx context.Context, res any) error

	// QueryAnnotation retrieves a single annotation from OpenTSDB using the 'GET /api/annotation' endpoint.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - queryAnnoParam: A map of parameters for the annotation query, such as client.AnQueryStartTime, client.AnQueryTSUid.
	// - res: A pointer to AnnotationResponse, where the server's response will be stored.
	//
	// Returns:
	// - Error if parameters are invalid, response parsing fails, or if connectivity issues occur.
	QueryAnnotation(ctx context.Context, queryAnnoParam map[string]any, res any) error

	// PostAnnotation creates or updates an annotation in OpenTSDB using the 'POST /api/annotation' endpoint.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - annotation: The annotation to be created or updated.
	// - res: A pointer to AnnotationResponse, where the server's response will be stored.
	//
	// Returns:
	// - Error if parameters are invalid, response parsing fails, or if connectivity issues occur.
	PostAnnotation(ctx context.Context, annotation any, res any) error

	// PutAnnotation creates or replaces an annotation in OpenTSDB using the 'PUT /api/annotation' endpoint.
	// Fields not included in the request will be reset to default values.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - annotation: The annotation to be created or replaced.
	// - res: A pointer to AnnotationResponse, where the server's response will be stored.
	//
	// Returns:
	// - Error if parameters are invalid, response parsing fails, or if connectivity issues occur.
	PutAnnotation(ctx context.Context, annotation any, res any) error

	// DeleteAnnotation removes an annotation from OpenTSDB using the 'DELETE /api/annotation' endpoint.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - annotation: The annotation to be deleted.
	// - res: A pointer to AnnotationResponse, where the server's response will be stored.
	//
	// Returns:
	// - Error if parameters are invalid, response parsing fails, or if connectivity issues occur.
	DeleteAnnotation(ctx context.Context, annotation any, res any) error
}

type ScyllaDB interface {
	// Query executes a CQL (Cassandra Query Language) query on the ScyllaDB cluster
	// and stores the result in the provided destination variable `dest`.
	// Accepts pointer to struct or slice as dest parameter for single and multiple
	Query(dest any, stmt string, values ...any) error
	// QueryWithCtx executes the query with a context and binds the result into dest parameter.
	// Accepts pointer to struct or slice as dest parameter for single and multiple rows retrieval respectively.
	QueryWithCtx(ctx context.Context, dest any, stmt string, values ...any) error
	// Exec executes a CQL statement (e.g., INSERT, UPDATE, DELETE) on the ScyllaDB cluster without returning any result.
	Exec(stmt string, values ...any) error
	// ExecWithCtx executes a CQL statement with the provided context and without returning any result.
	ExecWithCtx(ctx context.Context, stmt string, values ...any) error
	// ExecCAS executes a lightweight transaction (i.e. an UPDATE or INSERT statement containing an IF clause).
	// If the transaction fails because the existing values did not match, the previous values will be stored in dest.
	// Returns true if the query is applied otherwise false.
	// Returns false and error if any error occur while executing the query.
	// Accepts only pointer to struct and built-in types as the dest parameter.
	ExecCAS(dest any, stmt string, values ...any) (bool, error)
	// NewBatch initializes a new batch operation with the specified name and batch type.
	NewBatch(name string, batchType int) error
	// NewBatchWithCtx takes context,name and batchtype and return error.
	NewBatchWithCtx(_ context.Context, name string, batchType int) error
	// BatchQuery executes a batch query in the ScyllaDB cluster with the specified name, statement, and values.
	BatchQuery(name, stmt string, values ...any) error
	// BatchQueryWithCtx executes a batch query with the provided context.
	BatchQueryWithCtx(ctx context.Context, name, stmt string, values ...any) error
	// ExecuteBatchWithCtx executes a batch with context and name returns error.
	ExecuteBatchWithCtx(ctx context.Context, name string) error
	// HealthChecker defines the HealthChecker interface.
	HealthChecker
}

type ScyllaDBProvider interface {
	ScyllaDB
	provider
}

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
	// Parameters:
	//   - ctx: Request context for tracing and cancellation.
	//   - database: Name of the database where the graph will be created.
	//   - graph: Name of the graph to be created.
	//   - edgeDefinitions: Pointer to EdgeDefinition struct containing edge definitions.
	//
	// Returns an error if the edgeDefinitions parameter is not of type *EdgeDefinition or is nil.
	CreateGraph(ctx context.Context, database, graph string, edgeDefinitions any) error
	// DropGraph deletes an existing graph from a database.
	DropGraph(ctx context.Context, database, graph string) error

	// CreateDocument creates a new document in the specified collection.
	CreateDocument(ctx context.Context, dbName, collectionName string, document any) (string, error)
	// GetDocument retrieves a document by its ID from the specified collection.
	GetDocument(ctx context.Context, dbName, collectionName, documentID string, result any) error
	// UpdateDocument updates an existing document in the specified collection.
	UpdateDocument(ctx context.Context, dbName, collectionName, documentID string, document any) error
	// DeleteDocument deletes a document by its ID from the specified collection.
	DeleteDocument(ctx context.Context, dbName, collectionName, documentID string) error

	// GetEdges fetches all edges connected to a given vertex in the specified edge collection.
	//
	// Parameters:
	//   - ctx: Request context for tracing and cancellation.
	//   - dbName: Database name.
	//   - graphName: Graph name.
	//   - edgeCollection: Edge collection name.
	//   - vertexID: Full vertex ID (e.g., "persons/16563").
	//   - resp: Pointer to `*EdgeDetails` to store results.
	//
	// Returns an error if input is invalid, `resp` is of the wrong type, or the query fails.
	GetEdges(ctx context.Context, dbName, graphName, edgeCollection, vertexID string, resp any) error

	// Query executes an AQL query and binds the results.
	//
	// Parameters:
	//   - ctx: Request context for tracing and cancellation.
	//   - dbName: Name of the database where the query will be executed.
	//   - query: AQL query string to be executed.
	//   - bindVars: Map of bind variables to be used in the query.
	//   - result: Pointer to a slice of maps where the query results will be stored.
	//	 - options : A flexible map[string]any to customize query behavior. Keys should be in camelCase
	//     and correspond to fields in ArangoDB’s QueryOptions and QuerySubOptions structs.
	//
	// Returns an error if the database connection fails, the query execution fails, or
	// the result parameter is not a pointer to a slice of maps.
	Query(ctx context.Context, dbName string, query string, bindVars map[string]any, result any, options ...map[string]any) error

	HealthChecker
}

// ArangoDBProvider is an interface that extends ArangoDB with additional methods for logging, metrics, and connection management.
type ArangoDBProvider interface {
	ArangoDB

	provider
}

// Elasticsearch defines all the operations GoFr users need.
type Elasticsearch interface {
	// CreateIndex creates a new index with optional mapping/settings.
	CreateIndex(ctx context.Context, index string, settings map[string]any) error

	// DeleteIndex deletes an existing index.
	DeleteIndex(ctx context.Context, index string) error

	// IndexDocument indexes (creates or replaces) a single document.
	IndexDocument(ctx context.Context, index, id string, document any) error

	// GetDocument retrieves a single document by ID.
	// Returns the raw JSON as a map.
	GetDocument(ctx context.Context, index, id string) (map[string]any, error)

	// UpdateDocument applies a partial update to an existing document.
	UpdateDocument(ctx context.Context, index, id string, update map[string]any) error

	// DeleteDocument removes a document by ID.
	DeleteDocument(ctx context.Context, index, id string) error

	// Bulk executes multiple indexing/updating/deleting operations in one request.
	// Each entry in `operations` should be a JSON‑serializable object
	// following the Elasticsearch bulk API format.
	Bulk(ctx context.Context, operations []map[string]any) (map[string]any, error)

	// Search executes a query against one or more indices.
	// Returns the entire response JSON as a map.
	Search(ctx context.Context, indices []string, query map[string]any) (map[string]any, error)

	HealthChecker
}

// ElasticsearchProvider an interface that extends Elasticsearch with additional methods for logging, metrics, and connection management.
type ElasticsearchProvider interface {
	Elasticsearch

	provider
}

// Couchbase defines the methods for interacting with a Couchbase database.
type Couchbase interface {
	// Get retrieves a document by its key from the specified bucket.
	// The result parameter should be a pointer to the struct where the document will be unmarshaled.
	Get(ctx context.Context, key string, result any) error

	// InsertOne inserts a new document in the collection.
	Insert(ctx context.Context, key string, document, result any) error

	// Upsert inserts a new document or replaces an existing one in the specified bucket.
	// The document parameter can be any Go type that can be marshaled into JSON.
	Upsert(ctx context.Context, key string, document any, result any) error

	// Remove deletes a document by its key from the specified bucket.
	Remove(ctx context.Context, key string) error

	// Query executes a N1QL query against the Couchbase cluster.
	// The statement is the N1QL query string, and params are any query parameters.
	// The result parameter should be a pointer to a slice of structs or maps where the query results will be unmarshaled.
	Query(ctx context.Context, statement string, params map[string]any, result any) error

	// AnalyticsQuery executes an Analytics query against the Couchbase Analytics service.
	// The statement is the Analytics query string, and params are any query parameters.
	// The result parameter should be a pointer to a slice of structs or maps where the query results will be unmarshaled.
	AnalyticsQuery(ctx context.Context, statement string, params map[string]any, result any) error

	RunTransaction(ctx context.Context, logic func(attempt any) error) (any, error)

	Close(opts any) error

	HealthChecker
}

// CouchbaseProvider is an interface that extends Couchbase with additional methods
// for logging, metrics, tracing, and connection management, aligning with other
// data source providers in your package.
type CouchbaseProvider interface {
	Couchbase

	provider
}

// DBResolverProvider defines an interface for SQL read/write splitting providers.
type DBResolverProvider interface {
	GetResolver() DB

	provider
}

// InfluxDB defines the operations required to interact with an InfluxDB instance.
type InfluxDB interface {
	// CreateOrganization create new bucket in the influxdb
	CreateOrganization(ctx context.Context, org string) (string, error)

	// DeleteOrganization deletes a organization under the specified organization.
	DeleteOrganization(ctx context.Context, orgID string) error

	// ListOrganization list all the available organization
	ListOrganization(ctx context.Context) (orgs map[string]string, err error)

	// WritePoint writes one time-series points to a bucket.
	// 'points' should follow the line protocol format or structured map format.
	WritePoint(ctx context.Context, org, bucket string,
		measurement string,
		tags map[string]string,
		fields map[string]any,
		timestamp time.Time) error

	// Query runs a Flux query and returns the result as a slice of maps,
	// where each map is a row with column name-value pairs.
	Query(ctx context.Context, org, fluxQuery string) ([]map[string]any, error)

	// CreateBucket creates a new bucket under the specified organization.
	CreateBucket(ctx context.Context, org, bucket string) (string, error)

	// DeleteBucket deletes a bucketId with bucketID
	DeleteBucket(ctx context.Context, bucketID string) error

	// ListBuckets lists all buckets under the specified organization.
	ListBuckets(ctx context.Context, org string) (map[string]string, error)

	// Ping checks if the InfluxDB instance is reachable and healthy.
	Ping(ctx context.Context) (bool, error)

	HealthChecker
}

// InfluxDBProvider an interface that extends InfluxDB with additional methods for logging, metrics, and connection management.
type InfluxDBProvider interface {
	InfluxDB

	provider
}
