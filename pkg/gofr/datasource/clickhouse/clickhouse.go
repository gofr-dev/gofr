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

type Config struct {
	Hosts    string
	Username string
	Password string
	Database string
}

type client struct {
	conn    Conn
	config  Config
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

var errStatusDown = errors.New("status down")

// New initializes Clickhouse client with the provided configuration.
// Metrics, Logger has to be initialized before calling the Connect method.
// Usage:
//
//	client.UseLogger(Logger())
//	client.UseMetrics(Metrics())
//
//	client.Connect()
//
//nolint:revive // client is unexported as we want the user to implement the Conn interface.
func New(config Config) *client {
	return &client{config: config}
}

// UseLogger sets the logger for the Clickhouse client.
func (c *client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the Clickhouse client.
func (c *client) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for Clickhouse client.
func (c *client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

// Connect establishes a connection to Clickhouse and registers metrics using the provided configuration when the client was Created.
func (c *client) Connect() {
	var err error

	c.logger.Logf("connecting to clickhouse db at %v to database %v", c.config.Hosts, c.config.Database)

	clickHouseBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_clickhouse_stats", "Response time of Clickhouse queries in milliseconds.", clickHouseBuckets...)

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
		c.logger.Errorf("error while connecting to clickhouse %v", err)

		return
	}

	if err = c.conn.Ping(ctx); err != nil {
		c.logger.Errorf("ping failed with error %v", err)
	} else {
		c.logger.Logf("successfully connected to clickhouseDB")
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
func (c *client) Exec(ctx context.Context, query string, args ...any) error {
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
func (c *client) Select(ctx context.Context, dest any, query string, args ...any) error {
	tracedCtx, span := c.addTrace(ctx, "select", query)

	err := c.conn.Select(tracedCtx, dest, query, args...)

	defer c.sendOperationStats(time.Now(), "Select", query, "select", span, args...)

	return err
}

// AsyncInsert allows the user to specify whether the client should wait for the server to complete the insert or
// respond once the data has been received.
func (c *client) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
	tracedCtx, span := c.addTrace(ctx, "async-insert", query)

	err := c.conn.AsyncInsert(tracedCtx, query, wait, args...)

	defer c.sendOperationStats(time.Now(), "AsyncInsert", query, "async-insert", span, args...)

	return err
}

func (c *client) sendOperationStats(start time.Time, methodType, query string, method string,
	span trace.Span, args ...interface{}) {
	duration := time.Since(start).Milliseconds()

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

func getOperationType(query string) string {
	query = strings.TrimSpace(query)
	words := strings.Split(query, " ")

	return words[0]
}

type Health struct {
	Status  string                 `json:"status,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthCheck checks the health of the MongoDB client by pinging the database.
func (c *client) HealthCheck(ctx context.Context) (any, error) {
	h := Health{
		Details: make(map[string]interface{}),
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

func (c *client) addTrace(ctx context.Context, method, query string) (context.Context, trace.Span) {
	if c.tracer != nil {
		contextWithTrace, span := c.tracer.Start(ctx, fmt.Sprintf("clickhouse-%v", method))

		span.SetAttributes(
			attribute.String("clickhouse.query", query),
		)

		return contextWithTrace, span
	}

	return ctx, nil
}
