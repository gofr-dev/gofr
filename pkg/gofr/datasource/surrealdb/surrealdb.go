package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	errNotConnected             = errors.New("not connected to database")
	errNoDatabaseInstance       = errors.New("failed to connect to SurrealDB: no valid database instance")
	errInvalidCredentialsConfig = errors.New("both username and password must be provided")
	errNoRecord                 = errors.New("no record found")
	errNoResult                 = errors.New("no result found in query response")
	errUnexpectedResult         = errors.New("unexpected result type: expected []any")
	errQueryError               = errors.New("query error")
)

const (
	schemeHTTP      = "http"
	schemeHTTPS     = "https"
	schemeWS        = "ws"
	schemeWSS       = "wss"
	schemeMemory    = "memory"
	schemeMem       = "mem"
	schemeSurrealkv = "surrealkv"
	statusOK        = "OK"

	defaultTimeout = 30 * time.Second
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
	config  *Config
	db      DB
	logger  Logger
	metrics Metrics
	tracer  trace.Tracer
}

// New creates a new Client with the provided configuration.
func New(config *Config) *Client {
	return &Client{
		config: config,
	}
}

// UseLogger sets a custom logger for the Client.
func (c *Client) UseLogger(customlogger any) {
	if l, ok := customlogger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets a custom metrics recorder for the Client.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets a custom tracer for the Client.
func (c *Client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

// newDB creates a new SurrealDB client using v1.0.0 API.
func newDB(ctx context.Context, connectionURL string) (DB, error) {
	// Use the new FromEndpointURLString which handles both HTTP and WebSocket connections
	db, err := surrealdb.FromEndpointURLString(ctx, connectionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create SurrealDB client: %w", err)
	}

	return NewDBWrapper(db), nil
}

// Connect establishes a connection to the SurrealDB database.
func (c *Client) Connect() {
	c.logger.Debugf("connecting to SurrealDB at %v:%v to database %v", c.config.Host, c.config.Port, c.config.Database)

	surrealDBBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}

	c.metrics.NewHistogram("app_surrealdb_stats", "Response time of SurrealDB operations in microseconds.", surrealDBBuckets...)
	c.metrics.NewGauge("app_surrealdb_open_connections", "Number of open SurrealDB connections.")

	endpoint := c.buildEndpoint()

	// Create context with timeout for connection
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	err := c.connectToDatabase(ctx, endpoint)
	if err != nil {
		return
	}

	err = c.setupNamespaceAndDatabase(ctx)
	if err != nil {
		return
	}

	err = c.authenticateCredentials(ctx)
	if err != nil {
		return
	}

	c.logger.Logf("Successfully connected to SurrealDB at %v:%v to database %v", c.config.Host, c.config.Port, c.config.Database)
}

// buildEndpoint constructs the SurrealDB endpoint based on the configuration.
func (c *Client) buildEndpoint() string {
	scheme := schemeWS
	if c.config.TLSEnabled {
		scheme = schemeHTTPS
	}

	return fmt.Sprintf("%s://%s:%d", scheme, c.config.Host, c.config.Port)
}

// connectToDatabase handles the connection to SurrealDB and returns an error if failed.
func (c *Client) connectToDatabase(ctx context.Context, endpoint string) error {
	c.logger.Debugf("connecting to SurrealDB at %s", endpoint)

	var err error

	c.db, err = newDB(ctx, endpoint)
	if err != nil {
		c.logError("failed to connect to SurrealDB", err)
		return err
	}

	if c.db == nil {
		c.logError("failed to connect to SurrealDB: no valid database instance", nil)
		return errNoDatabaseInstance
	}

	return nil
}

// setupNamespaceAndDatabase sets the namespace and database for SurrealDB.
func (c *Client) setupNamespaceAndDatabase(ctx context.Context) error {
	err := c.db.Use(ctx, c.config.Namespace, c.config.Database)
	if err != nil {
		c.logError("unable to set the namespace and database for SurrealDB", err)
		return err
	}

	return nil
}

// signIn is a helper method for signing in a user using v1.0.0 API.
func (c *Client) signIn(ctx context.Context, authData *surrealdb.Auth) (string, error) {
	token, err := c.db.SignIn(ctx, authData)
	if err != nil {
		return "", err
	}

	return token, nil
}

// authenticateCredentials handles the authentication process if credentials are provided.
func (c *Client) authenticateCredentials(ctx context.Context) error {
	if c.config.Username == "" && c.config.Password == "" {
		return nil
	}

	if c.config.Username == "" || c.config.Password == "" {
		return errInvalidCredentialsConfig
	}

	_, err := c.signIn(ctx, &surrealdb.Auth{
		Username: c.config.Username,
		Password: c.config.Password,
	})
	if err != nil {
		c.logError("failed to sign in to SurrealDB", err)
		return err
	}

	return nil
}

// logError is a helper function to log errors.
func (c *Client) logError(message string, err error) {
	if c.logger == nil {
		return
	}

	if err != nil {
		c.logger.Errorf("%s: %v", message, err)
		return
	}

	c.logger.Errorf("%s", message)
}

// Query executes a query on the SurrealDB instance.
func (c *Client) Query(ctx context.Context, query string, vars map[string]any) ([]any, error) {
	const unknown = "unknown"

	table := unknown
	id := unknown

	if vars != nil {
		if idVal, ok := vars["id"]; ok {
			id = fmt.Sprintf("%v", idVal)
		}

		if strings.Contains(query, "type::thing") {
			parts := strings.Split(query, "'")
			if len(parts) > 1 {
				table = parts[1]
			}
		}
	}

	logMessage := fmt.Sprintf("Fetching record with ID %q from table %q", id, table)

	span := c.addTrace(ctx, "Query", query)

	if c.db == nil {
		return nil, errNotConnected
	}

	startTime := time.Now()
	defer c.sendOperationStats(&QueryLog{
		Query:         logMessage,
		OperationName: "query",
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Data:          vars,
		Span:          span,
	}, startTime)

	// Use the new v1.0.0 Query function with the wrapped DB
	dbWrapper := c.db.(*DBWrapper)

	results, err := surrealdb.Query[any](ctx, dbWrapper.GetDB(), query, vars)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	if results == nil || len(*results) == 0 {
		return nil, errNoResult
	}

	return c.processQueryResults(query, *results)
}

// processQueryResults processes query results from v1.0.0 API format.
func (c *Client) processQueryResults(query string, results []surrealdb.QueryResult[any]) ([]any, error) {
	var resp []any

	for _, result := range results {
		if result.Error != nil {
			c.logger.Errorf("query error: %v", result.Error.Message)

			if !isAdministrativeOperation(query) {
				return nil, fmt.Errorf("%w: %s", errQueryError, result.Error.Message)
			}

			continue
		}

		if result.Result == nil {
			continue
		}

		if isCustomNil(result.Result) {
			resp = append(resp, true)
			continue
		}

		c.handleResultRecord(result.Result, &resp)
	}

	return resp, nil
}

// handleResultRecord processes a single result record and appends to response.
func (c *Client) handleResultRecord(result any, resp *[]any) {
	switch res := result.(type) {
	case []any:
		for _, record := range res {
			extracted, err := c.extractRecord(record)
			if err != nil {
				c.logger.Errorf("failed to extract record: %v", err)
				continue
			}

			*resp = append(*resp, extracted)
		}
	case map[string]any:
		// Handle single record returned as map directly (e.g., from type::thing() queries)
		extracted, err := c.extractRecord(res)
		if err != nil {
			c.logger.Errorf("failed to extract record: %v", err)
		} else {
			*resp = append(*resp, extracted)
		}
	case map[any]any:
		// Handle single record as map[any]any for compatibility
		extracted, err := c.extractRecord(res)
		if err != nil {
			c.logger.Errorf("failed to extract record: %v", err)
		} else {
			*resp = append(*resp, extracted)
		}
	default:
		*resp = append(*resp, result)
	}
}

// extractRecord extracts and processes a single record into a map[string]any}.
func (c *Client) extractRecord(record any) (map[string]any, error) {
	// Handle map[string]any first (SurrealDB v1.0.0 format)
	if recordMap, ok := record.(map[string]any); ok {
		extracted := make(map[string]any, len(recordMap))

		for k, v := range recordMap {
			val := c.convertValue(v)
			extracted[k] = val
		}

		return extracted, nil
	}

	// Fall back to map[any]any for compatibility
	if recordMap, ok := record.(map[any]any); ok {
		extracted := make(map[string]any, len(recordMap))

		for k, v := range recordMap {
			keyStr, ok := k.(string)
			if !ok {
				c.logger.Errorf("non-string key encountered: %v", k)
				continue
			}

			val := c.convertValue(v)
			extracted[keyStr] = val
		}

		return extracted, nil
	}

	return nil, errUnexpectedResult
}

// convertValue handles the conversion of different numeric types and strings to appropriate Go types.
func (*Client) convertValue(v any) any {
	switch val := v.(type) {
	case float64:
		if val > math.MaxInt || val < math.MinInt {
			return nil
		}

		return int(val)
	case uint64:
		if val > math.MaxInt {
			return nil
		}

		return int(val)
	case int64:
		if val > math.MaxInt || val < math.MinInt {
			return nil
		}

		return int(val)
	case string:
		return val
	default:
		return val
	}
}

// executeQuery is a helper function that encapsulates common query execution logic.
func (c *Client) executeQuery(ctx context.Context, operation, entity, query string) error {
	span := c.addTrace(ctx, operation, query)

	if c.db == nil {
		return errNotConnected
	}

	logMessage := fmt.Sprintf("%s %q", operation, entity)

	startTime := time.Now()
	defer c.sendOperationStats(&QueryLog{
		Query:         logMessage,
		OperationName: strings.ToLower(operation),
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Span:          span,
	}, startTime)

	_, err := c.Query(ctx, query, nil)

	return err
}

// CreateNamespace creates a new namespace in the SurrealDB instance.
func (c *Client) CreateNamespace(ctx context.Context, namespace string) error {
	query := fmt.Sprintf("DEFINE NAMESPACE %s;", namespace)
	return c.executeQuery(ctx, "Creating", namespace, query)
}

// CreateDatabase creates a new database in the SurrealDB instance.
func (c *Client) CreateDatabase(ctx context.Context, database string) error {
	query := fmt.Sprintf("DEFINE DATABASE %s;", database)
	return c.executeQuery(ctx, "Creating", database, query)
}

// DropNamespace deletes a namespace from the SurrealDB instance.
func (c *Client) DropNamespace(ctx context.Context, namespace string) error {
	query := fmt.Sprintf("REMOVE NAMESPACE %s;", namespace)
	return c.executeQuery(ctx, "Dropping", namespace, query)
}

// DropDatabase deletes a database from the SurrealDB instance.
func (c *Client) DropDatabase(ctx context.Context, database string) error {
	query := fmt.Sprintf("REMOVE DATABASE %s;", database)
	return c.executeQuery(ctx, "Dropping", database, query)
}

// Select retrieves all records from the specified table in the SurrealDB database.
func (c *Client) Select(ctx context.Context, table string) ([]map[string]any, error) {
	query := fmt.Sprintf("SELECT * FROM %s", table)
	span := c.addTrace(ctx, "Select", query)

	if c.db == nil {
		return nil, errNotConnected
	}

	logMessage := fmt.Sprintf("Fetching all records from table %q", table)

	startTime := time.Now()
	defer c.sendOperationStats(&QueryLog{
		Query:         logMessage,
		OperationName: "select",
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Collection:    table,
		Span:          span,
	}, startTime)

	// Use the new v1.0.0 Select function with the wrapped DB
	dbWrapper := c.db.(*DBWrapper)

	results, err := surrealdb.Select[[]map[string]any](ctx, dbWrapper.GetDB(), models.Table(table))
	if err != nil {
		return nil, fmt.Errorf("select operation failed: %w", err)
	}

	if results == nil {
		return []map[string]any{}, nil
	}

	return *results, nil
}

// Create creates a new record into the specified table in the database.
func (c *Client) Create(ctx context.Context, table string, data any) (map[string]any, error) {
	query := fmt.Sprintf("CREATE INTO %s SET", table)
	span := c.addTrace(ctx, "Create", query)

	if c.db == nil {
		return nil, errNotConnected
	}

	logMessage := fmt.Sprintf("Creating new record in table %q", table)

	startTime := time.Now()
	defer c.sendOperationStats(&QueryLog{
		Query:         logMessage,
		OperationName: "create",
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Collection:    table,
		Data:          data,
		Span:          span,
	}, startTime)

	// Use the new v1.0.0 Create function with the wrapped DB
	dbWrapper := c.db.(*DBWrapper)

	result, err := surrealdb.Create[map[string]any](ctx, dbWrapper.GetDB(), models.Table(table), data)
	if err != nil {
		return nil, fmt.Errorf("create operation failed: %w", err)
	}

	if result == nil {
		return nil, errNoRecord
	}

	return *result, nil
}

// Update modifies an existing record in the specified table using a generic MERGE update.
func (c *Client) Update(ctx context.Context, table, id string, data any) (any, error) {
	if c.db == nil {
		return nil, errNotConnected
	}

	recordID := models.RecordID{
		Table: table,
		ID:    id,
	}

	span := c.addTrace(ctx, "Update", fmt.Sprintf("%s:%s", table, id))

	logMessage := fmt.Sprintf("Updating record with ID %q in table %q", id, table)

	startTime := time.Now()
	defer c.sendOperationStats(&QueryLog{
		Query:         logMessage,
		OperationName: "update",
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Collection:    table,
		Data:          data,
		Span:          span,
	}, startTime)

	// Use the new v1.0.0 Update function with the wrapped DB
	dbWrapper := c.db.(*DBWrapper)

	result, err := surrealdb.Update[map[string]any](ctx, dbWrapper.GetDB(), recordID, data)
	if err != nil {
		return nil, fmt.Errorf("update operation failed: %w", err)
	}

	if result == nil {
		return nil, errNoRecord
	}

	return *result, nil
}

// Insert inserts a new record into the specified table in SurrealDB.
func (c *Client) Insert(ctx context.Context, table string, data any) ([]map[string]any, error) {
	query := fmt.Sprintf("INSERT INTO %s", table)
	span := c.addTrace(ctx, "Insert", query)

	if c.db == nil {
		return nil, errNotConnected
	}

	logMessage := fmt.Sprintf("Inserting record to table %q", table)

	startTime := time.Now()
	defer c.sendOperationStats(&QueryLog{
		Query:         logMessage,
		OperationName: "insert",
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Collection:    table,
		Data:          data,
		Span:          span,
	}, startTime)

	// Use the new v1.0.0 Insert function with the wrapped DB
	dbWrapper := c.db.(*DBWrapper)

	results, err := surrealdb.Insert[map[string]any](ctx, dbWrapper.GetDB(), models.Table(table), data)
	if err != nil {
		return nil, fmt.Errorf("insert operation failed: %w", err)
	}

	if results == nil {
		return []map[string]any{}, nil
	}

	return *results, nil
}

// Delete removes a record from the specified table in SurrealDB.
func (c *Client) Delete(ctx context.Context, table, id string) (any, error) {
	query := fmt.Sprintf("DELETE FROM %s:%s RETURN BEFORE;", table, id)
	span := c.addTrace(ctx, "Delete", query)

	if c.db == nil {
		return nil, errNotConnected
	}

	logMessage := fmt.Sprintf("Deleting record with ID %q in table %q", id, table)

	startTime := time.Now()
	defer c.sendOperationStats(&QueryLog{
		Query:         logMessage,
		OperationName: "delete",
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Collection:    table,
		ID:            id,
		Span:          span,
	}, startTime)

	// Use the new v1.0.0 Delete function
	recordID := models.RecordID{
		Table: table,
		ID:    id,
	}

	// Use the new v1.0.0 Delete function with the wrapped DB
	dbWrapper := c.db.(*DBWrapper)

	result, err := surrealdb.Delete[map[string]any](ctx, dbWrapper.GetDB(), recordID)
	if err != nil {
		return nil, fmt.Errorf("delete operation failed: %w", err)
	}

	if result == nil {
		return nil, nil
	}

	return *result, nil
}

// addTrace starts a new trace span for the specified method and query.
func (c *Client) addTrace(ctx context.Context, method, query string) trace.Span {
	if c.tracer == nil {
		return nil
	}

	_, span := c.tracer.Start(ctx, fmt.Sprintf("SurrealDB.%v", method))
	span.SetAttributes(
		attribute.String("surrealdb.query", query),
		attribute.String("surrealdb.namespace", c.config.Namespace),
		attribute.String("surrealdb.database", c.config.Database),
	)

	return span
}

func (c *Client) sendOperationStats(ql *QueryLog, startTime time.Time) {
	duration := time.Since(startTime).Microseconds()
	ql.Duration = duration
	c.logger.Debug(ql)
	ql.Namespace = c.config.Namespace
	ql.Database = c.config.Database

	c.metrics.RecordHistogram(context.Background(), "app_surrealdb_stats", float64(duration),
		"namespace", ql.Namespace,
		"database", ql.Database,
		"operation", ql.OperationName)

	var nbConnection float64
	if c.db != nil {
		nbConnection = 1
	}

	c.metrics.SetGauge("app_surrealdb_open_connections", nbConnection)

	if ql.Span == nil {
		return
	}

	defer ql.Span.End()

	ql.Span.SetAttributes(
		attribute.Int64("surrealdb.duration", duration),
		attribute.String("surrealdb.query", ql.Query),
		attribute.String("surrealdb.operation", ql.OperationName),
		attribute.String("surrealdb.namespace", ql.Namespace),
		attribute.String("surrealdb.database", ql.Database),
		attribute.String("surrealdb.collection", ql.Collection),
	)
}

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	const (
		statusDown = "DOWN"
		statusUP   = "UP"
	)

	logMessage := fmt.Sprintf("Database health at \"%s:%d\"", c.config.Host, c.config.Port)

	span := c.addTrace(ctx, "HealthCheck", "info")

	startTime := time.Now()
	defer c.sendOperationStats(&QueryLog{
		Query:         logMessage,
		OperationName: "health_check",
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Span:          span,
	}, startTime)

	h := Health{
		Details: make(map[string]any),
	}

	h.Details["host"] = fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)
	h.Details["namespace"] = c.config.Namespace
	h.Details["database"] = c.config.Database

	if c.db == nil {
		h.Status = statusDown
		h.Details["error"] = "Database client is not connected"

		return &h, errNotConnected
	}

	// Use the new v1.0.0 Info method
	if _, err := c.db.Info(ctx); err != nil {
		h.Status = statusDown
		h.Details["error"] = fmt.Sprintf("Connection test failed: %v", err)
		h.Details["connection_state"] = "failed"

		return &h, err
	}

	if err := c.db.Use(ctx, c.config.Namespace, c.config.Database); err != nil {
		h.Status = statusDown
		h.Details["error"] = fmt.Sprintf("Database access verification failed: %v", err)
		h.Details["connection_state"] = "connected_but_access_failed"

		return &h, err
	}

	h.Status = statusUP
	h.Details["message"] = "Database is healthy"
	h.Details["connection_state"] = "fully_connected"

	return &h, nil
}
