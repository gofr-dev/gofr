package arangodb

import (
	"context"

	"github.com/arangodb/go-driver/v2/arangodb"
)

type ArangoDB interface {
	Connect()

	user(ctx context.Context, username string) (arangodb.User, error)
	database(ctx context.Context, name string) (arangodb.Database, error)
	databases(ctx context.Context) ([]arangodb.Database, error)
	version(ctx context.Context) (arangodb.VersionInfo, error)

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
	//
	// Returns an error if the database connection fails, the query execution fails, or
	// the result parameter is not a pointer to a slice of maps.
	Query(ctx context.Context, dbName string, query string, bindVars map[string]any, result any) error

	// Exists checks if a database, collection, or graph exists.
	// Parameters:
	//   - ctx: Request context for tracing and cancellation.
	//   - name: Name of the database, collection, or graph.
	//   - resourceType: Type of the resource ("database", "collection", "graph").
	//
	// Returns true if the resource exists, otherwise false.
	Exists(ctx context.Context, name, resourceType string) (bool, error)

	HealthCheck(ctx context.Context) (any, error)
}
