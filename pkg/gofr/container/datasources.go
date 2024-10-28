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
// through its REST APIs. Each method corresponds to an API endpoint as defined in
// the OpenTSDB documentation (http://opentsdb.net/docs/build/html/api_http/index.html#api-endpoints).
type OpenTSDB interface {

	// HealthChecker checks if the target OpenTSDB server is reachable.
	// It returns an error if the server is unreachable, otherwise returns nil.
	HealthChecker

	// Put handles the 'POST /api/put' endpoint, allowing the storage of data in OpenTSDB.
	//
	// Parameters:
	// - data: A slice of DataPoint objects, which must contain at least one instance.
	// - queryParam: Can be one of the following:
	//   - client.PutRespWithSummary: Requests a summary of the put operation.
	//   - client.PutRespWithDetails: Requests detailed information about the put operation.
	//   - An empty string (""): Indicates no additional response details are required.
	//
	// Return:
	// - On success, it returns a pointer to a PutResponse, along with the HTTP status code and relevant response information.
	// - On failure (due to invalid parameters, response parsing errors, or OpenTSDB connectivity issues), it returns an error.
	//
	// Notes:
	// - Use 'PutRespWithSummary' to receive summarized information about the data that was stored.
	// - Use 'PutRespWithDetails' for a more comprehensive breakdown of the put operation.
	Put(ctx context.Context, data any, queryParam string) (any, error)

	// Query implements the 'GET /api/query' endpoint for extracting data
	// in various formats based on the selected serializer.
	//
	// Parameters:
	// - param: An instance of QueryParam containing the current query parameters.
	//
	// Returns:
	// - *QueryResponse on success (status code and response info).
	// - Error on failure (invalid parameters, response parsing failure, or OpenTSDB connection issues).
	Query(ctx context.Context, param any) (any, error)

	// QueryLast is the implementation of 'GET /api/query/last' endpoint.
	// It is introduced firstly in v2.1, and fully supported in v2.2. So it should be aware that this api works
	// well since v2.2 of opentsdb.
	//
	// param is a instance of QueryLastParam holding current query parameters.
	//
	// When query operation is successful, a pointer of QueryLastResponse will be returned with the corresponding
	// status code and response info. Otherwise, an error instance will be returned, when the given parameter
	// is invalid, it failed to parse the response, or OpenTSDB is un-connectable right now.
	QueryLast(ctx context.Context, param any) (any, error)

	// Aggregators is the implementation of 'GET /api/aggregators' endpoint.
	// It simply lists the names of implemented aggregation functions used in time series queries.
	//
	// When query operation is successful, a pointer of AggregatorsResponse will be returned with the corresponding
	// status code and response info. Otherwise, an error instance will be returned, when it failed to parse the
	// response, or OpenTSDB is un-connectable right now.
	Aggregators(ctx context.Context) (any, error)

	// Suggest is the implementation of 'GET /api/suggest' endpoint.
	// It provides a means of implementing an "auto-complete" call that can be accessed repeatedly as a user
	// types a request in a GUI. It does not offer full text searching or wildcards, rather it simply matches
	// the entire string passed in the query on the first characters of the stored data.
	// For example, passing a query of type=metrics&q=sys will return the top 25 metrics in the system that start with sys.
	// Matching is case-sensitive, so sys will not match System.CPU. Results are sorted alphabetically.
	//
	// sugParam is an instance of SuggestParam storing parameters by invoking /api/suggest.
	//
	// When query operation is successful, a pointer of SuggestResponse will be returned with the corresponding
	// status code and response info. Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parse the response, or OpenTSDB is un-connectable right now.
	Suggest(ctx context.Context, sugParm any) (any, error)

	// Dropcaches is the implementation of 'GET /api/dropcaches' endpoint.
	// It purges the in-memory data cached in OpenTSDB. This includes all UID to name
	// and name to UID maps for metrics, tag names and tag values.
	//
	// When query operation is successful, a pointer of DropcachesResponse will be returned with the corresponding
	// status code and response info. Otherwise, an error instance will be returned, when it failed to parese the
	// response, or OpenTSDB is un-connectable right now.
	Dropcaches(ctx context.Context) (any, error)

	// QueryAnnotation is the implementation of 'GET /api/annotation' endpoint.
	// It retrieves a single annotation stored in the OpenTSDB backend.
	//
	// queryAnnoParam is a map storing parameters of a target queried annotation.
	// The key can be such as client.AnQueryStartTime, client.AnQueryTSUid.
	//
	// When query operation is handling properly by the OpenTSDB backend, a pointer of AnnotationResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parse the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only response by opentsdb-client, not the OpenTSDB backend.
	QueryAnnotation(ctx context.Context, queryAnnoParam map[string]any) (any, error)

	// UpdateAnnotation is the implementation of 'POST /api/annotation' endpoint.
	// It creates or modifies an annotation stored in the OpenTSDB backend.
	//
	// annotation is an annotation to be processed in the OpenTSDB backend.
	//
	// When modification operation is handling properly by the OpenTSDB backend, a pointer of AnnotationResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parse the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only response by opentsdb-client, not the OpenTSDB backend.
	UpdateAnnotation(ctx context.Context, annotation any) (any, error)

	// DeleteAnnotation is the implementation of 'DELETE /api/annotation' endpoint.
	// It deletes an annotation stored in the OpenTSDB backend.
	//
	// annotation is an annotation to be deleted in the OpenTSDB backend.
	//
	// When deleting operation is handling properly by the OpenTSDB backend, a pointer of AnnotationResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parse the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only response by opentsdb-client, not the OpenTSDB backend.
	DeleteAnnotation(ctx context.Context, annotation any) (any, error)

	// BulkUpdateAnnotations is the implementation of 'POST /api/annotation/bulk' endpoint.
	// It creates or modifies a list of annotation stored in the OpenTSDB backend.
	//
	// annotations is a list of annotations to be processed (to be created or modified) in the OpenTSDB backend.
	//
	// When bulk modification operation is handling properly by the OpenTSDB backend, a pointer of BulkAnnotatResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parse the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only response by opentsdb-client, not the OpenTSDB backend.
	BulkUpdateAnnotations(ctx context.Context, annotations any) (any, error)

	// BulkDeleteAnnotations is the implementation of 'DELETE /api/annotation/bulk' endpoint.
	// It deletes a list of annotation stored in the OpenTSDB backend.
	//
	// bulkDelParam contains the bulk deleting info in current invoking 'DELETE /api/annotation/bulk'.
	//
	// When bulk deleting operation is handling properly by the OpenTSDB backend, a pointer of BulkAnnotatResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parse the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only response by opentsdb-client, not the OpenTSDB backend.
	BulkDeleteAnnotations(ctx context.Context, bulkDelParam any) (any, error)

	// QueryUIDMetaData is the implementation of 'GET /api/uid/uidmeta' endpoint.
	// It retrieves a single UIDMetaData stored in the OpenTSDB backend with the given query parameters.
	//
	// metaQueryParam is a map storing parameters of a target queried UIDMetaData.
	// It must contain two key/value pairs with the key "uid" and "type".
	// "type" should be one of client.TypeMetrics ("metric"), client.TypeTagk ("tagk"), and client.TypeTagv ("tagv")
	//
	// When query operation is handling properly by the OpenTSDB backend, a pointer of UIDMetaDataResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parse the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only response by opentsdb-client, not the OpenTSDB backend.
	QueryUIDMetaData(ctx context.Context, metaQueryParam map[string]string) (any, error)

	// UpdateUIDMetaData is the implementation of 'POST /api/uid/uidmeta' endpoint.
	// It modifies a UIDMetaData.
	//
	// uidMetaData is an instance of UIDMetaData to be modified
	//
	// When update operation is handling properly by the OpenTSDB backend, a pointer of UIDMetaDataResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parse the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only response by opentsdb-client, not the OpenTSDB backend.
	UpdateUIDMetaData(ctx context.Context, uidMetaData any) (any, error)

	// DeleteUIDMetaData is the implementation of 'DELETE /api/uid/uidmeta' endpoint.
	// It deletes a target UIDMetaData.
	//
	// uidMetaData is an instance of UIDMetaData whose correspondence is to be deleted.
	// The values of uid and type in uidMetaData is required.
	//
	// When delete operation is handling properly by the OpenTSDB backend, a pointer of UIDMetaDataResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parse the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only response by opentsdb-client, not the OpenTSDB backend.
	DeleteUIDMetaData(ctx context.Context, uidMetaData any) (any, error)

	// AssignUID is the implementation of 'POST /api/uid/assign' endpoint.
	// It enables assigning UIDs to new metrics, tag names and tag values. Multiple types and names can be provided
	// in a single call and the API will process each name individually, reporting which names were assigned UIDs
	// successfully, along with the UID assigned, and which failed due to invalid characters or had already been assigned.
	// Assignment can be performed via query string or content data.
	//
	// assignParam is an instance of UIDAssignParam holding the parameters to invoke 'POST /api/uid/assign'.
	//
	// When assign operation is handling properly by the OpenTSDB backend, a pointer of UIDAssignResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parse the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only response by opentsdb-client, not the OpenTSDB backend.
	AssignUID(ctx context.Context, assignParam any) (any, error)

	// QueryTSMetaData is the implementation of 'GET /api/uid/tsmeta' endpoint.
	// It retrieves a single TSMetaData stored in the OpenTSDB backend with the given query parameters.
	//
	// tsuid is a tsuid of a target queried TSMetaData.
	//
	// When query operation is handling properly by the OpenTSDB backend, a pointer of TSMetaDataResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, if the given parameter is invalid,
	// or when it failed to parse the response, or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only response by opentsdb-client, not the OpenTSDB backend.
	QueryTSMetaData(ctx context.Context, tsuid string) (any, error)

	// UpdateTSMetaData is the implementation of 'POST /api/uid/tsmeta' endpoint.
	// It modifies a target TSMetaData with the given fields.
	//
	// tsMetaData is an instance of UIDMetaData whose correspondence is to be modified
	//
	// When update operation is handling properly by the OpenTSDB backend, a pointer of TSMetaDataResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, when it failed to parse the response,
	// or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only response by opentsdb-client, not the OpenTSDB backend.
	UpdateTSMetaData(ctx context.Context, tsMetaData any) (any, error)

	// DeleteTSMetaData is the implementation of 'DELETE /api/uid/tsmeta' endpoint.
	// It deletes a target TSMetaData.
	//
	// tsMetaData is an instance of UIDMetaData whose correspondence is to be deleted
	//
	// When delete operation is handling properly by the OpenTSDB backend, a pointer of TSMetaDataResponse
	// will be returned with the corresponding status code and response info (including the potential error
	// messages replied by OpenTSDB).
	//
	// Otherwise, an error instance will be returned, when it failed to parse the response,
	// or OpenTSDB is un-connectable right now.
	//
	// Note that: the returned non-nil error instance is only response by opentsdb-client, not the OpenTSDB backend.
	DeleteTSMetaData(ctx context.Context, tsMetaData any) (any, error)
}
