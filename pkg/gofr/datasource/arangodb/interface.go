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
	CreateGraph(ctx context.Context, database, graph string, edgeDefinitions any) error
	// DropGraph deletes an existing graph from a database.
	DropGraph(ctx context.Context, database, graph string) error

	CreateDocument(ctx context.Context, dbName, collectionName string, document any) (string, error)
	GetDocument(ctx context.Context, dbName, collectionName, documentID string, result any) error
	UpdateDocument(ctx context.Context, dbName, collectionName, documentID string, document any) error
	DeleteDocument(ctx context.Context, dbName, collectionName, documentID string) error

	// GetEdges retrieves all the edge documents connected to a specific vertex in an ArangoDB graph.
	GetEdges(ctx context.Context, dbName, graphName, edgeCollection, vertexID string, resp any) error

	// Query operations
	Query(ctx context.Context, dbName string, query string, bindVars map[string]any, result any) error

	HealthCheck(ctx context.Context) (any, error)
}
