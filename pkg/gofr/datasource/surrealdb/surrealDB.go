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
	// ErrNotConnected indicates that the database client is not connected.
	ErrNotConnected = errors.New("not connected to database")
)

// Config represents the configuration required to connect to SurrealDB.
type Config struct {
	Host       string
	Port       int
	Username   string
	Password   string
	Namespace  string
	Database   string
	TLSEnabled bool
}

// Client represents a client to interact with SurrealDB.
type Client struct {
	config  Config
	db      *surrealdb.DB
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

// New creates a new Client with the provided configuration.
func New(config Config) *Client {
	return &Client{
		config: config,
	}
}

// UseLogger sets a custom logger for the Client.
func (c *Client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets a custom metrics recorder for the Client.
func (c *Client) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets a custom tracer for the Client.
func (c *Client) UseTracer(tracer interface{}) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

// Connect establishes a connection to the SurrealDB server using the client's configuration.
func (c *Client) Connect() error {
	scheme := "ws"
	if c.config.TLSEnabled {
		scheme = "https"
	}
	endpoint := fmt.Sprintf("%s://%s:%d", scheme, c.config.Host, c.config.Port)

	if c.logger != nil {
		c.logger.Debugf("connecting to SurrealDB at %s", endpoint)
	}

	db, err := surrealdb.New(endpoint)
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("failed to connect to SurrealDB: %v", err)
		}
		return err
	}

	if db == nil {
		err := fmt.Errorf("failed to connect to SurrealDB: no valid database instance")
		if c.logger != nil {
			c.logger.Errorf("error")
		}
		return err
	}

	err = db.Use(c.config.Namespace, c.config.Database)
	if err != nil {
		if c.logger != nil {
			c.logger.Errorf("unable to set the namespace and database for SurrealDB: %v", err)
		}
		return err
	}

	c.db = db

	if c.metrics != nil {
		c.metrics.NewHistogram("surreal_db_operation_duration", "Duration of SurrealDB operations")
	}

	if (c.config.Username == "" && c.config.Password != "") || (c.config.Username != "" && c.config.Password == "") {
		return errors.New("both username and password must be provided")
	}

	if c.config.Username != "" && c.config.Password != "" {
		_, err := db.SignIn(&surrealdb.Auth{
			Username: c.config.Username,
			Password: c.config.Password,
		})
		if err != nil {
			c.logger.Errorf("failed to sign in to SurrealDB: %v", err)
			return err
		}
	}
	c.logger.Debugf("successfully connected to SurrealDB")

	return nil
}

// Close closes the database connection.
func (c *Client) Close() error {
	if c.db != nil {
		return c.db.Close()
	}

	return nil
}

// UseNamespace switches the active namespace for the database connection.
func (c *Client) UseNamespace(ns string) error {
	if c.db == nil {
		return ErrNotConnected
	}

	return c.db.Use(ns, "")
}

// UseDatabase switches the active database for the connection.
func (c *Client) UseDatabase(db string) error {
	if c.db == nil {
		return ErrNotConnected
	}

	return c.db.Use("", db)
}

// Query executes a query on the SurrealDB instance.
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
				c.logger.Errorf("unexpected result type: %v", r.Result)
			}
		} else {
			c.logger.Errorf("query result error: %v", r.Status)
		}
	}

	return resp, nil
}

// Select retrieves all records from a specific table in SurrealDB.
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

// Create inserts a new record into the specified table in SurrealDB.
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

// Update modifies an existing record in the specified table.
func (c *Client) Update(ctx context.Context, table, id string, data interface{}) (interface{}, error) {
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

// Delete removes a record from the specified table in SurrealDB.
func (c *Client) Delete(ctx context.Context, table, id string) (any, error) {

	if c.db == nil {
		return nil, ErrNotConnected
	}

	result, err := surrealdb.Delete[any, string](c.db, table)

	if err != nil {
		return nil, err
	}

	return *result, nil
}

// sendOperationStats logs and records metrics for a database operation.
func (c *Client) sendOperationStats(ql *QueryLog, startTime time.Time, method string, span trace.Span) {
	duration := time.Since(startTime).Microseconds()

	ql.Duration = duration

	c.logger.Debug(ql)

	c.metrics.RecordHistogram(context.Background(), "app_surreal_stats", float64(duration), "hostname",
		"namespace", ql.Namespace, "database", ql.Database, "query", ql.Query)

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64(fmt.Sprintf("surreal.%v.duration", method), duration))
	}
}

type Health struct {
	Status  string                 `json:"status,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthCheck performs a health check on the SurrealDB connection.
func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	const (
		statusDown = "DOWN"
		statusUP   = "UP"
	)

	h := Health{

		Details: make(map[string]interface{}),
	}
	h.Details["host"] = fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	h.Details["namespace"] = c.config.Namespace
	h.Details["database"] = c.config.Database

	if c.db == nil {
		h.Status = statusDown
		h.Details["error"] = "Database client is not connected"

		return &h, ErrNotConnected
	}

	query := "RETURN 'SurrealDB Health Check'"

	_, err := surrealdb.Query[any](c.db, query, nil)

	if err != nil {
		h.Status = statusDown
		h.Details["error"] = fmt.Sprintf("Failed to execute health check query: %v", err)

		return &h, err
	}

	h.Status = statusUP

	return &h, nil
}
