package dgraph

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"
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
	conn    *grpc.ClientConn
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
		tracer: otel.GetTracerProvider().Tracer("gofr-dgraph"),
	}
}

// Connect connects to the Dgraph database using the provided configuration.
func (d *Client) Connect() {
	address := fmt.Sprintf("%s:%s", d.config.Host, d.config.Port)
	d.logger.Logf("connecting to dgraph at %v", address)

	conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		d.logger.Errorf("error connecting to Dgraph, err: %v", err)
		return
	}

	d.logger.Logf("connected to dgraph client at %v:%v", d.config.Host, d.config.Port)

	// Register metrics
	// Register all metrics
	d.metrics.NewHistogram("dgraph_query_duration", "Response time of Dgraph queries in milliseconds.",
		0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10)
	d.metrics.NewHistogram("dgraph_query_with_vars_duration", "Response time of Dgraph queries with variables in milliseconds.",
		0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10)
	d.metrics.NewHistogram("dgraph_mutate_duration", "Response time of Dgraph mutations in milliseconds.",
		0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10)
	d.metrics.NewHistogram("dgraph_alter_duration", "Response time of Dgraph alter operations in milliseconds.",
		0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10)

	dg := dgo.NewDgraphClient(api.NewDgraphClient(conn))
	d.client = NewDgraphClient(dg)

	// Check connection by performing a basic health check
	if _, err := d.HealthCheck(context.Background()); err != nil {
		d.logger.Errorf("dgraph health check failed: %v", err)
		return
	}
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

// Query executes a read-only query in the Dgraph database and returns the result.
func (d *Client) Query(ctx context.Context, query string) (any, error) {
	start := time.Now()

	ctx, span := d.tracer.Start(ctx, "dgraph-query")
	defer span.End()

	// Execute query
	resp, err := d.client.NewTxn().Query(ctx, query)
	duration := time.Since(start).Microseconds()

	// Create and log the query details
	ql := &QueryLog{
		Type:     "query",
		URL:      query,
		Duration: duration,
	}

	span.SetAttributes(
		attribute.String("dgraph.query.query", query),
		attribute.Int64("dgraph.query.duration", duration),
	)

	if err != nil {
		d.logger.Error("dgraph query failed: ", err)
		ql.PrettyPrint(d.logger)
		return nil, err
	}

	d.sendOperationStats(ctx, ql, "dgraph_query_duration")

	return resp, nil
}

// QueryWithVars executes a read-only query with variables in the Dgraph database.
// QueryWithVars executes a read-only query with variables in the Dgraph database.
func (d *Client) QueryWithVars(ctx context.Context, query string, vars map[string]string) (any, error) {
	start := time.Now()

	ctx, span := d.tracer.Start(ctx, "dgraph-query-with-vars")
	defer span.End()

	// Execute the query with variables
	resp, err := d.client.NewTxn().QueryWithVars(ctx, query, vars)
	duration := time.Since(start).Microseconds()

	// Create and log the query details
	ql := &QueryLog{
		Type:     "queryWithVars",
		URL:      fmt.Sprintf("Query: %s, Vars: %v", query, vars),
		Duration: duration,
	}

	span.SetAttributes(
		attribute.String("dgraph.query.query", query),
		attribute.String("dgraph.query.vars", fmt.Sprintf("%v", vars)),
		attribute.Int64("dgraph.query.duration", duration),
	)

	if err != nil {
		d.logger.Error("dgraph queryWithVars failed: ", err)
		ql.PrettyPrint(d.logger)
		return nil, err
	}

	d.sendOperationStats(ctx, ql, "dgraph_query_with_vars_duration")

	return resp, nil
}

// Mutate executes a write operation (mutation) in the Dgraph database and returns the result.
func (d *Client) Mutate(ctx context.Context, mu any) (any, error) {
	start := time.Now()

	ctx, span := d.tracer.Start(ctx, "dgraph-mutate")
	defer span.End()

	// Cast to proper mutation type
	mutation, ok := mu.(*api.Mutation)
	if !ok {
		return nil, errInvalidMutation
	}

	// Execute mutation
	resp, err := d.client.NewTxn().Mutate(ctx, mutation)
	duration := time.Since(start).Microseconds()

	// Create and log the mutation details
	ql := &QueryLog{
		Type:     "mutation",
		URL:      mutationToString(mutation),
		Duration: duration,
	}

	span.SetAttributes(
		attribute.String("dgraph.mutation.query", mutationToString(mutation)),
		attribute.Int64("dgraph.mutation.duration", duration),
	)

	if err != nil {
		d.logger.Error("dgraph mutation failed: ", err)
		ql.PrettyPrint(d.logger)
		return nil, err
	}

	d.sendOperationStats(ctx, ql, "dgraph_mutate_duration")

	return resp, nil
}

// Alter applies schema or other changes to the Dgraph database.
func (d *Client) Alter(ctx context.Context, op any) error {
	start := time.Now()

	ctx, span := d.tracer.Start(ctx, "dgraph-alter")
	defer span.End()

	// Cast to proper operation type
	operation, ok := op.(*api.Operation)
	if !ok {
		d.logger.Error("invalid operation type provided to alter")
		return errInvalidOperation
	}

	// Apply the schema changes
	err := d.client.Alter(ctx, operation)
	duration := time.Since(start).Microseconds()

	// Create and log the operation details
	ql := &QueryLog{
		Type:     "Alter",
		URL:      operation.String(),
		Duration: duration,
	}

	span.SetAttributes(
		attribute.String("dgraph.alter.operation", operation.String()),
		attribute.Int64("dgraph.alter.duration", duration),
	)

	if err != nil {
		d.logger.Error("dgraph alter failed: ", err)
		ql.PrettyPrint(d.logger)
		return err
	}

	d.sendOperationStats(ctx, ql, "dgraph_alter_duration")

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

func (d *Client) sendOperationStats(ctx context.Context, query *QueryLog, metricName string) {
	query.PrettyPrint(d.logger)
	d.metrics.RecordHistogram(ctx, metricName, float64(query.Duration))
}

func mutationToString(mutation *api.Mutation) string {
	var compacted bytes.Buffer
	if err := json.Compact(&compacted, mutation.SetJson); err != nil {
		return ""
	}

	return compacted.String()

}
