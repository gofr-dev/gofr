package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	
)


// Config defines the configuration needed to connect to a Clickhouse database.
type Config struct {
	Hosts    string // Comma-separated list of Clickhouse host addresses.
	Username string // Username for authentication.
	Password string // Password for authentication.
	Database string // Target database name.
}

// Client represents the Clickhouse client, encapsulating connection details, configuration, logging, metrics, and tracing.
type Client struct {
	conn    Conn        // The database connection interface.
	config  Config      // Configuration for the Clickhouse connection.
	logger  Logger      // Logger interface for logging messages.
	metrics Metrics     // Metrics interface for monitoring database operations.
	tracer  trace.Tracer // OpenTelemetry tracer for distributed tracing.
}

// New initializes a new Clickhouse client with the provided configuration.
// Metrics and logger must be set separately before calling the Connect method.
func New(config Config) *Client {
	return &Client{config: config}
}

// UseLogger sets the logger for the Clickhouse client.
// The logger must implement the Logger interface.
func (c *Client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the Clickhouse client.
// The metrics must implement the Metrics interface.
func (c *Client) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the OpenTelemetry tracer for the Clickhouse client.
// The tracer must implement the trace.Tracer interface.
func (c *Client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

// Connect establishes a connection to the Clickhouse database using the client's configuration.
// It also registers metrics for monitoring the database connection.
func (c *Client) Connect() {
	var err error

	// Log the connection attempt.
	c.logger.Debugf("connecting to Clickhouse db at %v to database %v", c.config.Hosts, c.config.Database)

	// Define histogram and gauge metrics for monitoring query performance and connection statistics.
	clickHouseBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_clickhouse_stats", "Response time of Clickhouse queries in microseconds.", clickHouseBuckets...)
	c.metrics.NewGauge("app_clickhouse_open_connections", "Number of open Clickhouse connections.")
	c.metrics.NewGauge("app_clickhouse_idle_connections", "Number of idle Clickhouse connections.")

	// Parse the host addresses and initialize the Clickhouse connection.
	addresses := strings.Split(c.config.Hosts, ",")
	ctx := context.Background()
	c.conn, err = clickhouse.Open(&clickhouse.Options{
		Addr: addresses,
		Auth: clickhouse.Auth{
			Database: c.config.Database,
			Username: c.config.Username,
			Password: c.config.Password,
		},
	})

	// Handle connection errors.
	if err != nil {
		c.logger.Errorf("error while connecting to Clickhouse %v", err)
		return
	}

	// Attempt to ping the database to ensure connectivity.
	if err = c.conn.Ping(ctx); err != nil {
		c.logger.Errorf("ping failed with error %v", err)
	} else {
		c.logger.Logf("successfully connected to ClickhouseDB")
	}

	// Start a goroutine to push database metrics periodically.
	go pushDBMetrics(c.conn, c.metrics)
}

// pushDBMetrics periodically updates metrics for open and idle database connections.
func pushDBMetrics(conn Conn, metrics Metrics) {
	const frequency = 10 // Frequency in seconds for pushing metrics.

	for {
		if conn != nil {
			stats := conn.Stats()

			metrics.SetGauge("app_clickhouse_open_connections", float64(stats.Open))
			metrics.SetGauge("app_clickhouse_idle_connections", float64(stats.Idle))

			time.Sleep(frequency * time.Second)
		}
	}
}

// Exec executes a simple SQL statement or DDL command.
// It should not be used for large inserts or queries requiring iterations.
func (c *Client) Exec(ctx context.Context, query string, args ...any) error {
	// Start a trace for the operation.
	tracedCtx, span := c.addTrace(ctx, "exec", query)

	// Execute the query.
	err := c.conn.Exec(tracedCtx, query, args...)

	// Record metrics and tracing stats after execution.
	defer c.sendOperationStats(time.Now(), "Exec", query, "exec", span, args...)

	return err
}

// Select retrieves rows from the database and maps them into a destination struct.
// Example:
// type User struct {
//     Id   string `ch:"id"`
//     Name string `ch:"name"`
//     Age  string `ch:"age"`
// }
// var users []User
// err := client.Select(ctx, &users, "SELECT * FROM users")
func (c *Client) Select(ctx context.Context, dest any, query string, args ...any) error {
	// Start a trace for the operation.
	tracedCtx, span := c.addTrace(ctx, "select", query)

	// Execute the query and map the result to the destination.
	err := c.conn.Select(tracedCtx, dest, query, args...)

	// Record metrics and tracing stats after execution.
	defer c.sendOperationStats(time.Now(), "Select", query, "select", span, args...)

	return err
}

// AsyncInsert inserts data into the database asynchronously.
// The `wait` parameter specifies whether to wait for the server to complete the insert.
func (c *Client) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	// Start a trace for the operation.
	tracedCtx, span := c.addTrace(ctx, "async-insert", query)

	// Execute the asynchronous insert.
	err := c.conn.AsyncInsert(tracedCtx, query, wait, args...)

	// Record metrics and tracing stats after execution.
	defer c.sendOperationStats(time.Now(), "AsyncInsert", query, "async-insert", span, args...)

	return err
}

// sendOperationStats records metrics and updates tracing attributes for a completed database operation.
func (c *Client) sendOperationStats(start time.Time, methodType, query string, method string, span trace.Span, args ...interface{}) {
	duration := time.Since(start).Microseconds()

	// Log the operation details.
	c.logger.Debug(&Log{
		Type:     methodType,
		Query:    query,
		Duration: duration,
		Args:     args,
	})

	// Update tracing attributes.
	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64(fmt.Sprintf("clickhouse.%v.duration", method), duration))
	}

	// Record the duration as a histogram metric.
	c.metrics.RecordHistogram(context.Background(), "app_clickhouse_stats", float64(duration), "hosts", c.config.Hosts,
		"database", c.config.Database, "type", getOperationType(query))
}

// getOperationType extracts the operation type (e.g., SELECT, INSERT) from a query.
func getOperationType(query string) string {
	query = strings.TrimSpace(query)
	words := strings.Split(query, " ")
	return words[0]
}

// HealthCheck checks the health of the Clickhouse client by pinging the database.
// Returns the health status and details.
func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	h := Health{
		Details: make(map[string]interface{}),
	}

	h.Details["host"] = c.config.Hosts
	h.Details["database"] = c.config.Database

	// Ping the database to check connectivit
