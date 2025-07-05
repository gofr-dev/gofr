package couchbase

import (
	"time"

	"github.com/couchbase/gocb/v2"
	"gofr.dev/pkg/gofr/container"
)

// clusterProvider is an interface that abstracts the gocb.Cluster for easier testing.
type clusterProvider interface {
	Bucket(bucketName string) *gocb.Bucket
	AnalyticsQuery(statement string, opts *gocb.AnalyticsOptions) (*gocb.AnalyticsResult, error)
	WaitUntilReady(timeout time.Duration, opts *gocb.WaitUntilReadyOptions) error
}

// bucketProvider is an interface that abstracts the gocb.Bucket for easier testing.
type bucketProvider interface {
	Collection(collectionName string) collectionProvider
	DefaultCollection() collectionProvider
	WaitUntilReady(timeout time.Duration, opts *gocb.WaitUntilReadyOptions) error
}

// gocbProvider is an interface that embeds container.CouchbaseProvider for dependency injection.
type gocbProvider interface {
	container.CouchbaseProvider
}

// collectionProvider is an interface that abstracts the gocb.Collection for easier testing.
type collectionProvider interface {
	Get(id string, opts *gocb.GetOptions) (docOut *gocb.GetResult, errOut error)
	Upsert(id string, val any, opts *gocb.UpsertOptions) (*gocb.MutationResult, error)
	Remove(id string, opts *gocb.RemoveOptions) (mutOut *gocb.MutationResult, errOut error)
}
