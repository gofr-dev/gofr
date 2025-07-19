package couchbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/couchbase/gocb/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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

// Collection represents a handle to a Couchbase collection.
type Collection struct {
	collection collectionProvider
	client     *Client
}

// Scope represents a handle to a Couchbase scope.
type Scope struct {
	scope  scopeProvider
	client *Client
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
func (c *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		c.tracer = tracer
	}
}

func (c *Client) sendOperationStats(ql *QueryLog, startTime time.Time, method string, span trace.Span) {
	duration := time.Since(startTime).Microseconds()

	ql.Duration = duration

	c.logger.Debug(ql)

	c.metrics.RecordHistogram(context.Background(), "app_couchbase_stats", float64(duration), "hostname", c.config.Host,
		"bucket", c.config.Bucket, "type", method)

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64(fmt.Sprintf("couchbase.%v.duration", method), duration))
	}
}

func (c *Client) Connect() error {
	uri, err := c.generateCouchbaseURI()
	if err != nil {
		c.logger.Errorf("error generating Couchbase URI: %v", err)
		return err
	}

	c.logger.Debugf("connecting to Couchbase at %v to bucket %v", c.config.Host, c.config.Bucket)

	if err := c.establishConnection(uri); err != nil {
		c.logger.Errorf("error while connecting to Couchbase, err:%v", err)
		return err
	}

	if err := c.waitForClusterReady(); err != nil {
		c.logger.Errorf("could not connect to Couchbase at %v due to err: %v", c.config.Host, err)
		return err
	}

	c.bucket = c.cluster.Bucket(c.config.Bucket)

	if err := c.waitForBucketReady(); err != nil {
		c.logger.Errorf("could not connect to bucket %v at %v due to err: %v", c.config.Bucket, c.config.Host, err)
		return err
	}

	couchbaseBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_couchbase_stats", "Response time of Couchbase queries in milliseconds.", couchbaseBuckets...)

	c.logger.Logf("connected to Couchbase at %v to bucket %v", c.config.Host, c.config.Bucket)

	return nil
}

func (c *Client) generateCouchbaseURI() (string, error) {
	if c.config.URI != "" {
		return c.config.URI, nil
	}

	if c.config.Host == "" {
		return "", fmt.Errorf("%w: host is empty", errMissingField)
	}

	return fmt.Sprintf("couchbase://%s", c.config.Host), nil
}

func (c *Client) establishConnection(uri string) error {
	cluster, err := gocb.Connect(uri, gocb.ClusterOptions{
		Authenticator: gocb.PasswordAuthenticator{
			Username: c.config.User,
			Password: c.config.Password,
		},
	})
	if err != nil {
		return err
	}

	c.cluster = &clusterWrapper{cluster}

	return nil
}

func (c *Client) waitForClusterReady() error {
	timeout := c.getTimeout()
	return c.cluster.WaitUntilReady(timeout, nil)
}

func (c *Client) waitForBucketReady() error {
	timeout := c.getTimeout()
	return c.bucket.WaitUntilReady(timeout, nil)
}

func (c *Client) getTimeout() time.Duration {
	if c.config.ConnectionTimeout == 0 {
		return defaultTimeout
	}

	return c.config.ConnectionTimeout
}

// HealthCheck performs a health check on the Couchbase cluster.
func (c *Client) HealthCheck(_ context.Context) (any, error) {
	h := Health{
		Details: make(map[string]any),
	}

	h.Details["host"] = c.config.URI
	h.Details["bucket"] = c.config.Bucket

	_, err := c.cluster.Ping(nil)
	if err != nil {
		h.Status = "DOWN"
		return &h, errStatusDown
	}

	h.Status = "UP"

	return &h, nil
}

// DefaultCollection returns a handle for the default collection.
func (c *Client) DefaultCollection() *Collection {
	if c.bucket == nil {
		c.logger.Error("bucket not initialized")

		return &Collection{client: c}
	}

	return &Collection{
		collection: c.bucket.DefaultCollection(),
		client:     c,
	}
}

// Scope returns a handle for a specific scope.
func (c *Client) Scope(name string) *Scope {
	if c.bucket == nil {
		c.logger.Error("bucket not initialized")

		return &Scope{client: c}
	}

	return &Scope{
		scope:  c.bucket.Scope(name),
		client: c,
	}
}

// Collection returns a handle for a specific collection within the scope.
func (s *Scope) Collection(name string) *Collection {
	if s.scope == nil {
		return &Collection{client: s.client}
	}

	return &Collection{
		collection: s.scope.Collection(name),
		client:     s.client,
	}
}

// Upsert performs an upsert operation on the collection.
func (c *Collection) Upsert(ctx context.Context, key string, document, result any) error {
	if c.collection == nil {
		return errBucketNotInitialized
	}

	tracerCtx, span := c.client.addTrace(ctx, "Upsert", key)

	mr, err := c.collection.Upsert(key, document, &gocb.UpsertOptions{Context: tracerCtx})

	defer c.client.sendOperationStats(&QueryLog{Query: "Upsert", Key: key, Parameters: document}, time.Now(), "Upsert", span)

	if err != nil {
		return fmt.Errorf("failed to Upsert document with key %s: %w", key, err)
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

// Insert inserts a new document in the collection.
func (c *Collection) Insert(ctx context.Context, key string, document, result any) error {
	if c.collection == nil {
		return errBucketNotInitialized
	}

	tracerCtx, span := c.client.addTrace(ctx, "Insert", key)

	mr, err := c.collection.Insert(key, document, &gocb.InsertOptions{Context: tracerCtx})

	defer c.client.sendOperationStats(&QueryLog{Query: "Insert", Key: key, Parameters: document}, time.Now(), "Insert", span)

	if err != nil {
		return fmt.Errorf("failed to Insert document with key %s: %w", key, err)
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

// Get performs a get operation on the collection.
func (c *Collection) Get(ctx context.Context, key string, result any) error {
	if c.collection == nil {
		return errBucketNotInitialized
	}

	tracerCtx, span := c.client.addTrace(ctx, "Get", key)

	res, err := c.collection.Get(key, &gocb.GetOptions{Context: tracerCtx})

	defer c.client.sendOperationStats(&QueryLog{Query: "Get", Key: key}, time.Now(), "Get", span)

	if err != nil {
		c.client.logger.Errorf("failed to get document with key %s: %w", key, err)

		return fmt.Errorf("failed to get document with key %s: %w", key, err)
	}

	if err = res.Content(result); err != nil {
		return fmt.Errorf("failed to unmarshal document content for key %s: %w", key, err)
	}

	return nil
}

// Remove performs a remove operation on the collection.
func (c *Collection) Remove(ctx context.Context, key string) error {
	if c.collection == nil {
		return errBucketNotInitialized
	}

	tracerCtx, span := c.client.addTrace(ctx, "Remove", key)

	_, err := c.collection.Remove(key, &gocb.RemoveOptions{Context: tracerCtx})

	defer c.client.sendOperationStats(&QueryLog{Query: "Remove", Key: key}, time.Now(), "Remove", span)

	if err != nil {
		return fmt.Errorf("failed to remove document with key %s: %w", key, err)
	}

	return nil
}

func (c *Client) executeTracedQuery(
	ctx context.Context,
	statement string,
	params map[string]any,
	result any,
	operation string,
	queryType string,
	queryFn func(tracerCtx context.Context) (resultProvider, error),
) error {
	if c.cluster == nil {
		return errClustertNotInitialized
	}

	tracerCtx, span := c.addTrace(ctx, operation, statement)

	startTime := time.Now()

	err := executeQuery(func() (resultProvider, error) {
		return queryFn(tracerCtx)
	}, queryType, result)

	defer c.sendOperationStats(&QueryLog{Query: operation, Statement: statement, Parameters: params}, startTime, operation, span)

	if err != nil {
		c.logger.Errorf("%s query failed: %v", queryType, err)
	}

	return err
}

// Query executes a N1QL query against the Couchbase cluster.
func (c *Client) Query(ctx context.Context, statement string, params map[string]any, result any) error {
	queryFn := func(tracerCtx context.Context) (resultProvider, error) {
		opts := &gocb.QueryOptions{Context: tracerCtx}
		if params != nil {
			opts.NamedParameters = params
		}

		return c.cluster.Query(statement, opts)
	}

	return c.executeTracedQuery(ctx, statement, params, result, "N1QLQuery", "N1QL", queryFn)
}

// AnalyticsQuery executes an Analytics query against the Couchbase Analytics service.
func (c *Client) AnalyticsQuery(ctx context.Context, statement string, params map[string]any, result any) error {
	queryFn := func(tracerCtx context.Context) (resultProvider, error) {
		opts := &gocb.AnalyticsOptions{Context: tracerCtx}
		if params != nil {
			opts.NamedParameters = params
		}

		return c.cluster.AnalyticsQuery(statement, opts)
	}

	return c.executeTracedQuery(ctx, statement, params, result, "AnalyticsQuery", "Analytics", queryFn)
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
		switch queryType {
		case "N1QL":
			return fmt.Errorf("%w: %w", errFailedToUnmarshalN1QL, err)
		case "Analytics":
			return fmt.Errorf("%w: %w", errFailedToUnmarshalAnalytics, err)
		}
	}

	return nil
}

// RunTransaction executes a transaction.
func (c *Client) RunTransaction(ctx context.Context, logic func(t *gocb.TransactionAttemptContext) error) (*gocb.TransactionResult, error) {
	if c.cluster == nil {
		return nil, errClustertNotInitialized
	}

	_, span := c.addTrace(ctx, "RunTransaction", "transaction")

	startTime := time.Now()

	// gocb transactions are not directly context-aware in the Run method signature in the same way as other operations.
	// The context is passed down to operations within the transaction lambda.
	result, err := c.cluster.Transactions().Run(logic, nil)

	defer c.sendOperationStats(&QueryLog{Query: "RunTransaction"}, startTime, "RunTransaction", span)

	if err != nil {
		c.logger.Errorf("Transaction failed: %v", err)
	}

	return result, err
}

// Get performs a get operation on the default collection.
func (c *Client) Get(ctx context.Context, key string, result any) error {
	return c.DefaultCollection().Get(ctx, key, result)
}

// Insert inserts a new document in the default collection.
func (c *Client) Insert(ctx context.Context, key string, document, result any) error {
	return c.DefaultCollection().Insert(ctx, key, document, result)
}

// Upsert performs an upsert operation on the default collection.
func (c *Client) Upsert(ctx context.Context, key string, document, result any) error {
	return c.DefaultCollection().Upsert(ctx, key, document, result)
}

// Remove performs a remove operation on the default collection.
func (c *Client) Remove(ctx context.Context, key string) error {
	return c.DefaultCollection().Remove(ctx, key)
}

// Close closes the connection to the Couchbase cluster.
func (c *Client) Close(opts any) error {
	if c.cluster != nil {
		return c.cluster.Close(opts.(*gocb.ClusterCloseOptions))
	}

	return nil
}

func (c *Client) addTrace(ctx context.Context, method, statement string) (context.Context, trace.Span) {
	if c.tracer != nil {
		contextWithTrace, span := c.tracer.Start(ctx, fmt.Sprintf("couchbase-%v", method))

		span.SetAttributes(
			attribute.String("couchbase.statement", statement),
		)

		return contextWithTrace, span
	}

	return ctx, nil
}
