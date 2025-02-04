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
