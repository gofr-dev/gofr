package dgraph

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/dgo/v210"
	"github.com/dgraph-io/dgo/v210/protos/api"
	"google.golang.org/grpc"
)

// Config holds the configuration for connecting to Dgraph.
type Config struct {
	Host string
	Port string
}

// Client represents the Dgraph client with logging and metrics.
type Client struct {
	client  *dgo.Dgraph
	conn    *grpc.ClientConn
	logger  Logger
	metrics Metrics
	config  Config
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
func (c *Client) Connect() error {
	address := fmt.Sprintf("%s:%s", c.config.Host, c.config.Port)
	c.logger.Logf("Connecting to Dgraph at %v", address)

	conn, err := grpc.Dial(address, grpc.WithInsecure()) // Use secure connection in production
	if err != nil {
		c.logger.Errorf("Error connecting to Dgraph, err: %v", err)
		return err
	}

	c.client = dgo.NewDgraphClient(api.NewDgraphClient(conn))
	c.conn = conn

	// Check connection by performing a basic health check
	if _, err := c.HealthCheck(context.Background()); err != nil {
		c.logger.Errorf("Dgraph health check failed: %v", err)
		return err
	}

	return nil
}

// UseLogger sets the logger for the Dgraph client which asserts the Logger interface.
func (c *Client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the Dgraph client which asserts the Metrics interface.
func (c *Client) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// Query executes a read-only query in the Dgraph database and returns the result.
func (d *Client) Query(ctx context.Context, query string) (interface{}, error) {
	start := time.Now()

	// Execute query
	resp, err := d.client.NewTxn().Query(ctx, query)
	duration := time.Since(start).Microseconds()

	// Create and log the query details
	ql := &QueryLog{
		Type:     "Query",
		URL:      query,
		Duration: duration,
	}

	d.logger.Debug("Executing Dgraph Query")
	ql.PrettyPrint(d.logger)

	if err != nil {
		d.logger.Error("Dgraph Query failed: ", err)
		return nil, err
	}

	d.logger.Debugf("Dgraph Query succeeded in %dµs", duration)

	d.metrics.RecordHistogram(ctx, "dgraph_query_duration", float64(duration))

	return resp, nil
}

// QueryWithVars executes a read-only query with variables in the Dgraph database.
func (d *Client) QueryWithVars(ctx context.Context, query string, vars map[string]string) (interface{}, error) {
	start := time.Now()

	// Execute the query with variables
	resp, err := d.client.NewTxn().QueryWithVars(ctx, query, vars)
	duration := time.Since(start).Microseconds()

	// Create and log the query details
	ql := &QueryLog{
		Type:     "QueryWithVars",
		URL:      fmt.Sprintf("Query: %s, Vars: %v", query, vars),
		Duration: duration,
	}

	if err != nil {
		d.logger.Error("Dgraph QueryWithVars failed: ", err)
		ql.PrettyPrint(d.logger)
		return nil, err
	}

	d.logger.Debugf("Dgraph QueryWithVars succeeded in %dµs", duration)
	ql.PrettyPrint(d.logger)

	d.metrics.RecordHistogram(ctx, "dgraph_query_with_vars_duration", float64(duration))

	return resp, nil
}

// Mutate executes a write operation (mutation) in the Dgraph database and returns the result.
func (d *Client) Mutate(ctx context.Context, mu interface{}) (interface{}, error) {
	start := time.Now()

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
		Type:     "Mutation",
		URL:      mutation.String(),
		Duration: duration,
	}
	d.logger.Debug("Executing Dgraph Mutation")
	ql.PrettyPrint(d.logger)

	if err != nil {
		d.logger.Error("Dgraph Mutation failed: ", err)
		return nil, err
	}

	d.logger.Debugf("Dgraph Mutation succeeded in %dµs", duration)

	d.metrics.RecordHistogram(ctx, "dgraph_mutate_duration", float64(duration))

	return resp, nil
}

// Alter applies schema or other changes to the Dgraph database.
func (d *Client) Alter(ctx context.Context, op interface{}) error {
	start := time.Now()

	// Cast to proper operation type
	operation, ok := op.(*api.Operation)
	if !ok {
		d.logger.Error("Invalid operation type provided to Alter")
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

	if err != nil {
		d.logger.Error("Dgraph Alter failed: ", err)
		ql.PrettyPrint(d.logger)
		return err
	}

	d.logger.Debugf("Dgraph Alter succeeded in %dµs", duration)

	ql.PrettyPrint(d.logger)

	d.metrics.RecordHistogram(ctx, "dgraph_alter_duration", float64(duration))

	return nil
}

// NewTxn creates a new transaction (read-write) for interacting with the Dgraph database.
func (d *Client) NewTxn() interface{} {
	return d.client.NewTxn()
}

// NewReadOnlyTxn creates a new read-only transaction for querying the Dgraph database.
func (d *Client) NewReadOnlyTxn() interface{} {
	return d.client.NewReadOnlyTxn()
}

// HealthCheck performs a basic health check by pinging the Dgraph server.
// HealthCheck performs a basic health check by pinging the Dgraph server.
func (d *Client) HealthCheck(ctx context.Context) (any, error) {
	healthResponse, err := d.client.NewTxn().Query(ctx, `{
        health(func: has(dgraph.type)) {
            status
        }
    }`)

	if err != nil || len(healthResponse.Json) == 0 {
		d.logger.Error("Dgraph health check failed: ", err)
		return nil, errHealthCheckFailed
	}

	d.logger.Log("Dgraph health check passed.")
	return nil, nil
}

// Close closes the Dgraph client connection.
func (d *Client) Close() error {
	return d.conn.Close()
}
