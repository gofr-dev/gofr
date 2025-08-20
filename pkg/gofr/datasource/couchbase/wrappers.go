package couchbase

import (
	"time"

	"github.com/couchbase/gocb/v2"
)

// analyticsResultWrapper is a wrapper around gocb.AnalyticsResult to implement the analyticsResultProvider interface.
type analyticsResultWrapper struct {
	*gocb.AnalyticsResult
}

// clusterWrapper is a wrapper around gocb.Cluster to implement the clusterProvider interface.
type clusterWrapper struct {
	*gocb.Cluster
}

// collectionWrapper is a wrapper around gocb.Collection to implement the collectionProvider interface.
type collectionWrapper struct {
	*gocb.Collection
}

// bucketWrapper is a wrapper around gocb.Bucket to implement the bucketProvider interface.
type bucketWrapper struct {
	*gocb.Bucket
}

// queryResultWrapper is a wrapper around gocb.QueryResult to implement the queryResultProvider interface.
type queryResultWrapper struct {
	*gocb.QueryResult
}

// scopeWrapper is a wrapper around gocb.Scope to implement the scopeProvider interface.
type scopeWrapper struct {
	*gocb.Scope
}

// transactionsWrapper is a wrapper around gocb.Transactions to implement the transactionsProvider interface.
type transactionsWrapper struct {
	*gocb.Transactions
}

type getResultWrapper struct {
	*gocb.GetResult
}

// Bucket returns a bucketProvider for the specified bucket name.
func (cw *clusterWrapper) Bucket(bucketName string) bucketProvider {
	return &bucketWrapper{cw.Cluster.Bucket(bucketName)}
}

// Query executes a N1QL query against the Couchbase cluster.
func (cw *clusterWrapper) Query(statement string, opts *gocb.QueryOptions) (resultProvider, error) {
	res, err := cw.Cluster.Query(statement, opts)
	if err != nil {
		return nil, err
	}

	return &queryResultWrapper{res}, nil
}

// AnalyticsQuery executes an Analytics query against the Couchbase Analytics service.
func (cw *clusterWrapper) AnalyticsQuery(statement string, opts *gocb.AnalyticsOptions) (resultProvider, error) {
	res, err := cw.Cluster.AnalyticsQuery(statement, opts)
	if err != nil {
		return nil, err
	}

	return &analyticsResultWrapper{res}, nil
}

func (cw *clusterWrapper) Close(opts *gocb.ClusterCloseOptions) error {
	return cw.Cluster.Close(opts)
}

func (cw *clusterWrapper) Ping(opts *gocb.PingOptions) (*gocb.PingResult, error) {
	return cw.Cluster.Ping(opts)
}

// WaitUntilReady waits until the cluster is ready.
func (cw *clusterWrapper) WaitUntilReady(timeout time.Duration, opts *gocb.WaitUntilReadyOptions) error {
	return cw.Cluster.WaitUntilReady(timeout, opts)
}

func (cw *clusterWrapper) Transactions() transactionsProvider {
	return &transactionsWrapper{cw.Cluster.Transactions()}
}

// Collection returns a collectionProvider for the specified collection name.
func (bw *bucketWrapper) Collection(name string) collectionProvider {
	return &collectionWrapper{bw.Bucket.Collection(name)}
}

// DefaultCollection returns the default collectionProvider for the bucket.
func (bw *bucketWrapper) DefaultCollection() collectionProvider {
	return &collectionWrapper{bw.Bucket.DefaultCollection()}
}

func (bw *bucketWrapper) Scope(name string) scopeProvider {
	return &scopeWrapper{bw.Bucket.Scope(name)}
}

func (bw *bucketWrapper) WaitUntilReady(timeout time.Duration, opts *gocb.WaitUntilReadyOptions) error {
	return bw.Bucket.WaitUntilReady(timeout, opts)
}

// Next returns true if there are more rows to be retrieved.
func (qrw *queryResultWrapper) Next() bool {
	return qrw.QueryResult.Next()
}

// Row unmarshals the current row into the value pointed to by the result parameter.
func (qrw *queryResultWrapper) Row(value any) error {
	return qrw.QueryResult.Row(value)
}

// Err returns the error, if any, that occurred during the rows iteration.
func (qrw *queryResultWrapper) Err() error {
	return qrw.QueryResult.Err()
}

// Close closes the query result.
func (qrw *queryResultWrapper) Close() error {
	return qrw.QueryResult.Close()
}

// Next returns true if there are more rows to be retrieved.
func (arw *analyticsResultWrapper) Next() bool {
	return arw.AnalyticsResult.Next()
}

// Row unmarshals the current row into the value pointed to by the result parameter.
func (arw *analyticsResultWrapper) Row(value any) error {
	return arw.AnalyticsResult.Row(value)
}

// Err returns the error, if any, that occurred during the rows iteration.
func (arw *analyticsResultWrapper) Err() error {
	return arw.AnalyticsResult.Err()
}

// Close closes the analytics result.
func (arw *analyticsResultWrapper) Close() error {
	return arw.AnalyticsResult.Close()
}

func (sw *scopeWrapper) Collection(name string) collectionProvider {
	return &collectionWrapper{sw.Scope.Collection(name)}
}

func (tw *transactionsWrapper) Run(
	logic func(*gocb.TransactionAttemptContext) error, opts *gocb.TransactionOptions,
) (*gocb.TransactionResult, error) {
	return tw.Transactions.Run(logic, opts)
}

func (grw *getResultWrapper) Content(value any) error {
	return grw.GetResult.Content(value)
}

// Upsert performs an upsert operation on the collection.
func (cw *collectionWrapper) Upsert(key string, value any, opts *gocb.UpsertOptions) (*gocb.MutationResult, error) {
	return cw.Collection.Upsert(key, value, opts)
}

// Get performs a get operation on the collection.
func (cw *collectionWrapper) Get(key string, opts *gocb.GetOptions) (getResultProvider, error) {
	res, err := cw.Collection.Get(key, opts)
	if err != nil {
		return nil, err
	}

	return &getResultWrapper{res}, nil
}

// Remove performs a remove operation on the collection.
func (cw *collectionWrapper) Remove(key string, opts *gocb.RemoveOptions) (*gocb.MutationResult, error) {
	return cw.Collection.Remove(key, opts)
}

func (cw *collectionWrapper) Insert(key string, value any, opts *gocb.InsertOptions) (*gocb.MutationResult, error) {
	return cw.Collection.Insert(key, value, opts)
}
