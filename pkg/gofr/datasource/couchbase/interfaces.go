package couchbase

import (
	"context"
	"time"

	gocb "github.com/couchbase/gocb/v2"
)

type Couchbase interface {
	Get(ctx context.Context, key string, result any) error
	Insert(ctx context.Context, key string, document, result any) error
	Upsert(ctx context.Context, key string, document any, result any) error
	Remove(ctx context.Context, key string) error
	Query(ctx context.Context, statement string, params map[string]any, result any) error
	AnalyticsQuery(ctx context.Context, statement string, params map[string]any, result any) error
	RunTransaction(ctx context.Context, logic func(attempt *gocb.TransactionAttemptContext) error) (*gocb.TransactionResult, error)
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
	Transactions() transactionsProvider
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
	Scope(name string) scopeProvider
}

// collectionProvider is an interface that abstracts the gocb.Collection for easier testing.
type collectionProvider interface {
	Upsert(key string, value any, opts *gocb.UpsertOptions) (*gocb.MutationResult, error)
	Insert(key string, value any, opts *gocb.InsertOptions) (*gocb.MutationResult, error)
	Get(key string, opts *gocb.GetOptions) (getResultProvider, error)
	Remove(key string, opts *gocb.RemoveOptions) (*gocb.MutationResult, error)
}

type getResultProvider interface {
	Content(value any) error
}

// scopeProvider is an interface that abstracts the gocb.Scope for easier testing.
type scopeProvider interface {
	Collection(name string) collectionProvider
}

// transactionsProvider is an interface that abstracts the gocb.Transactions for easier testing.
type transactionsProvider interface {
	Run(logic func(*gocb.TransactionAttemptContext) error, opts *gocb.TransactionOptions) (*gocb.TransactionResult, error)
}
