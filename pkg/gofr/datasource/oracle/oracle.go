package oracle

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"

    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
    _ "github.com/godror/godror"
)

type Config struct {
    Host     string
    Port     int
    Username string
    Password string
    Service  string // or SID
}

type Client struct {
    conn    Conn
    config  Config
    logger  Logger
    metrics Metrics
    tracer  trace.Tracer
}

var errStatusDown = errors.New("status down")

func New(config Config) *Client {
    return &Client{config: config}
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
    c.logger.Debugf("connecting to OracleDB at %v:%v/%v", c.config.Host, c.config.Port, c.config.Service)
    dsn := fmt.Sprintf(`user="%s" password="%s" connectString="%s:%d/%s"`,
        c.config.Username, c.config.Password, c.config.Host, c.config.Port, c.config.Service)
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
    // Metrics registration can be added here.
}

func (c *Client) Exec(ctx context.Context, query string, args ...any) error {
    tracedCtx, span := c.addTrace(ctx, "exec", query)
    err := c.conn.Exec(tracedCtx, query, args...)
    defer c.sendOperationStats(time.Now(), "Exec", query, "exec", span, args...)
    return err
}

func (c *Client) Select(ctx context.Context, dest any, query string, args ...any) error {
    tracedCtx, span := c.addTrace(ctx, "select", query)
    err := c.conn.Select(tracedCtx, dest, query, args...)
    defer c.sendOperationStats(time.Now(), "Select", query, "select", span, args...)
    return err
}

func (c *Client) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
    tracedCtx, span := c.addTrace(ctx, "async-insert", query)
    err := c.conn.AsyncInsert(tracedCtx, query, wait, args...)
    defer c.sendOperationStats(time.Now(), "AsyncInsert", query, "async-insert", span, args...)
    return err
}

type Health struct {
    Status  string         `json:"status,omitempty"`
    Details map[string]any `json:"details,omitempty"`
}

func (c *Client) HealthCheck(ctx context.Context) (any, error) {
    h := Health{Details: make(map[string]any)}
    h.Details["host"] = c.config.Host
    h.Details["database"] = c.config.Service
    err := c.conn.Ping(ctx)
    if err != nil {
        h.Status = "DOWN"
        return &h, errStatusDown
    }
    h.Status = "UP"
    return &h, nil
}

func (c *Client) sendOperationStats(start time.Time, methodType, query, method string, span trace.Span, args ...any) {
    duration := time.Since(start).Microseconds()
    c.logger.Debug(&Log{Type: methodType, Query: query, Duration: duration, Args: args})
    if span != nil {
        defer span.End()
        span.SetAttributes(attribute.Int64(fmt.Sprintf("oracle.%v.duration", method), duration))
    }
    // Metrics recording can be added here.
}

func (c *Client) addTrace(ctx context.Context, method, query string) (context.Context, trace.Span) {
    if c.tracer != nil {
        ctxWithTrace, span := c.tracer.Start(ctx, fmt.Sprintf("oracle-%v", method))
        span.SetAttributes(attribute.String("oracle.query", query))
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

    var results []map[string]interface{}
    for rows.Next() {
        values := make([]interface{}, len(columns))
        valuePtrs := make([]interface{}, len(columns))
        for i := range columns {
            valuePtrs[i] = &values[i]
        }

        if err := rows.Scan(valuePtrs...); err != nil {
            return err
        }

        rowMap := make(map[string]interface{})
        for i, col := range columns {
            rowMap[col] = values[i]
        }
        results = append(results, rowMap)
    }
    // Set the result to dest (must be *[]map[string]interface{})
    p, ok := dest.(*[]map[string]interface{})
    if !ok {
        return errors.New("dest must be *[]map[string]interface{}")
    }
    *p = results
    return nil
}

func (s *sqlConn) AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error {
    return s.Exec(ctx, query, args...)
}
func (s *sqlConn) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }
func (s *sqlConn) Stats() any                    { return s.db.Stats() }
