package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
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
	// errNotConnected indicates that the database client is not connected.
	errNotConnected = errors.New("not connected to database")
)

const (
	schemeHTTPS     = "https"
	schemeWS        = "ws"
	schemeHTTP      = "http"
	schemeWss       = "wss"
	schemeMemory    = "memory"
	schemeMem       = "mem"
	schemeSurrealkv = "surrealkv"
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

// Connect establishes a connection to the SurrealDB server using the client's configuration.
var (
	errNoDatabaseInstance       = errors.New("failed to connect to SurrealDB: no valid database instance")
	errInvalidCredentialsConfig = errors.New("both username and password must be provided")
	errEmbeddedDBNotEnabled     = errors.New("embedded database not enabled")
	errInvalidConnectionURL     = errors.New("invalid connection URL")
)

// NewDB creates a new SurrealDB client.
func NewDB(connectionURL string) (con connection.Connection, err error) {
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
	case schemeWS, schemeWss:
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

	c.logger.Debugf("successfully connected to SurrealDB")
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
	c.db, err = NewDB(endpoint)

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
	if c.logger != nil {
		if err != nil {
			c.logger.Errorf("%s: %v", message, err)
		} else {
			c.logger.Errorf("%s", message)
		}
	}
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

type QueryResponse struct {
	ID     any                  `json:"id" msgpack:"id"`
	Error  *connection.RPCError `json:"error,omitempty" msgpack:"error,omitempty"`
	Result *[]QueryResult       `json:"result,omitempty" msgpack:"result,omitempty"`
}

type QueryResult struct {
	Status string `json:"status"`
	Time   string `json:"time"`
	Result any    `json:"result"`
}

// Query executes a query on the SurrealDB instance.
// Query executes a query on the SurrealDB instance.
func (c *Client) Query(ctx context.Context, query string, vars map[string]any) ([]any, error) {
	span := c.addTrace(ctx, "Query", query)
	if span != nil {
		defer span.End()
	}

	if c.db == nil {
		return nil, errNotConnected
	}

	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{
		Query:         query,
		OperationName: "query",
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Data:          vars,
		Span:          span,
	}, startTime)

	var res QueryResponse

	if err := c.db.Send(&res, "query", query, vars); err != nil {
		return nil, err
	}

	result := res.Result

	resp := make([]any, 0)

	for _, r := range *result {
		if r.Status != "OK" {
			c.logger.Errorf("query result error: %v", r.Status)
			continue
		}

		if res, ok := r.Result.([]any); ok {
			resp = append(resp, res...)
		} else {
			c.logger.Errorf("unexpected result type: %v", r.Result)
		}
	}

	return resp, nil
}

type Response struct {
	ID     any                  `json:"id" msgpack:"id"`
	Error  *connection.RPCError `json:"error,omitempty" msgpack:"error,omitempty"`
	Result any                  `json:"result,omitempty" msgpack:"result,omitempty"`
}

var (
	errNonStringKey = errors.New("non-string key encountered")
)

// Select queries the specified table in the database and retrieves all records.
func (c *Client) Select(ctx context.Context, table string) ([]map[string]any, error) {
	query := fmt.Sprintf("SELECT * FROM %s", table)
	span := c.addTrace(ctx, "Select", query)
	if span != nil {
		defer span.End()
	}

	if c.db == nil {
		return nil, errNotConnected
	}

	startTime := time.Now()

	// Set up deferred stats collection at the start
	defer c.sendOperationStats(&QueryLog{
		Query:         query,
		OperationName: "select",
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Collection:    table,
		Span:          span,
	}, startTime)

	var res Response
	if err := c.db.Send(&res, "select", table); err != nil {
		return nil, err
	}

	result, ok := res.Result.([]any)
	if !ok {
		return nil, fmt.Errorf("%w", errNonStringKey)
	}

	resSlice := make([]map[string]any, 0)

	for _, record := range result {
		recordMap := record.(map[any]any)

		resMap := make(map[string]any)

		for k, v := range recordMap {
			keyStr, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("%w: %v", errNonStringKey, k)
			}

			resMap[keyStr] = v
		}

		resSlice = append(resSlice, resMap)
	}

	c.metrics.RecordHistogram(ctx, "surreal_db_operation_duration", 0, "operation", "select")

	return resSlice, nil
}

var errUnexpectedResultType = errors.New("unexpected result type")

// Create creates a new record into the specified table in the database.
func (c *Client) Create(ctx context.Context, table string, data any) (map[string]any, error) {
	query := fmt.Sprintf("CREATE INTO %s", table)
	span := c.addTrace(ctx, "Create", query)
	if span != nil {
		defer span.End()
	}

	if c.db == nil {
		return nil, errNotConnected
	}

	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{
		Query:         query,
		OperationName: "create",
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Collection:    table,
		Data:          data,
		Span:          span,
	}, startTime)

	var CreateResult Response
	if err := c.db.Send(&CreateResult, "create", table, data); err != nil {
		return nil, err
	}

	c.metrics.RecordHistogram(ctx, "surreal_db_operation_duration", 0, "operation", "create")

	result, ok := CreateResult.Result.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%w: %v", errUnexpectedResultType, CreateResult.Result)
	}

	return result, nil
}

// Update modifies an existing record in the specified table.
func (c *Client) Update(ctx context.Context, table, _ string, data any) (any, error) {
	query := fmt.Sprintf("UPDATE %s", table)
	span := c.addTrace(ctx, "Update", query)
	if span != nil {
		defer span.End()
	}

	if c.db == nil {
		return nil, errNotConnected
	}

	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{
		Query:         query,
		OperationName: "update",
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Collection:    table,
		Data:          data,
		Update:        data,
		Span:          span,
	}, startTime)

	var UpdateResult Response

	if err := c.db.Send(&UpdateResult, "update", table, data); err != nil {
		return nil, err
	}

	resultSlice := (UpdateResult.Result).([]any)

	resMap := make(map[string]any)

	for _, r := range resultSlice {
		rMap := r.(map[any]any)
		for k, v := range rMap {
			kStr := k.(string)
			resMap[kStr] = v
		}
	}

	c.metrics.RecordHistogram(ctx, "surreal_db_operation_duration", 0, "operation", "update")

	return resMap, nil
}

// Insert inserts a new record into the specified table in SurrealDB.
func (c *Client) Insert(ctx context.Context, table string, data any) (*Response, error) {
	query := fmt.Sprintf("INSERT INTO %s", table)
	span := c.addTrace(ctx, "Insert", query)
	if span != nil {
		defer span.End()
	}

	if c.db == nil {
		return nil, errNotConnected
	}

	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{
		Query:         query,
		OperationName: "insert",
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Collection:    table,
		Data:          data,
		Span:          span,
	}, startTime)

	var insertResult Response
	if err := c.db.Send(&insertResult, "insert", table, data); err != nil {
		return nil, err
	}

	c.metrics.RecordHistogram(ctx, "surreal_db_operation_duration", 0, "operation", "insert")

	return &insertResult, nil
}

// Delete removes a record from the specified table in SurrealDB.
func (c *Client) Delete(ctx context.Context, table, id string) (any, error) {
	query := fmt.Sprintf("DELETE FROM %s WHERE id = %s", table, id)
	span := c.addTrace(ctx, "Delete", query)
	if span != nil {
		defer span.End()
	}

	if c.db == nil {
		return nil, errNotConnected
	}

	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{
		Query:         query,
		OperationName: "delete",
		Namespace:     c.config.Namespace,
		Database:      c.config.Database,
		Collection:    table,
		ID:            id,
		Span:          span,
	}, startTime)

	var DeleteResult Response

	arg := models.RecordID{
		Table: table,
		ID:    id,
	}

	if err := c.db.Send(&DeleteResult, "delete", arg); err != nil {
		return nil, err
	}

	c.metrics.RecordHistogram(ctx, "surreal_db_operation_duration", 0, "operation", "delete")

	return DeleteResult.Result, nil
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

	if ql.Span != nil {
		ql.Span.SetAttributes(
			attribute.Int64("surrealdb.duration", duration),
			attribute.String("surrealdb.query", ql.Query),
			attribute.String("surrealdb.operation", ql.OperationName),
			attribute.String("surrealdb.namespace", ql.Namespace),
			attribute.String("surrealdb.database", ql.Database),
			attribute.String("surrealdb.collection", ql.Collection),
		)
	}
}

type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// HealthCheck performs a health check on the SurrealDB connection.
var errUnexpectedHealthCheckResult = errors.New("unexpected result from health check query")

func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	const (
		statusDown = "DOWN"
		statusUP   = "UP"
	)

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

	query := "RETURN 'SurrealDB Health Check'"

	result, err := c.Query(ctx, query, nil)
	if err != nil {
		h.Status = statusDown
		h.Details["error"] = fmt.Sprintf("Failed to execute health check query: %v", err)

		return &h, err
	}

	if len(result) == 0 || result[0] != "SurrealDB Health Check" {
		h.Status = statusDown
		h.Details["error"] = errUnexpectedHealthCheckResult.Error()

		return &h, fmt.Errorf("%w", errUnexpectedHealthCheckResult)
	}

	h.Status = statusUP

	return &h, nil
}
