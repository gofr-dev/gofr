package dgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds the configuration for connecting to Dgraph.
type Config struct {
	Host string
	Port string
}

// Client represents the Dgraph client with logging and metrics.
type Client struct {
	client  DgraphClient
	logger  Logger
	metrics Metrics
	config  Config
	tracer  trace.Tracer
}

type (
	Mutation  = api.Mutation
	Operation = api.Operation
)

var (
	errInvalidMutation   = errors.New("invalid mutation type")
	errInvalidOperation  = errors.New("invalid operation type")
	errHealthCheckFailed = errors.New("dgraph health check failed")
)

// New creates a new Dgraph client with the given configuration.
func New(config Config) *Client {
	return &Client{
		config: config,
	}
}

// Connect connects to the Dgraph database using the provided configuration.
func (d *Client) Connect() {
	address := fmt.Sprintf("%s:%s", d.config.Host, d.config.Port)
	d.logger.Debugf("connecting to Dgraph at %v", address)

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		d.logger.Errorf("error while connecting to Dgraph, err: %v", err)
		return
	}

	var responseTimeBuckets = []float64{0.05, 0.1, 0.2, 0.5, 1.0, 2.0, 5.0, 10.0}

	// Register metrics
	d.metrics.NewHistogram("dgraph_query_duration", "Response time of Dgraph queries in microseconds.", responseTimeBuckets...)
	d.metrics.NewHistogram("dgraph_query_with_vars_duration", "Response time of Dgraph "+
		"queries with variables in microseconds.", responseTimeBuckets...)
	d.metrics.NewHistogram("dgraph_mutate_duration", "Response time of Dgraph mutations in microseconds.", responseTimeBuckets...)
	d.metrics.NewHistogram("dgraph_alter_duration", "Response time of Dgraph alter operations in microseconds.", responseTimeBuckets...)

	dg := dgo.NewDgraphClient(api.NewDgraphClient(conn))
	d.client = NewDgraphClient(dg)

	// Check connection by performing a basic health check
	if _, err := d.HealthCheck(context.Background()); err != nil {
		d.logger.Errorf("error while connecting to Dgraph: %v", err)
		return
	}

	d.logger.Logf("connected to Dgraph server at %v:%v", d.config.Host, d.config.Port)
}

// UseLogger sets the logger for the Dgraph client which asserts the Logger interface.
func (d *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		d.logger = l
	}
}

// UseMetrics sets the metrics for the Dgraph client which asserts the Metrics interface.
func (d *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		d.metrics = m
	}
}

// UseTracer sets the tracer for DGraph client.
func (d *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		d.tracer = tracer
	}
}

// Query executes a read-only query in the Dgraph database and returns the result.
func (d *Client) Query(ctx context.Context, query string) (any, error) {
	start := time.Now()

	tracedCtx, span := d.addTrace(ctx, "query")

	// Execute query
	resp, err := d.client.NewTxn().Query(tracedCtx, query)
	duration := time.Since(start).Microseconds()

	// Create and log the query details
	ql := &QueryLog{
		Type:     "query",
		URL:      query,
		Duration: duration,
	}

	if err != nil {
		d.logger.Error("dgraph query failed: ", err)
		ql.PrettyPrint(d.logger)

		return nil, err
	}

	d.sendOperationStats(tracedCtx, start, query, "query", span, ql, "dgraph_query_duration")

	return resp, nil
}

// QueryWithVars executes a read-only query with variables in the Dgraph database.
// QueryWithVars executes a read-only query with variables in the Dgraph database.
func (d *Client) QueryWithVars(ctx context.Context, query string, vars map[string]string) (any, error) {
	start := time.Now()

	tracedCtx, span := d.addTrace(ctx, "query-with-vars")

	// Execute the query with variables
	resp, err := d.client.NewTxn().QueryWithVars(tracedCtx, query, vars)
	duration := time.Since(start).Microseconds()

	// Create and log the query details
	ql := &QueryLog{
		Type:     "queryWithVars",
		URL:      fmt.Sprintf("Query: %s, Vars: %v", query, vars),
		Duration: duration,
	}

	if span != nil {
		span.SetAttributes(attribute.String("dgraph.query.vars", fmt.Sprintf("%v", vars)))
	}

	if err != nil {
		d.logger.Error("dgraph queryWithVars failed: ", err)
		ql.PrettyPrint(d.logger)

		return nil, err
	}

	d.sendOperationStats(tracedCtx, start, query, "query-with-vars", span, ql, "dgraph_query_with_vars_duration")

	return resp, nil
}

// Mutate executes a write operation (mutation) in the Dgraph database and returns the result.
func (d *Client) Mutate(ctx context.Context, mu any) (any, error) {
	start := time.Now()

	tracedCtx, span := d.addTrace(ctx, "mutate")

	// Cast to proper mutation type
	mutation, ok := mu.(*api.Mutation)
	if !ok {
		return nil, errInvalidMutation
	}

	// Execute mutation
	resp, err := d.client.NewTxn().Mutate(tracedCtx, mutation)
	duration := time.Since(start).Microseconds()

	// Create and log the mutation details
	ql := &QueryLog{
		Type:     "mutation",
		URL:      mutationToString(mutation),
		Duration: duration,
	}

	if err != nil {
		d.logger.Error("dgraph mutation failed: ", err)
		ql.PrettyPrint(d.logger)

		return nil, err
	}

	d.sendOperationStats(tracedCtx, start, mutationToString(mutation), "mutate", span, ql, "dgraph_mutate_duration")

	return resp, nil
}

// Alter applies schema or other changes to the Dgraph database.
func (d *Client) Alter(ctx context.Context, op any) error {
	start := time.Now()

	tracedCtx, span := d.addTrace(ctx, "alter")

	// Cast to proper operation type
	operation, ok := op.(*api.Operation)
	if !ok {
		d.logger.Error("invalid operation type provided to alter")
		return errInvalidOperation
	}

	// Apply the schema changes
	err := d.client.Alter(tracedCtx, operation)
	duration := time.Since(start).Microseconds()

	// Create and log the operation details
	ql := &QueryLog{
		Type:     "Alter",
		URL:      operation.String(),
		Duration: duration,
	}

	if span != nil {
		span.SetAttributes(attribute.String("dgraph.alter.operation", operation.String()))
	}

	if err != nil {
		d.logger.Error("dgraph alter failed: ", err)
		ql.PrettyPrint(d.logger)

		return err
	}

	d.sendOperationStats(tracedCtx, start, operation.String(), "alter", span, ql, "dgraph_alter_duration")

	return nil
}

// NewTxn creates a new transaction (read-write) for interacting with the Dgraph database.
func (d *Client) NewTxn() any {
	return d.client.NewTxn()
}

// NewReadOnlyTxn creates a new read-only transaction for querying the Dgraph database.
func (d *Client) NewReadOnlyTxn() any {
	return d.client.NewReadOnlyTxn()
}

// HealthCheck performs a basic health check by pinging the Dgraph server.
func (d *Client) HealthCheck(ctx context.Context) (any, error) {
	healthResponse, err := d.client.NewTxn().Query(ctx, `{
        health(func: has(dgraph.type)) {
            status
        }
    }`)

	if err != nil || len(healthResponse.Json) == 0 {
		d.logger.Error("dgraph health check failed: ", err)
		return "DOWN", errHealthCheckFailed
	}

	return "UP", nil
}

func (d *Client) addTrace(ctx context.Context, method string) (context.Context, trace.Span) {
	if d.tracer == nil {
		return ctx, nil
	}

	tracedCtx, span := d.tracer.Start(ctx, fmt.Sprintf("dgraph-%v", method))

	return tracedCtx, span
}

func (d *Client) sendOperationStats(ctx context.Context, start time.Time, query, method string,
	span trace.Span, queryLog *QueryLog, metricName string) {
	duration := time.Since(start).Microseconds()

	if span != nil {
		defer span.End()

		span.SetAttributes(attribute.String(fmt.Sprintf("dgraph.%v.query", method), query))
		span.SetAttributes(attribute.Int64(fmt.Sprintf("dgraph.%v.duration", method), duration))
	}

	queryLog.PrettyPrint(d.logger)
	d.metrics.RecordHistogram(ctx, metricName, float64(queryLog.Duration))
}

func mutationToString(mutation *api.Mutation) string {
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, mutation.SetJson); err != nil {
		return ""
	}

	return compacted.String()
}
