package couchbase

import (
	"context"
	"time"

	gocb "github.com/couchbase/gocb/v2"
)

type Couchbase interface {
	// Get retrieves a document by its key from the specified bucket.
	// The result parameter should be a pointer to the struct where the document will be unmarshaled.
	Get(ctx context.Context, bucket, key string, result any) error

	// Upsert inserts a new document or replaces an existing one in the specified bucket.
	// The document parameter can be any Go type that can be marshaled into JSON.
	Upsert(ctx context.Context, bucket, key string, document any, result any) error

	// Remove deletes a document by its key from the specified bucket.
	Remove(ctx context.Context, bucket, key string) error

	// Query executes a N1QL query against the Couchbase cluster.
	// The statement is the N1QL query string, and params are any query parameters.
	// The result parameter should be a pointer to a slice of structs or maps where the query results will be unmarshaled.
	Query(ctx context.Context, statement string, params map[string]any, result any) error

	// AnalyticsQuery executes an Analytics query against the Couchbase Analytics service.
	// The statement is the Analytics query string, and params are any query parameters.
	// The result parameter should be a pointer to a slice of structs or maps where the query results will be unmarshaled.
	AnalyticsQuery(ctx context.Context, statement string, params map[string]any, result any) error

	Close(opts any) error
}

// clusterProvider is an interface that abstracts the gocb.Cluster for easier testing.
type clusterProvider interface {
	Bucket(bucketName string) bucketProvider
	Query(statement string, opts *gocb.QueryOptions) (resultProvider, error)
	AnalyticsQuery(statement string, opts *gocb.AnalyticsOptions) (resultProvider, error)
	WaitUntilReady(timeout time.Duration, opts *gocb.WaitUntilReadyOptions) error
	Ping(opts *gocb.PingOptions) (*gocb.PingResult, error)
	Close(opts *gocb.ClusterCloseOptions) error
}

// resultProvider is an interface that abstracts gocb.QueryResult and gocb.AnalyticsResult for easier testing.
type resultProvider interface {
	Next() bool
	Row(value any) error
	Err() error
	Close() error
}

// bucketProvider is an interface that abstracts the gocb.Bucket for easier testing.
type bucketProvider interface {
	Collection(collectionName string) collectionProvider
	DefaultCollection() collectionProvider
	WaitUntilReady(timeout time.Duration, opts *gocb.WaitUntilReadyOptions) error
}

// collectionProvider is an interface that abstracts the gocb.Collection for easier testing.
type collectionProvider interface {
	Upsert(key string, value any, opts *gocb.UpsertOptions) (*gocb.MutationResult, error)
	Get(key string, opts *gocb.GetOptions) (getResultProvider, error)
	Remove(key string, opts *gocb.RemoveOptions) (*gocb.MutationResult, error)
}

type getResultProvider interface {
	Content(value any) error
}
