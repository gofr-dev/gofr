package couchbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/couchbase/gocb/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
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

// sendOperationStats sends statistics about a Couchbase operation.
func (c *Client) sendOperationStats(ql *QueryLog, startTime time.Time, method string) {
	duration := time.Since(startTime).Microseconds()

	ql.Duration = duration

	c.logger.Debug(ql)

	c.metrics.RecordHistogram(context.Background(), "app_couchbase_stats", float64(duration), "hostname", c.config.Host,
		"bucket", c.config.Bucket, "type", method)
}

// Connect establishes a connection to the Couchbase cluster.
func (c *Client) Connect() {
	uri, err := c.generateCouchbaseURI()
	if err != nil {
		c.logger.Errorf("error generating Couchbase URI: %v", err)
		return
	}

	c.logger.Debugf("connecting to Couchbase at %v to bucket %v", c.config.Host, c.config.Bucket)

	if err := c.establishConnection(uri); err != nil {
		c.logger.Errorf("error while connecting to Couchbase, err:%v", err)
		return
	}

	if err := c.waitForClusterReady(); err != nil {
		c.logger.Errorf("could not connect to Couchbase at %v due to err: %v", c.config.Host, err)
		return
	}

	c.bucket = c.cluster.Bucket(c.config.Bucket)

	if err := c.waitForBucketReady(); err != nil {
		c.logger.Errorf("could not connect to bucket %v at %v due to err: %v", c.config.Bucket, c.config.Host, err)
		return
	}

	couchbaseBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_couchbase_stats", "Response time of Couchbase queries in milliseconds.", couchbaseBuckets...)

	c.logger.Logf("connected to Couchbase at %v to bucket %v", c.config.Host, c.config.Bucket)
}

// generateCouchbaseURI generates the Couchbase connection URI.
func (c *Client) generateCouchbaseURI() (string, error) {
	if c.config.URI != "" {
		return c.config.URI, nil
	}

	if c.config.Host == "" {
		return "", fmt.Errorf("%w: host is empty", errMissingField)
	}

	return fmt.Sprintf("couchbase://%s", c.config.Host), nil
}

// establishConnection establishes a connection to the Couchbase cluster.
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

// waitForClusterReady waits for the Couchbase cluster to be ready.
func (c *Client) waitForClusterReady() error {
	timeout := c.getTimeout()
	return c.cluster.WaitUntilReady(timeout, nil)
}

// waitForBucketReady waits for the Couchbase bucket to be ready.
func (c *Client) waitForBucketReady() error {
	timeout := c.getTimeout()
	return c.bucket.WaitUntilReady(timeout, nil)
}

// getTimeout returns the connection timeout.
func (c *Client) getTimeout() time.Duration {
	if c.config.ConnectionTimeout == 0 {
		return defaultTimeout
	}

	return c.config.ConnectionTimeout
}

// HealthCheck performs a health check on the Couchbase connection.
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

// defaultCollection returns a handle for the default collection.
func (c *Client) defaultCollection() *Collection {
	if c.bucket == nil {
		c.logger.Error("bucket not initialized")

		return &Collection{client: c}
	}

	return &Collection{
		collection: c.bucket.DefaultCollection(),
		client:     c,
	}
}

// scope returns a handle for a specific scope.
func (c *Client) scope(name string) *Scope {
	if c.bucket == nil {
		c.logger.Error("bucket not initialized")

		return &Scope{client: c}
	}

	return &Scope{
		scope:  c.bucket.Scope(name),
		client: c,
	}
}

// mutationOperation performs a mutation operation on the collection.
func (c *Collection) mutationOperation(ctx context.Context, opName, key string, document, result any,
	op func(tracerCtx context.Context) (*gocb.MutationResult, error),
) error {
	if c.collection == nil {
		return errBucketNotInitialized
	}

	tracerCtx, span := c.client.addTrace(ctx, opName, key)
	startTime := time.Now()

	mr, err := op(tracerCtx)

	// Finish span with error status
	c.client.finishSpan(span, err)

	defer c.client.sendOperationStats(&QueryLog{Query: opName, Key: key, Parameters: document}, startTime, opName)

	if err != nil {
		return fmt.Errorf("failed to %s document with key %s: %w", opName, key, err)
	}

	switch r := result.(type) {
	case *gocb.MutationResult:
		*r = *mr
	case **gocb.MutationResult:
		*r = mr
	case nil:
	default:
		return errWrongResultType
	}

	return nil
}

// Upsert performs an upsert operation on the collection.
func (c *Collection) Upsert(ctx context.Context, key string, document, result any) error {
	return c.mutationOperation(ctx, "Upsert", key, document, result, func(tracerCtx context.Context) (*gocb.MutationResult, error) {
		return c.collection.Upsert(key, document, &gocb.UpsertOptions{Context: tracerCtx})
	})
}

// Insert inserts a new document in the collection.
func (c *Collection) Insert(ctx context.Context, key string, document, result any) error {
	return c.mutationOperation(ctx, "Insert", key, document, result, func(tracerCtx context.Context) (*gocb.MutationResult, error) {
		return c.collection.Insert(key, document, &gocb.InsertOptions{Context: tracerCtx})
	})
}

// Remove removes a document from the collection.
func (c *Collection) Remove(ctx context.Context, key string) error {
	if c.collection == nil {
		return errBucketNotInitialized
	}

	tracerCtx, span := c.client.addTrace(ctx, "Remove", key)
	startTime := time.Now()

	_, err := c.collection.Remove(key, &gocb.RemoveOptions{Context: tracerCtx})

	// Finish span with error status
	c.client.finishSpan(span, err)

	defer c.client.sendOperationStats(&QueryLog{Query: "Remove", Key: key}, startTime, "Remove")

	if err != nil {
		return fmt.Errorf("failed to remove document with key %s: %w", key, err)
	}

	return nil
}

// executeTracedQuery executes a traced query.
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

	// Add query parameters as span attributes if they exist
	if len(params) > 0 && c.tracer != nil {
		// Only add a count of parameters to avoid sensitive data leakage
		span.SetAttributes(attribute.Int("db.couchbase.parameter_count", len(params)))
	}

	startTime := time.Now()

	err := executeQuery(func() (resultProvider, error) {
		return queryFn(tracerCtx)
	}, queryType, result)

	// Finish span with error status
	c.finishSpan(span, err)

	defer c.sendOperationStats(&QueryLog{Query: operation, Statement: statement, Parameters: params}, startTime, operation)

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

// executeQuery executes a query and processes the results.
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
func (c *Client) RunTransaction(ctx context.Context, logic func(any) error) (any, error) {
	if c.cluster == nil {
		return nil, errClustertNotInitialized
	}

	_, span := c.addTrace(ctx, "RunTransaction", "transaction")
	defer span.End()

	startTime := time.Now()

	// Wrap the generic logic function to match the expected signature
	wrappedLogic := func(t *gocb.TransactionAttemptContext) error {
		return logic(t)
	}

	// gocb transactions are not directly context-aware in the Run method signature in the same way as other operations.
	// The context is passed down to operations within the transaction lambda.
	result, err := c.cluster.Transactions().Run(wrappedLogic, nil)

	defer c.sendOperationStats(&QueryLog{Query: "RunTransaction"}, startTime, "RunTransaction")

	if err != nil {
		c.logger.Errorf("Transaction failed: %v", err)
	}

	return result, err
}

// Get performs a get operation on the default collection.
func (c *Client) Get(ctx context.Context, key string, result any) error {
	return c.defaultCollection().Get(ctx, key, result)
}

// Insert inserts a new document in the default collection.
func (c *Client) Insert(ctx context.Context, key string, document, result any) error {
	return c.defaultCollection().Insert(ctx, key, document, result)
}

// Upsert performs an upsert operation on the default collection.
func (c *Client) Upsert(ctx context.Context, key string, document, result any) error {
	return c.defaultCollection().Upsert(ctx, key, document, result)
}

// Remove performs a remove operation on the default collection.
func (c *Client) Remove(ctx context.Context, key string) error {
	return c.defaultCollection().Remove(ctx, key)
}

// Close closes the connection to the Couchbase cluster.
func (c *Client) Close(opts any) error {
	if c.cluster != nil {
		return c.cluster.Close(opts.(*gocb.ClusterCloseOptions))
	}

	return nil
}

// addTrace adds a trace to the context.
func (c *Client) addTrace(ctx context.Context, method, statement string) (context.Context, trace.Span) {
	if c.tracer == nil {
		// Return a no-op span when tracer is not available
		return ctx, trace.SpanFromContext(ctx)
	}

	// Set the span attributes following OpenTelemetry semantic conventions
	attrs := []attribute.KeyValue{
		attribute.String("db.system", "couchbase"),
		attribute.String("db.operation", method),
		attribute.String("db.name", c.config.Bucket),
		attribute.String("server.address", c.config.Host),
	}

	// Add statement/key information based on the operation
	if statement != "" {
		if method == "Get" || method == "Insert" || method == "Upsert" || method == "Remove" {
			attrs = append(attrs, attribute.String("db.couchbase.document_key", statement))
		} else {
			attrs = append(attrs, attribute.String("db.statement", statement))
		}
	}

	// Create a new span with proper naming
	spanName := fmt.Sprintf("couchbase.%s", strings.ToLower(method))
	ctx, span := c.tracer.Start(ctx, spanName, trace.WithAttributes(attrs...))

	return ctx, span
}

// finishSpan finishes a trace span.
func (*Client) finishSpan(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}

	span.End()
}

// Get performs a get operation on the collection.
func (c *Collection) Get(ctx context.Context, key string, result any) error {
	if c.collection == nil {
		return errBucketNotInitialized
	}

	tracerCtx, span := c.client.addTrace(ctx, "Get", key)
	startTime := time.Now()

	res, err := c.collection.Get(key, &gocb.GetOptions{Context: tracerCtx})

	// Finish span with error status
	c.client.finishSpan(span, err)

	defer c.client.sendOperationStats(&QueryLog{Query: "Get", Key: key}, startTime, "Get")

	if err != nil {
		c.client.logger.Errorf("failed to get document with key %s: %v", key, err)
		return fmt.Errorf("failed to get document with key %s: %w", key, err)
	}

	if err = res.Content(result); err != nil {
		return fmt.Errorf("failed to unmarshal document content for key %s: %w", key, err)
	}

	return nil
}
