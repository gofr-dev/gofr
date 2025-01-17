package arango

import (
	"context"

	"github.com/arangodb/go-driver/v2/arangodb"
)

type Arango interface {
	Connect()

	User(ctx context.Context, username string) (arangodb.User, error)
	Database(ctx context.Context, name string) (arangodb.Database, error)
	Version(ctx context.Context) (arangodb.VersionInfo, error)

	CreateUser(ctx context.Context, name string, options *arangodb.UserOptions) (arangodb.User, error)
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

	CreateDocument(ctx context.Context, dbName, collectionName string, document interface{}) (string, error)
	GetDocument(ctx context.Context, dbName, collectionName, documentID string, result interface{}) error
	UpdateDocument(ctx context.Context, dbName, collectionName, documentID string, document interface{}) error
	DeleteDocument(ctx context.Context, dbName, collectionName, documentID string) error

	CreateEdgeDocument(ctx context.Context, dbName, collectionName string, from, to string, document interface{}) (string, error)

	CreateGraph(ctx context.Context, database, graph string, edgeDefinitions []EdgeDefinition) error
	DropGraph(ctx context.Context, database, graph string) error
	ListGraphs(ctx context.Context, database string) ([]string, error)

	// Query operations
	Query(ctx context.Context, dbName string, query string, bindVars map[string]interface{}, result interface{}) error

	HealthCheck(ctx context.Context) (interface{}, error)
}

type User interface {
	SetDatabaseAccess(ctx context.Context, database string, grant arangodb.Grant) error
	SetCollectionAccess(ctx context.Context, database, collection string, grant arangodb.Grant) error
}

type Database interface {
	Collection(ctx context.Context, name string) (arangodb.Collection, error)
	Collections(ctx context.Context) ([]arangodb.Collection, error)
	CreateCollection(ctx context.Context, name string, options *arangodb.CreateCollectionProperties) (arangodb.Collection, error)
	Graph(ctx context.Context, name string, options *arangodb.GraphDefinition) (arangodb.Graph, error)
	Graphs(ctx context.Context) (arangodb.Cursor, error)
	Remove(ctx context.Context) error
}

type Collection interface {
	CreateDocument(ctx context.Context, document interface{}) (arangodb.DocumentMeta, error)
	ReadDocument(ctx context.Context, key string, result interface{}) (arangodb.DocumentMeta, error)
	UpdateDocument(ctx context.Context, key string, document interface{}) (arangodb.DocumentMeta, error)
	DeleteDocument(ctx context.Context, key string) (arangodb.DocumentMeta, error)
	Remove(ctx context.Context) error
	Truncate(ctx context.Context) error
}

type Graph interface {
	Remove(ctx context.Context, options *arangodb.RemoveGraphOptions) error
}
