package arangodb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/arangodb/go-driver/v2/arangodb"
	arangoShared "github.com/arangodb/go-driver/v2/arangodb/shared"
	"github.com/arangodb/go-driver/v2/connection"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultTimeout           = 5 * time.Second
	arangoEdgeCollectionType = 3
)

// Client represents an ArangoDB client.
type Client struct {
	client   arangodb.Client
	logger   Logger
	metrics  Metrics
	tracer   trace.Tracer
	config   *Config
	endpoint string
	*DB
	*Document
	*Graph
}

type EdgeDefinition []arangodb.EdgeDefinition

type UserOptions struct {
	Password string `json:"passwd,omitempty"`
	Active   *bool  `json:"active,omitempty"`
	Extra    any    `json:"extra,omitempty"`
}

// Config holds the configuration for ArangoDB connection.
type Config struct {
	Host     string
	User     string
	Password string
	Port     int
}

var (
	errStatusDown             = errors.New("status down")
	errMissingField           = errors.New("missing required field in config")
	errInvalidResultType      = errors.New("result must be a pointer to a slice of maps")
	errInvalidUserOptionsType = errors.New("userOptions must be a *UserOptions type")
	ErrDatabaseExists         = errors.New("database already exists")
	ErrCollectionExists       = errors.New("collection already exists")
	ErrGraphExists            = errors.New("graph already exists")
)

// New creates a new ArangoDB client with the provided configuration.
func New(c Config) *Client {
	client := &Client{
		config: &c,
	}

	client.DB = &DB{client: client}
	client.Document = &Document{client: client}
	client.Graph = &Graph{client: client}

	return client
}

// UseLogger sets the logger for the ArangoDB client.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the ArangoDB client.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for the ArangoDB client.
func (c *Client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

// Connect establishes a connection to the ArangoDB server.
func (c *Client) Connect() {
	if err := c.validateConfig(); err != nil {
		c.logger.Errorf("config validation error: %v", err)
		return
	}

	c.endpoint = fmt.Sprintf("http://%s:%d", c.config.Host, c.config.Port)
	c.logger.Debugf("connecting to ArangoDB at %s", c.endpoint)

	// Use HTTP connection instead of HTTP2
	endpoint := connection.NewRoundRobinEndpoints([]string{c.endpoint})
	conn := connection.NewHttpConnection(connection.HttpConfiguration{Endpoint: endpoint})

	// Set authentication
	auth := connection.NewBasicAuth(c.config.User, c.config.Password)
	if err := conn.SetAuthentication(auth); err != nil {
		c.logger.Errorf("authentication setup failed: %v", err)
		return
	}

	// Create ArangoDB client
	client := arangodb.NewClient(conn)
	c.client = client

	// Test connection by fetching server version
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	_, err := c.client.Version(ctx)
	if err != nil {
		c.logger.Errorf("failed to verify connection: %v", err)
		return
	}

	// Initialize metrics
	arangoBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_arango_stats", "Response time of ArangoDB operations in milliseconds.", arangoBuckets...)

	c.logger.Logf("Connected to ArangoDB successfully at %s", c.endpoint)
}

func (c *Client) validateConfig() error {
	if c.config.Host == "" {
		return fmt.Errorf("%w: host is empty", errMissingField)
	}

	if c.config.Port == 0 {
		return fmt.Errorf("%w: port is empty", errMissingField)
	}

	if c.config.User == "" {
		return fmt.Errorf("%w: user is empty", errMissingField)
	}

	if c.config.Password == "" {
		return fmt.Errorf("%w: password is empty", errMissingField)
	}

	return nil
}

// Query executes an AQL (ArangoDB Query Language) query on the specified database and stores the result.
//
// Parameters:
//   - ctx: Context for request-scoped values, cancellation, and tracing.
//   - dbName: Name of the ArangoDB database where the query will be executed.
//   - query: The AQL query string to execute.
//   - bindVars: Map of bind parameters used in the AQL query.
//   - result: Pointer to a slice of maps where the query results will be unmarshaled.
//     Must be a valid pointer to avoid runtime errors.
//   - options: A flexible map[string]any to customize query behavior. Keys should be in camelCase
//     and correspond to fields in ArangoDB’s QueryOptions and QuerySubOptions structs.
//
// Available option keys include (but are not limited to):
//
// QueryOptions:
//   - count (bool): Include the total number of results in the result set.
//   - batchSize (int): Number of results to return per batch.
//   - cache (bool): Whether to cache the query results.
//   - memoryLimit (int64): Maximum memory in bytes for query execution.
//   - ttl (float64): Time-to-live for the cursor in seconds.
//   - options (map[string]any): Nested options from QuerySubOptions.
//
// QuerySubOptions:
//   - allowDirtyReads (bool)
//   - allowRetry (bool)
//   - failOnWarning (*bool)
//   - fullCount (bool): Return full count ignoring LIMIT clause.
//   - optimizer (map[string]any): Optimizer-specific directives.
//   - maxRuntime (float64): Maximum query runtime in seconds.
//   - stream (bool): Enable streaming cursor.
//   - profile (uint): Enable query profiling (0-2).
//   - skipInaccessibleCollections (*bool)
//   - intermediateCommitCount (*int)
//   - intermediateCommitSize (*int)
//   - maxDNFConditionMembers (*int)
//   - maxNodesPerCallstack (*int)
//   - maxNumberOfPlans (*int)
//   - maxTransactionSize (*int)
//   - maxWarningCount (*int)
//   - satelliteSyncWait (float64)
//   - spillOverThresholdMemoryUsage (*int)
//   - spillOverThresholdNumRows (*int)
//   - maxPlans (int)
//   - shardIds ([]string)
//   - forceOneShardAttributeValue (*string)
//
// Returns an error if:
//   - The database connection fails.
//   - The query execution fails.
//   - The result parameter is not a valid pointer to a slice of maps.
//
// Example:
//
//	var results []map[string]interface{}
//
//	query := `FOR u IN users FILTER u.age > @minAge RETURN u`
//
//	bindVars := map[string]interface{}{
//	    "minAge": 21,
//	}
//
//	options := map[string]any{
//	    "count":     true,
//	    "batchSize": 100,
//	    "options": map[string]any{
//	        "fullCount": true,
//	        "profile":   2,
//	    },
//	}
//
//	err := Query(ctx, "myDatabase", query, bindVars, &results, options)
//	if err != nil {
//	    log.Fatalf("Query failed: %v", err)
//	}
//
//	for _, doc := range results {
//	    fmt.Printf("User: %+v\n", doc)
//	}
func (c *Client) Query(ctx context.Context, dbName, query string, bindVars map[string]any, result any, options ...map[string]any) error {
	tracerCtx, span := c.addTrace(ctx, "query", map[string]string{"DB": dbName})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Operation: "query",
		Database: dbName, Query: query}, startTime, "query", span)

	db, err := c.client.GetDatabase(tracerCtx, dbName, nil)
	if err != nil {
		return err
	}

	var queryOptions arangodb.QueryOptions

	err = bindQueryOptions(&queryOptions, options)
	if err != nil {
		return err
	}

	queryOptions.BindVars = bindVars

	cursor, err := db.Query(tracerCtx, query, &queryOptions)
	if err != nil {
		return err
	}

	defer cursor.Close()

	resultSlice, ok := result.(*[]map[string]any)
	if !ok {
		return errInvalidResultType
	}

	for {
		var doc map[string]any

		_, err = cursor.ReadDocument(tracerCtx, &doc)
		if arangoShared.IsNoMoreDocuments(err) {
			break
		}

		if err != nil {
			return err
		}

		*resultSlice = append(*resultSlice, doc)
	}

	return nil
}

func bindQueryOptions(queryOptions *arangodb.QueryOptions, options []map[string]any) error {
	if len(options) > 0 {
		// Merge all options into a single map
		mergedOpts := make(map[string]any)

		for _, opts := range options {
			for k, v := range opts {
				mergedOpts[k] = v
			}
		}

		bytes, err := json.Marshal(mergedOpts)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(bytes, &queryOptions); err != nil {
			return err
		}
	}

	return nil
}

// addTrace adds tracing to context if tracer is configured.
func (c *Client) addTrace(ctx context.Context, operation string, attributes map[string]string) (context.Context, trace.Span) {
	if c.tracer != nil {
		contextWithTrace, span := c.tracer.Start(ctx, fmt.Sprintf("arangodb-%v", operation))

		// Add default attributes
		span.SetAttributes(attribute.String("arangodb.operation", operation))

		// Add custom attributes if provided
		for key, value := range attributes {
			span.SetAttributes(attribute.String(fmt.Sprintf("arangodb.%s", key), value))
		}

		return contextWithTrace, span
	}

	return ctx, nil
}

func (c *Client) sendOperationStats(ql *QueryLog, startTime time.Time, method string, span trace.Span) {
	duration := time.Since(startTime).Microseconds()
	ql.Duration = duration

	c.logger.Debug(ql)

	c.metrics.RecordHistogram(context.Background(), "app_arango_stats", float64(duration),
		"endpoint", c.endpoint,
		"type", ql.Query,
	)

	if span != nil {
		defer span.End()

		span.SetAttributes(attribute.Int64(fmt.Sprintf("arangodb.%v.duration", method), duration))
	}
}

// Health represents the health status of ArangoDB.
type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// HealthCheck performs a health check.
func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	h := Health{
		Details: map[string]any{
			"endpoint": c.endpoint,
		},
	}

	version, err := c.client.Version(ctx)
	if err != nil {
		h.Status = "DOWN"
		return &h, errStatusDown
	}

	h.Status = "UP"
	h.Details["version"] = version.Version
	h.Details["server"] = version.Server

	return &h, nil
}

func (uo *UserOptions) toArangoUserOptions() *arangodb.UserOptions {
	return &arangodb.UserOptions{
		Password: uo.Password,
		Active:   uo.Active,
		Extra:    uo.Extra,
	}
}
