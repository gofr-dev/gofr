package clickhouse

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"github.com/ClickHouse/clickhouse-go/v2"
)

// Config holds the configuration needed to connect to the Clickhouse database.
type Config struct {
	Hosts    string
	Username string
	Password string
	Database string
}

// Client provides methods for interacting with the Clickhouse database, including query execution, data retrieval, and health checks.
type Client struct {
	conn    Conn
	config  Config
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

var errStatusDown = errors.New("status down")

// New initializes a Clickhouse client with the provided configuration.
//
// Metrics and Logger must be initialized before calling the Connect method.
//
// Usage:
//	client := clickhouse.New(config)
//	client.UseLogger(Logger())
//	client.UseMetrics(Metrics())
//	client.Connect()
func New(config Config) *Client {
	return &Client{config: config}
}

// UseLogger sets the logger for the Clickhouse client.
func (c *Client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the Clickhouse client.
func (c *Client) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for the Clickhouse client.
func (c *Client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

// Connect establishes a connection to the Clickhouse database using the configuration provided during initialization.
func (c *Client) Connect() {
	var err error
	c.logger.Debugf("Connecting to Clickhouse at %v to database %v", c.config.Hosts, c.config.Database)

	clickHouseBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_clickhouse_stats", "Response time of Clickhouse queries in microseconds.", clickHouseBuckets...)
	c.metrics.NewGauge("app_clickhouse_open_connections", "Number of open Clickhouse connections.")
	c.metrics.NewGauge("app_clickhouse_idle_connections", "Number of idle Clickhouse connections.")

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

	if err != nil {
		c.logger.Errorf("Error connecting to Clickhouse: %v", err)
		return
	}

	if err = c.conn.Ping(ctx); err != nil {
		c.logger.Errorf("Ping failed: %v", err)
	} else {
		c.logger.Logf("Successfully connected to Clickhouse")
	}

	go pushDBMetrics(c.conn, c.metrics)
}

func pushDBMetrics(conn Conn, metrics Metrics) {
	const frequency = 10 * time.Second
	for {
		if conn != nil {
			stats := conn.Stats()
			metrics.SetGauge("app_clickhouse_open_connections", float64(stats.Open))
			metrics.SetGauge("app_clickhouse_idle_connections", float64(stats.Idle))
			time.Sleep(frequency)
		}
	}
}

// Exec executes a query for DDL or simple statements.
//
// Avoid using it for larger inserts or query iterations.
func (c *Client) Exec(ctx context.Context, query string, args ...any) error {
	tracedCtx, span := c.addTrace(ctx, "exec", query)
	defer c.sendOperationStats(time.Now(), "Exec", query, "exec", span, args...)
	return c.conn.Exec(tracedCtx, query, args...)
}

// Select retrieves data and populates it into the provided destination.
//
// Example:
//	type User struct {
//		ID   string `ch:"id"`
//		Name string `ch:"name"`
//		Age  int    `ch:"age"`
//	}
//	var users []User
//	err := client.Select(ctx, &users, "SELECT * FROM users")
func (c *Client) Select(ctx context.Context, dest any, query string, args ...any) error {
	tracedCtx, span := c.addTrace(ctx, "select", query)
	defer c.sendOperationStats(time.Now(), "Select", query, "select", span, args...)
	return c.conn.Select(tracedCtx, dest, query, args...)
}

// AsyncInsert performs an asynchronous insert operation.
//
// The `wait` parameter specifies whether to wait for the server to complete the insert or to respond immediately.
func (c *Client) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	tracedCtx, span := c.addTrace(ctx, "async-insert", query)
	defer c.sendOperationStats(time.Now(), "AsyncInsert", query, "async-insert", span, args...)
	return c.conn.AsyncInsert(tracedCtx, query, wait, args...)
}

// HealthCheck verifies the health of the Clickhouse connection by pinging the database.
func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	h := Health{
		Details: map[string]interface{}{
			"host":     c.config.Hosts,
			"database": c.config.Database,
		},
	}

	if err := c.conn.Ping(ctx); err != nil {
		h.Status = "DOWN"
		return &h, errStatusDown
	}

	h.Status = "UP"
	return &h, nil
}

// Helper methods for tracing and metrics.
func (c *Client) addTrace(ctx context.Context, method, query string) (context.Context, trace.Span) {
	if c.tracer != nil {
		tracedCtx, span := c.tracer.Start(ctx, fmt.Sprintf("clickhouse-%v", method))
		span.SetAttributes(attribute.String("clickhouse.query", query))
		return tracedCtx, span
	}
	return ctx, nil
}

func (c *Client) sendOperationStats(start time.Time, methodType, query, method string, span trace.Span, args ...interface{}) {
	duration := time.Since(start).Microseconds()
	c.logger.Debug(&Log{Type: methodType, Query: query, Duration: duration, Args: args})
	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64(fmt.Sprintf("clickhouse.%v.duration", method), duration))
	}
	c.metrics.RecordHistogram(context.Background(), "app_clickhouse_stats", float64(duration), "hosts", c.config.Hosts, "database", c.config.Database, "type", getOperationType(query))
}

func getOperationType(query string) string {
	query = strings.TrimSpace(query)
	return strings.Split(query, " ")[0]
}
