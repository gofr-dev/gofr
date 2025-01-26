package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	errNotConnected             = errors.New("not connected to database")
	errNoDatabaseInstance       = errors.New("failed to connect to SurrealDB: no valid database instance")
	errInvalidCredentialsConfig = errors.New("both username and password must be provided")
	errEmbeddedDBNotEnabled     = errors.New("embedded database not enabled")
	errInvalidConnectionURL     = errors.New("invalid connection URL")
	errNoRecord                 = errors.New("no record found")
	errUnexpectedResultType     = errors.New("unexpected result type")
	errNoResult                 = errors.New("no result found in query response")
	errInvalidResult            = errors.New("unexpected result format: expected []any")
	errUnexpectedResult         = errors.New("unexpected result type: expected []any")
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
	db      Connection
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

// newDB creates a new SurrealDB client.
func newDB(connectionURL string) (con connection.Connection, err error) {
	u, err := url.ParseRequestURI(connectionURL)
	if err != nil {
		return nil, err
	}

	scheme := u.Scheme

	newParams := connection.NewConnectionParams{
		Marshaler:   models.CborMarshaler{},
		Unmarshaler: models.CborUnmarshaler{},
		BaseURL:     fmt.Sprintf("%s://%s", u.Scheme, u.Host),
		Logger:      logger.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	switch scheme {
	case schemeHTTP, schemeHTTPS:
		con = connection.NewHTTPConnection(newParams)
	case schemeWS, schemeWSS:
		con = connection.NewWebSocketConnection(newParams)
	case schemeMemory, schemeMem, schemeSurrealkv:
		return nil, fmt.Errorf("%w", errEmbeddedDBNotEnabled)
	default:
		return nil, fmt.Errorf("%w", errInvalidConnectionURL)
	}

	err = con.Connect()
	if err != nil {
		return nil, err
	}

	return con, nil
}

// Connect establishes a connection to the SurrealDB database.
func (c *Client) Connect() {
	c.logger.Debugf("connecting to SurrealDB at %v:%v to database %v", c.config.Host, c.config.Port, c.config.Database)

	endpoint := c.buildEndpoint()
	err := c.connectToDatabase(endpoint)

	if err != nil {
		return
	}

	err = c.setupNamespaceAndDatabase()
	if err != nil {
		return
	}

	err = c.authenticateCredentials()
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
func (c *Client) connectToDatabase(endpoint string) error {
	c.logger.Debugf("connecting to SurrealDB at %s", endpoint)

	var err error
	c.db, err = newDB(endpoint)

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
func (c *Client) setupNamespaceAndDatabase() error {
	err := c.db.Use(c.config.Namespace, c.config.Database)
	if err != nil {
		c.logError("unable to set the namespace and database for SurrealDB", err)
		return err
	}

	return nil
}

// SignIn is a helper method for signing in a user.
func (c *Client) signIn(authData *surrealdb.Auth) (string, error) {
	var token connection.RPCResponse[string]
	if err := c.db.Send(&token, "signin", authData); err != nil {
		return "", err
	}

	if err := c.db.Let(constants.AuthTokenKey, token.Result); err != nil {
		return "", err
	}

	return *token.Result, nil
}

// authenticate handles the authentication process if credentials are provided.
func (c *Client) authenticateCredentials() error {
	if c.config.Username == "" && c.config.Password == "" {
		return nil
	}

	if c.config.Username == "" || c.config.Password == "" {
		return errInvalidCredentialsConfig
	}

	_, err := c.signIn(&surrealdb.Auth{
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

// useNamespace switches the active namespace for the database connection.
func (c *Client) useNamespace(ns string) error {
	if c.db == nil {
		return errNotConnected
	}

	return c.db.Use(ns, "")
}

// useDatabase switches the active database for the connection.
func (c *Client) useDatabase(db string) error {
	if c.db == nil {
		return errNotConnected
	}

	return c.db.Use("", db)
}

type DBResponse struct {
	ID     any                  `json:"id" msgpack:"id"`
	Error  *connection.RPCError `json:"error,omitempty" msgpack:"error,omitempty"`
	Result any
}

// QueryResponse defines the structure of the query response.
type QueryResponse struct {
	ID     any                  `json:"id" msgpack:"id"`
	Error  *connection.RPCError `json:"error,omitempty" msgpack:"error,omitempty"`
	Result *[]QueryResult       `json:"result,omitempty" msgpack:"result,omitempty"`
}

// QueryResult represents each query result from SurrealDB.
type QueryResult struct {
	Status string `json:"status"`
	Time   string `json:"time"`
	Result any    `json:"result"`
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
	defer func() {
		if span != nil {
			span.End()
		}
	}()

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

	var res QueryResponse
	if err := c.db.Send(&res, "query", query, vars); err != nil {
		return nil, errNoResult
	}

	if res.Result == nil {
		return nil, errNoResult
	}

	return c.processResults(res.Result)
}

// processResults processes and extracts meaningful data from query results.
func (c *Client) processResults(results *[]QueryResult) ([]any, error) {
	var resp []any

	if len(*results) > 0 {
		resp = make([]any, 0, len(*results))
	}

	for _, r := range *results {
		if r.Status != statusOK {
			c.logger.Errorf("query result error: %v", r.Status)
			continue
		}

		recordList, ok := r.Result.([]any)
		if !ok {
			return nil, errInvalidResult
		}

		for _, record := range recordList {
			extracted, err := c.extractRecord(record)
			if err != nil {
				return nil, fmt.Errorf("failed to extract record: %w", err)
			}

			resp = append(resp, extracted)
		}
	}

	return resp, nil
}

// extractRecord extracts and processes a single record into a map[string]any}.
func (c *Client) extractRecord(record any) (map[string]any, error) {
	recordMap, ok := record.(map[any]any)
	if !ok {
		return nil, errUnexpectedResult
	}

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

// Select retrieves all records from the specified table in the SurrealDB database.
func (c *Client) Select(ctx context.Context, table string) ([]map[string]any, error) {
	query := fmt.Sprintf("SELECT * FROM %s", table)
	span := c.addTrace(ctx, "Select", query)

	defer func() {
		if span != nil {
			span.End()
		}
	}()

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

	var res DBResponse
	if err := c.db.Send(&res, "select", table); err != nil {
		return nil, fmt.Errorf("select operation failed: %w", err)
	}

	return c.processSelectResults(res.Result)
}

// processSelectResults handles the conversion of raw database results into a structured format.
func (c *Client) processSelectResults(result any) ([]map[string]any, error) {
	records, ok := result.([]any)
	if !ok {
		return nil, errUnexpectedResult
	}

	processed := make([]map[string]any, 0, len(records))

	for _, record := range records {
		extracted, err := c.extractRecord(record)
		if err != nil {
			return nil, fmt.Errorf("failed to process record: %w", err)
		}

		processed = append(processed, extracted)
	}

	return processed, nil
}

// Create creates a new record into the specified table in the database.
func (c *Client) Create(ctx context.Context, table string, data any) (map[string]any, error) {
	query := fmt.Sprintf("CREATE INTO %s SET", table)
	span := c.addTrace(ctx, "Create", query)

	defer func() {
		if span != nil {
			span.End()
		}
	}()

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

	var res DBResponse
	if err := c.db.Send(&res, "create", table, data); err != nil {
		return nil, fmt.Errorf("create operation failed: %w", err)
	}

	return c.extractRecord(res.Result)
}

// Update modifies an existing record in the specified table.
func (c *Client) Update(ctx context.Context, table, id string, data any) (any, error) {
	if c.db == nil {
		return nil, errNotConnected
	}

	query := fmt.Sprintf("UPDATE %s SET", table)
	span := c.addTrace(ctx, "Update", query)

	if span != nil {
		defer span.End()
	}

	logMessage := fmt.Sprintf("Updating record with ID %q in table %q", id, table)

	startTime := time.Now()
	defer c.sendOperationStats(&QueryLog{
		Query:         logMessage,
		OperationName: "update",
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Collection:    table,
		Data:          data,
		Update:        data,
		Span:          span,
	}, startTime)

	dataMap := data.(map[string]any)

	var updateResult DBResponse

	updateQuery := fmt.Sprintf(`
        UPDATE %s:%s SET 
        name = $name, 
        age = $age, 
        email = $email
        RETURN *`, table, id)

	if err := c.db.Send(&updateResult, "query", updateQuery, map[string]any{
		"name":  dataMap["name"],
		"age":   dataMap["age"],
		"email": dataMap["email"],
	}); err != nil {
		return nil, err
	}

	resultSlice, ok := updateResult.Result.([]any)
	if !ok || len(resultSlice) == 0 {
		return nil, errNoRecord
	}

	return resultSlice[0], nil
}

// Insert inserts a new record into the specified table in SurrealDB.
func (c *Client) Insert(ctx context.Context, table string, data any) ([]map[string]any, error) {
	query := fmt.Sprintf("INSERT INTO %s", table)
	span := c.addTrace(ctx, "Insert", query)

	if span != nil {
		defer span.End()
	}

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

	var res DBResponse
	if err := c.db.Send(&res, "insert", table, data); err != nil {
		return nil, err
	}

	result, ok := res.Result.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: %T", errUnexpectedResultType, res.Result)
	}

	resSlice := make([]map[string]any, 0)
	for _, record := range result {
		resSlice = append(resSlice, record.(map[string]any))
	}

	return resSlice, nil
}

// Delete removes a record from the specified table in SurrealDB.
func (c *Client) Delete(ctx context.Context, table, id string) (any, error) {
	query := fmt.Sprintf("DELETE FROM %s:%s RETURN BEFORE;", table, id)
	span := c.addTrace(ctx, "Delete", query)

	defer func() {
		if span != nil {
			span.End()
		}
	}()

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

	var res DBResponse
	if err := c.db.Send(&res, "query", query, nil); err != nil {
		return nil, fmt.Errorf("delete operation failed: %w", err)
	}

	results, ok := res.Result.([]any)
	if !ok || len(results) == 0 {
		return nil, nil
	}

	return c.extractRecord(results[0])
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

	if ql.Span == nil {
		return
	}

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
	defer func() {
		if span != nil {
			span.End()
		}
	}()

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

	var res DBResponse
	if err := c.db.Send(&res, "info"); err != nil {
		h.Status = statusDown
		h.Details["error"] = fmt.Sprintf("Connection test failed: %v", err)
		h.Details["connection_state"] = "failed"

		return &h, err
	}

	if err := c.db.Use(c.config.Namespace, c.config.Database); err != nil {
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
