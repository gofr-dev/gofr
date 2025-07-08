package couchbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/couchbase/gocb/v2"
	"go.opencensus.io/trace"
)

// Error variables for the couchbase package.
var (
	errStatusDown                 = errors.New("status down")
	errMissingField               = errors.New("missing required field in config")
	errWrongResultType            = errors.New("result must be *gocb.MutationResult or **gocb.MutationResult")
	errBucketNotInitialized       = errors.New("couchbase bucket is not initialized")
	errClustertNotInitialized     = errors.New("couchbase cluster is not initialized")
	errFailedToUnmarshalN1QL      = errors.New("failed to unmarshal N1QL results into target")
	errFailedToUnmarshalAnalytics = errors.New("failed to unmarshal analytics results into target")
)

const defaultTimeout = 5 * time.Second

// Client represents a Couchbase client that interacts with a Couchbase cluster.
type Client struct {
	cluster clusterProvider
	bucket  bucketProvider
	config  *Config
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

// Config holds the configuration parameters for connecting to a Couchbase cluster.
type Config struct {
	Host              string
	User              string
	Password          string
	Bucket            string
	URI               string
	ConnectionTimeout time.Duration
}

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

// Health represents the health status of the Couchbase connection.
type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// New creates a new Couchbase client with the provided configuration.
func New(c *Config) *Client {
	return &Client{config: c}
}

// UseLogger sets the logger for the Couchbase client.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics collector for the Couchbase client.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for the Couchbase client.
func (c *Client) UseTracer(tracer trace.Tracer) {
	c.tracer = tracer
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

// Collection returns a collectionProvider for the specified collection name.
func (bw *bucketWrapper) Collection(name string) collectionProvider {
	return &collectionWrapper{bw.Bucket.Collection(name)}
}

// DefaultCollection returns the default collectionProvider for the bucket.
func (bw *bucketWrapper) DefaultCollection() collectionProvider {
	return &collectionWrapper{bw.Bucket.DefaultCollection()}
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

// Connect establishes a connection to the Couchbase cluster and bucket.
func (c *Client) Connect() {
	uri, err := generateCouchbaseURI(c.config)
	if err != nil {
		c.logger.Errorf("error generating Couchbase URI: %v", err)
		return
	}

	c.logger.Debugf("connecting to Couchbase at %v to bucket %v", c.config.Host, c.config.Bucket)

	timeout := c.config.ConnectionTimeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	cluster, err := gocb.Connect(uri, gocb.ClusterOptions{
		Authenticator: gocb.PasswordAuthenticator{
			Username: c.config.User,
			Password: c.config.Password,
		},
	})
	if err != nil {
		c.logger.Errorf("error while connecting to Couchbase, err:%v", err)
		return
	}

	c.cluster = &clusterWrapper{cluster}

	err = c.cluster.WaitUntilReady(timeout, nil)
	if err != nil {
		c.logger.Errorf("could not connect to Couchbase at %v due to err: %v", c.config.Host, err)
		return
	}

	c.bucket = c.cluster.Bucket(c.config.Bucket)

	err = c.bucket.WaitUntilReady(timeout, nil)
	if err != nil {
		c.logger.Errorf("could not connect to bucket %v at %v due to err: %v", c.config.Bucket, c.config.Host, err)
		return
	}

	couchbaseBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_couchbase_stats", "Response time of Couchbase queries in milliseconds.", couchbaseBuckets...)

	c.logger.Logf("connected to Couchbase at %v to bucket %v", c.config.Host, c.config.Bucket)
}

// generateCouchbaseURI generates the Couchbase connection URI from the configuration.
func generateCouchbaseURI(config *Config) (string, error) {
	if config.URI != "" {
		return config.URI, nil
	}

	if config.Host == "" {
		return "", fmt.Errorf("%w: host is empty", errMissingField)
	}

	return fmt.Sprintf("couchbase://%s", config.Host), nil
}

// HealthCheck performs a health check on the Couchbase cluster.
func (c *Client) HealthCheck() (any, error) {
	h := Health{
		Details: make(map[string]any),
	}

	h.Details["host"] = c.config.Host
	h.Details["bucket"] = c.config.Bucket

	_, err := c.cluster.Ping(nil)
	if err != nil {
		h.Status = "DOWN"
		return &h, errStatusDown
	}

	h.Status = "UP"

	return &h, nil
}

// Upsert inserts a new document or replaces an existing one in the specified bucket.
func (c *Client) Upsert(ctx context.Context, key string, document, result any) error {
	if c.bucket == nil {
		return errBucketNotInitialized
	}

	mr, err := c.bucket.DefaultCollection().Upsert(key, document, &gocb.UpsertOptions{Context: ctx})
	if err != nil {
		return fmt.Errorf("failed to upsert document with key %s: %w", key, err)
	}

	switch r := result.(type) {
	case *gocb.MutationResult:
		*r = *mr
	case **gocb.MutationResult:
		*r = mr
	default:
		return errWrongResultType
	}

	return nil
}

// Upsert performs an upsert operation on the collection.
func (cw *collectionWrapper) Upsert(key string, value any, opts *gocb.UpsertOptions) (*gocb.MutationResult, error) {
	return cw.Collection.Upsert(key, value, opts)
}

// Get retrieves a document by its key from the specified bucket.
func (c *Client) Get(ctx context.Context, key string, result any) error {
	if c.bucket == nil {
		return errBucketNotInitialized
	}

	res, err := c.bucket.DefaultCollection().Get(key, &gocb.GetOptions{Context: ctx})
	if err != nil {
		return fmt.Errorf("failed to get document with key %s: %w", key, err)
	}

	if err = res.Content(result); err != nil {
		return fmt.Errorf("failed to unmarshal document content for key %s: %w", key, err)
	}

	return nil
}

// Get performs a get operation on the collection.
func (cw *collectionWrapper) Get(key string, opts *gocb.GetOptions) (getResultProvider, error) {
	return cw.Collection.Get(key, opts)
}

// Remove deletes a document by its key from a bucket.
func (c *Client) Remove(ctx context.Context, key string) error {
	if c.bucket == nil {
		return errBucketNotInitialized
	}

	_, err := c.bucket.DefaultCollection().Remove(key, &gocb.RemoveOptions{Context: ctx})
	if err != nil {
		return fmt.Errorf("failed to remove document with key %s: %w", key, err)
	}

	return nil
}

// Remove performs a remove operation on the collection.
func (cw *collectionWrapper) Remove(key string, opts *gocb.RemoveOptions) (*gocb.MutationResult, error) {
	return cw.Collection.Remove(key, opts)
}

// Query executes a N1QL query against the Couchbase cluster.
func (c *Client) Query(ctx context.Context, statement string, params map[string]any, result any) error {
	if c.cluster == nil {
		return errClustertNotInitialized
	}

	queryFn := func() (resultProvider, error) {
		opts := &gocb.QueryOptions{Context: ctx}
		if params != nil {
			opts.NamedParameters = params
		}

		return c.cluster.Query(statement, opts)
	}

	return executeQuery(queryFn, "N1QL", result)
}

// AnalyticsQuery executes an Analytics query against the Couchbase Analytics service.
func (c *Client) AnalyticsQuery(ctx context.Context, statement string, params map[string]any, result any) error {
	if c.cluster == nil {
		return errClustertNotInitialized
	}

	queryFn := func() (resultProvider, error) {
		opts := &gocb.AnalyticsOptions{Context: ctx}
		if params != nil {
			opts.NamedParameters = params
		}

		return c.cluster.AnalyticsQuery(statement, opts)
	}

	return executeQuery(queryFn, "Analytics", result)
}

func executeQuery(queryFn func() (resultProvider, error), queryType string, result any) error {
	rows, err := queryFn()
	if err != nil {
		return fmt.Errorf("%s query failed: %w", queryType, err)
	}
	defer rows.Close()

	var tempResults []map[string]any

	for rows.Next() {
		var row map[string]any
		if err = rows.Row(&row); err != nil {
			return fmt.Errorf("failed to unmarshal %s query row into map: %w", queryType, err)
		}

		tempResults = append(tempResults, row)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("%s query iteration error: %w", queryType, err)
	}

	data, err := json.Marshal(tempResults)
	if err != nil {
		return fmt.Errorf("failed to marshal %s results: %w", queryType, err)
	}

	if err := json.Unmarshal(data, result); err != nil {
		return fmt.Errorf("failed to unmarshal %s results into target: %w", queryType, err)
	}

	return nil
}

// Close closes the connection to the Couchbase cluster.
func (c *Client) Close(opts *gocb.ClusterCloseOptions) error {
	if c.cluster != nil {
		return c.cluster.Close(opts)
	}

	return nil
}
