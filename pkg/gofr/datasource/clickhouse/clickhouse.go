// Package clickhouse provides a client for interacting with ClickHouse databases,
// supporting query execution, asynchronous inserts, and observability integration
// through logging, metrics, and tracing.
//
// It is designed to be used with the GoFr framework and allows configuration
// of connection parameters, observability tools, and health checks.
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

type Config struct {
	Hosts    string // Comma-separated list of ClickHouse server addresses.
	Username string // Username used for authentication.
	Password string // Password used for authentication.
	Database string // Name of the database to connect to.
}

// Client is a ClickHouse client implementation that wraps a Conn interface.
// It provides methods for executing queries, performing inserts, and collecting metrics.
type Client struct {
	conn    Conn
	config  Config
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

var errStatusDown = errors.New("status down")

// New initializes ClickHouse client with the provided configuration.
// Metrics, Logger has to be initialized before calling the Connect method.
// Usage:
//
//	client.UseLogger(Logger())
//	client.UseMetrics(Metrics())
//
//	client.Connect()
func New(config Config) *Client {
	return &Client{config: config}
}

// UseLogger sets the logger for the ClickHouse client.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the ClickHouse client.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for ClickHouse client.
func (c *Client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

// Connect establishes a connection to ClickHouse and registers metrics using the provided configuration when the client was Created.
func (c *Client) Connect() {
	var err error

	c.logger.Debugf("connecting to Clickhouse db at %v to database %v", c.config.Hosts, c.config.Database)

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
		c.logger.Errorf("error while connecting to Clickhouse %v", err)

		return
	}

	if err = c.conn.Ping(ctx); err != nil {
		c.logger.Errorf("ping failed with error %v", err)
	} else {
		c.logger.Logf("successfully connected to ClickhouseDB")
	}

	go pushDBMetrics(c.conn, c.metrics)
}

func pushDBMetrics(conn Conn, metrics Metrics) {
	const frequency = 10

	for {
		if conn != nil {
			stats := conn.Stats()

			metrics.SetGauge("app_clickhouse_open_connections", float64(stats.Open))
			metrics.SetGauge("app_clickhouse_idle_connections", float64(stats.Idle))

			time.Sleep(frequency * time.Second)
		}
	}
}

// Exec should be used for DDL and simple statements.
// It should not be used for larger inserts or query iterations.
func (c *Client) Exec(ctx context.Context, query string, args ...any) error {
	tracedCtx, span := c.addTrace(ctx, "exec", query)

	err := c.conn.Exec(tracedCtx, query, args...)

	defer c.sendOperationStats(time.Now(), "Exec", query, "exec", span, args...)

	return err
}

// Select method allows a set of response rows to be marshaled into a slice of structs with a single invocation..
// DB column names should be defined in the struct in `ch` tag.
// Example Usages:
//
//	type User struct {
//		Id   string `ch:"id"`
//		Name string `ch:"name"`
//		Age  string `ch:"age"`
//	}
//
// var user []User
//
// err = ctx.Clickhouse.Select(ctx, &user, "SELECT * FROM users") .
func (c *Client) Select(ctx context.Context, dest any, query string, args ...any) error {
	tracedCtx, span := c.addTrace(ctx, "select", query)

	err := c.conn.Select(tracedCtx, dest, query, args...)

	defer c.sendOperationStats(time.Now(), "Select", query, "select", span, args...)

	return err
}

// AsyncInsert allows the user to specify whether the client should wait for the server to complete the insert or
// respond once the data has been received.
func (c *Client) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	tracedCtx, span := c.addTrace(ctx, "async-insert", query)

	err := c.conn.AsyncInsert(tracedCtx, query, wait, args...)

	defer c.sendOperationStats(time.Now(), "AsyncInsert", query, "async-insert", span, args...)

	return err
}

// sendOperationStats records the duration of a database operation and logs the query context.
// It also attaches metrics and trace attributes if enabled.
func (c *Client) sendOperationStats(start time.Time, methodType, query string, method string,
	span trace.Span, args ...any) {
	duration := time.Since(start).Microseconds()

	c.logger.Debug(&Log{
		Type:     methodType,
		Query:    query,
		Duration: duration,
		Args:     args,
	})

	if span != nil {
		defer span.End()

		span.SetAttributes(attribute.Int64(fmt.Sprintf("clickhouse.%v.duration", method), duration))
	}

	c.metrics.RecordHistogram(context.Background(), "app_clickhouse_stats", float64(duration), "hosts", c.config.Hosts,
		"database", c.config.Database, "type", getOperationType(query))
}

// getOperationType extracts the operation type (e.g., SELECT, INSERT) from a query.
func getOperationType(query string) string {
	query = strings.TrimSpace(query)
	words := strings.Split(query, " ")

	return words[0]
}

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// HealthCheck checks the health of the MongoDB client by pinging the database.
func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	h := Health{
		Details: make(map[string]any),
	}

	h.Details["host"] = c.config.Hosts
	h.Details["database"] = c.config.Database

	err := c.conn.Ping(ctx)
	if err != nil {
		h.Status = "DOWN"

		return &h, errStatusDown
	}

	h.Status = "UP"

	return &h, nil
}

// addTrace starts a new trace span for the given operation and query.
func (c *Client) addTrace(ctx context.Context, method, query string) (context.Context, trace.Span) {
	if c.tracer != nil {
		contextWithTrace, span := c.tracer.Start(ctx, fmt.Sprintf("clickhouse-%v", method))

		span.SetAttributes(
			attribute.String("clickhouse.query", query),
		)

		return contextWithTrace, span
	}

	return ctx, nil
}
