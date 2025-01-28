package arango

import (
	"context"

	"github.com/arangodb/go-driver/v2/arangodb"
)

type Arango interface {
	Connect()

	User(ctx context.Context, username string) (arangodb.User, error)
	Database(ctx context.Context, name string) (arangodb.Database, error)
	Databases(ctx context.Context) ([]arangodb.Database, error)
	Version(ctx context.Context) (arangodb.VersionInfo, error)

	CreateUser(ctx context.Context, username string, options any) error
	DropUser(ctx context.Context, username string) error
	GrantDB(ctx context.Context, database, username, permission string) error
	GrantCollection(ctx context.Context, database, collection, username, permission string) error

	ListDBs(ctx context.Context) ([]string, error)
	CreateDB(ctx context.Context, database string) error
	DropDB(ctx context.Context, database string) error

	CreateCollection(ctx context.Context, database, collection string, isEdge bool) error
	DropCollection(ctx context.Context, database, collection string) error
	TruncateCollection(ctx context.Context, database, collection string) error
	ListCollections(ctx context.Context, database string) ([]string, error)

	CreateDocument(ctx context.Context, dbName, collectionName string, document any) (string, error)
	GetDocument(ctx context.Context, dbName, collectionName, documentID string, result any) error
	UpdateDocument(ctx context.Context, dbName, collectionName, documentID string, document any) error
	DeleteDocument(ctx context.Context, dbName, collectionName, documentID string) error

	CreateEdgeDocument(ctx context.Context, dbName, collectionName string, from, to string, document any) (string, error)

	CreateGraph(ctx context.Context, database, graph string, edgeDefinitions any) error
	DropGraph(ctx context.Context, database, graph string) error
	ListGraphs(ctx context.Context, database string) ([]string, error)

	// Query operations
	Query(ctx context.Context, dbName string, query string, bindVars map[string]any, result any) error

	HealthCheck(ctx context.Context) (any, error)
}
