package oracle

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"time"

	// Import for Oracle driver registration.
	_ "github.com/godror/godror"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"gofr.dev/pkg/gofr/container"
)

var (
	_ container.OracleDB = (*Client)(nil)
	_ container.OracleTx = (*oracleTx)(nil)
)

type Config struct {
	Host             string
	Port             int
	Username         string
	Password         string
	Service          string // or SID.
	ConnectionString string
}

type Client struct {
	conn    Connection
	config  Config
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

var (
	errStatusDown      = errors.New("status down")
	errInvalidDestType = errors.New("dest must be *[]map[string]any")
	errNoConnection    = errors.New("oracle connection not established")
	errInvalidConnType = errors.New("invalid connection type")
)

const (
	StatusUp   = "UP"
	StatusDown = "DOWN"
)

func New(config *Config) *Client {
	return &Client{config: *config}
}

func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

func (c *Client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

func (c *Client) Connect() {
	// Validation: check if host is non-empty.
	if c.config.Host == "" {
		c.logger.Errorf("invalid OracleDB host: host is empty")
		return
	}

	// Validation: check if port is within a valid range.
	if c.config.Port <= 0 || c.config.Port > 65535 {
		c.logger.Errorf("invalid OracleDB port: %v", c.config.Port)
		return
	}

	c.logger.Debugf("connecting to OracleDB using connection string")
	dsn := fmt.Sprintf(`user=%q password=%q connectString=%q libDir="/Users/zopdev/oracle-client/lib" wallet_location="/Users/zopdev/wallet"`,
		c.config.Username, c.config.Password, c.config.ConnectionString)

	db, err := sql.Open("godror", dsn)

	if err != nil {
		c.logger.Errorf("error while connecting to OracleDB: %v", err)

		return
	}

	c.conn = &sqlConn{db: db}

	if err = c.conn.Ping(context.Background()); err != nil {
		c.logger.Errorf("ping failed with error %v", err)
	} else {
		c.logger.Logf("successfully connected to OracleDB")
	}
}

// Exec executes a non-query SQL statement (such as INSERT, UPDATE, or DELETE) against the Oracle database.
// It enables callers to run statements that modify data or schema without returning any result sets.
// This includes common operations like data mutation, transaction management, or schema changes (DDL).
// The method provides a standardized entry point for write and schema operations across gofr’s supported databases,
// ensuring consistent usage patterns and compatibility with the gofr datasource interface conventions.
func (c *Client) Exec(ctx context.Context, query string, args ...any) error {
	tracedCtx, span := c.addTrace(ctx, "exec", query)

	err := c.conn.Exec(tracedCtx, query, args...)

	defer c.sendOperationStats(time.Now(), "Exec", query, "exec", span, args...)

	return err
}

// Select executes a SELECT query and scans the resulting rows into dest.
// The dest parameter should be a pointer to a slice or other suitable container.
// Query parameters can be passed via args to replace placeholders.
func (c *Client) Select(ctx context.Context, dest any, query string, args ...any) error {
	tracedCtx, span := c.addTrace(ctx, "select", query)

	if reflect.TypeOf(dest).Kind() != reflect.Ptr || reflect.TypeOf(dest).Elem().Kind() != reflect.Slice {
		return errInvalidDestType
	}

	err := c.conn.Select(tracedCtx, dest, query, args...)

	defer c.sendOperationStats(time.Now(), "Select", query, "select", span, args...)

	return err
}

// oracleTx wraps a sql.Tx to implement the Txn interface.
type oracleTx struct {
	tx     *sql.Tx
	logger Logger
}

// Begin starts a new transaction.
func (c *Client) Begin() (container.OracleTx, error) {
	if c.conn == nil {
		return nil, errNoConnection
	}

	start := time.Now()

	// Get the underlying SQL DB.
	sqlConn, ok := c.conn.(*sqlConn)
	if !ok {
		return nil, errInvalidConnType
	}

	// Begin a new SQL transaction.
	tx, err := sqlConn.db.BeginTx(context.Background(), nil)
	if err != nil {
		c.logger.Errorf("failed to begin transaction: %v", err)
		return nil, err
	}

	c.logger.Debug(&Log{
		Type:     "Begin",
		Duration: time.Since(start).Microseconds(),
	})

	return &oracleTx{tx: tx, logger: c.logger}, nil
}

func (t *oracleTx) ExecContext(ctx context.Context, query string, args ...any) error {
	start := time.Now()
	_, err := t.tx.ExecContext(ctx, query, args...)

	if t.logger != nil {
		t.logger.Debug(&Log{
			Type:     "Tx-Exec",
			Query:    query,
			Duration: time.Since(start).Microseconds(),
			Args:     args,
		})
	}

	return err
}

// Extract the row scanning logic to reduce complexity.
func scanRows(rows *sql.Rows) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))

		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]any)
		for columnIndex, columnName := range columns {
			rowMap[columnName] = values[columnIndex]
		}

		results = append(results, rowMap)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return results, nil
}

func (t *oracleTx) SelectContext(ctx context.Context, dest any, query string, args ...any) error {
	start := time.Now()

	if reflect.TypeOf(dest).Kind() != reflect.Ptr || reflect.TypeOf(dest).Elem().Kind() != reflect.Slice {
		return errInvalidDestType
	}

	rows, err := t.tx.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	results, err := scanRows(rows)
	if err != nil {
		return err
	}

	// Set the result to dest
	type Destination = []map[string]any

	p, ok := dest.(*Destination)

	if !ok {
		return errInvalidDestType
	}

	*p = results

	if t.logger != nil {
		t.logger.Debug(&Log{
			Type:     "Tx-Select",
			Query:    query,
			Duration: time.Since(start).Microseconds(),
			Args:     args,
		})
	}

	return nil
}

func (t *oracleTx) Commit() error {
	start := time.Now()
	err := t.tx.Commit()

	if t.logger != nil {
		t.logger.Debug(&Log{
			Type:     "Tx-Commit",
			Duration: time.Since(start).Microseconds(),
		})

		if err != nil {
			t.logger.Errorf("transaction commit failed: %v", err)
		}
	}

	return err
}

func (t *oracleTx) Rollback() error {
	start := time.Now()
	err := t.tx.Rollback()

	if t.logger != nil {
		t.logger.Debug(&Log{
			Type:     "Tx-Rollback",
			Duration: time.Since(start).Microseconds(),
		})

		if err != nil {
			t.logger.Errorf("transaction rollback failed: %v", err)
		}
	}

	return err
}

// sendOperationStats collects and sends operation metrics for monitoring purposes.
// It tracks execution times, counts, and error occurrences related to database operations.
func (c *Client) sendOperationStats(start time.Time, methodType, query, method string, span trace.Span, args ...any) {
	duration := time.Since(start).Microseconds()

	c.logger.Debug(&Log{
		Type:     methodType,
		Query:    query,
		Duration: duration,
		Args:     args,
	})

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64(fmt.Sprintf("oracle.%v.duration", method), duration))
	}
}

type Health struct {
	Status string `json:"status,omitempty"`
	// Details provide additional runtime metadata (host, service) to aid debugging.
	Details map[string]any `json:"details,omitempty"`
}

func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	h := Health{
		Details: make(map[string]any),
	}

	h.Details["host"] = c.config.Host
	h.Details["database"] = c.config.Service

	err := c.conn.Ping(ctx)
	if err != nil {
		h.Status = StatusDown
		return &h, errStatusDown
	}

	h.Status = StatusUp

	return &h, nil
}

// addTrace adds tracing information to the current context or operation.
// It records metadata such as correlation IDs or span details for distributed tracing.
func (c *Client) addTrace(ctx context.Context, method, query string) (context.Context, trace.Span) {
	if c.tracer != nil {
		ctxWithTrace, span := c.tracer.Start(ctx, fmt.Sprintf("oracle-%v", method))

		span.SetAttributes(
			attribute.String("oracle.query", query),
		)

		return ctxWithTrace, span
	}

	return ctx, nil
}

type sqlConn struct{ db *sql.DB }

func (s *sqlConn) Exec(ctx context.Context, query string, args ...any) error {
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

func (s *sqlConn) Select(ctx context.Context, dest any, query string, args ...any) error {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	var results []map[string]any

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))

		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}

		rowMap := make(map[string]any)
		for columnIndex, columnName := range columns {
			rowMap[columnName] = values[columnIndex]
		}

		results = append(results, rowMap)
	}

	if rows.Err() != nil {
		return rows.Err()
	}

	// Set the result to dest (must be *[]map[string]any).
	type Destination = []map[string]any

	p, ok := dest.(*Destination)
	if !ok {
		return errInvalidDestType
	}

	*p = results

	return nil
}

func (s *sqlConn) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
