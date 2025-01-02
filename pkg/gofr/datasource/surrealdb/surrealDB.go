package surrealdb

import (
	"context"
	"errors"
	"fmt"
	surrealdb "github.com/surrealdb/surrealdb.go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"time"
)

var (
	ErrNotConnected = errors.New("not connected to database")
)

type Config struct {
	Host       string
	Port       int
	Username   string
	Password   string
	Namespace  string
	Database   string
	TLSEnabled bool
}

type Client struct {
	config  Config
	db      *surrealdb.DB
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

func New(config Config) *Client {
	return &Client{
		config: config,
	}
}

func (c *Client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

func (c *Client) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

func (c *Client) UseTracer(tracer interface{}) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

func (c *Client) Connect() {
	endpoint := fmt.Sprintf("ws://%s:%d", c.config.Host, c.config.Port)
	if c.config.TLSEnabled {
		endpoint = fmt.Sprintf("https://%s:%d", c.config.Host, c.config.Port)
	}

	if c.logger != nil {
		c.logger.Debugf("connecting to SurrealDB at %s", endpoint)
	}

	db, err := surrealdb.New(endpoint)
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("failed to connect to SurrealDB: %v", err)
		}
		return
	}

	err = db.Use(c.config.Namespace, c.config.Database)
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("unable to set the namespace and database for SurrealDB: %v", err)
		}
		return
	}

	c.db = db

	if c.metrics != nil {
		c.metrics.NewHistogram("surreal_db_operation_duration", "Duration of SurrealDB operations")
	}

	if c.config.Username != "" && c.config.Password != "" {
		_, err := db.SignIn(&surrealdb.Auth{
			Username: c.config.Username,
			Password: c.config.Password,
		})
		if err != nil {
			if c.logger != nil {
				c.logger.Errorf("failed to sign in to SurrealDB: %v", err)
			}
			return
		}
	}

	if c.logger != nil {
		c.logger.Debugf("successfully connected to SurrealDB")
	}
}

func (c *Client) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

func (c *Client) UseNamespace(ns string) error {
	if c.db == nil {
		return ErrNotConnected
	}
	return c.db.Use(ns, "")
}

func (c *Client) UseDatabase(db string) error {
	if c.db == nil {
		return ErrNotConnected
	}
	return c.db.Use("", db)
}

func (c *Client) Query(ctx context.Context, query string, vars map[string]interface{}) ([]interface{}, error) {
	if c.db == nil {
		return nil, ErrNotConnected
	}

	result, err := surrealdb.Query[any](c.db, query, vars)
	if err != nil {
		return nil, err
	}

	fmt.Println("Raw Query Result:", result)

	if c.metrics != nil {
		c.metrics.RecordHistogram(ctx, "surreal_db_operation_duration", 0, "operation", "query")
	}

	resp := make([]interface{}, 0)

	for _, r := range *result {
		if r.Status == "OK" {
			if res, ok := r.Result.([]interface{}); ok {
				resp = append(resp, res...)
			} else {

				if c.logger != nil {
					c.logger.Errorf("unexpected result type: %v", r.Result)
				}
			}
		} else {
			if c.logger != nil {
				c.logger.Errorf("query result error: %v", r.Status)
			}
		}
	}

	return resp, nil
}

func (c *Client) Select(ctx context.Context, table string) ([]map[string]interface{}, error) {
	if c.db == nil {
		return nil, ErrNotConnected
	}

	result, err := surrealdb.Select[[]any](c.db, table)
	if err != nil {
		return nil, fmt.Errorf("failed to select from table %s: %w", table, err)
	}

	var resSlice []map[string]interface{}

	for _, record := range *result {
		recordMap, ok := record.(map[interface{}]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected record format: %v", record)
		}

		resMap := make(map[string]interface{})
		for k, v := range recordMap {
			keyStr, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("non-string key encountered: %v", k)
			}
			resMap[keyStr] = v
		}

		resSlice = append(resSlice, resMap)
	}

	return resSlice, nil
}

func (c *Client) Create(ctx context.Context, table string, data interface{}) (map[string]interface{}, error) {
	if c.db == nil {
		return nil, ErrNotConnected
	}

	result, err := surrealdb.Create[map[string]interface{}](c.db, table, data)
	if err != nil {
		return nil, fmt.Errorf("failed to create record in table %s: %w", table, err)
	}

	if c.metrics != nil {
		c.metrics.RecordHistogram(ctx, "surreal_db_operation_duration", 0, "operation", "create")
	}

	return *result, nil
}

func (c *Client) Update(ctx context.Context, table string, id string, data interface{}) (interface{}, error) {
	if c.db == nil {
		return nil, ErrNotConnected
	}

	result, err := surrealdb.Update[any, string](c.db, table, data)
	if err != nil {
		return nil, err
	}

	resultSlice := (*result).([]interface{})

	resMap := make(map[string]interface{})

	for _, r := range resultSlice {
		rMap := r.(map[interface{}]interface{})
		for k, v := range rMap {
			kStr := k.(string)

			resMap[kStr] = v
		}
	}
	return resMap, nil
}

func (c *Client) Delete(ctx context.Context, table string, id string) (any, error) {
	if c.db == nil {
		return nil, ErrNotConnected
	}
	result, err := surrealdb.Delete[any, string](c.db, table)
	if err != nil {
		return nil, err
	}

	return *result, nil
}

func (c *Client) sendOperationStats(ql *QueryLog, startTime time.Time, method string, span trace.Span) {
	duration := time.Since(startTime).Microseconds()

	ql.Duration = duration

	c.logger.Debug(ql)

	//c.metrics.RecordHistogram(context.Background(), "app_surreal_stats", float64(duration), "hostname",
	//	"namespace", ql.Namespace, "database", ql.Database, "query", ql.Query)

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64(fmt.Sprintf("surreal.%v.duration", method), duration))
	}
}

type Health struct {
	Status  string                 `json:"status,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	h := Health{
		Details: make(map[string]interface{}),
	}
	h.Details["host"] = fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	h.Details["namespace"] = c.config.Namespace
	h.Details["database"] = c.config.Database

	if c.db == nil {
		h.Status = "DOWN"
		h.Details["error"] = "Database client is not connected"
		return &h, ErrNotConnected
	}

	query := "RETURN 'SurrealDB Health Check'"
	_, err := surrealdb.Query[any](c.db, query, nil)
	if err != nil {
		h.Status = "DOWN"
		h.Details["error"] = fmt.Sprintf("Failed to execute health check query: %v", err)
		return &h, err
	}

	h.Status = "UP"
	return &h, nil
}
