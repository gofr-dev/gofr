package surrealdb

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"github.com/surrealdb/surrealdb.go"
	"github.com/surrealdb/surrealdb.go/pkg/connection"
	"github.com/surrealdb/surrealdb.go/pkg/constants"
	"github.com/surrealdb/surrealdb.go/pkg/logger"
	"github.com/surrealdb/surrealdb.go/pkg/models"

	"go.opentelemetry.io/otel/trace"
)

var (
	// ErrNotConnected indicates that the database client is not connected.
	ErrNotConnected = errors.New("not connected to database")
)

const (
	https = "https"
	ws    = "ws"
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
func (c *Client) UseLogger(customlogger interface{}) {
	if l, ok := customlogger.(Logger); ok {
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
var (
	ErrNoDatabaseInstance       = errors.New("failed to connect to SurrealDB: no valid database instance")
	ErrInvalidCredentialsConfig = errors.New("both username and password must be provided")
	ErrEmbeddedDBNotEnabled     = errors.New("embedded database not enabled")
	ErrInvalidConnectionURL     = errors.New("invalid connection URL")
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

	if scheme == "http" || scheme == https {
		con = connection.NewHTTPConnection(newParams)
	} else if scheme == ws || scheme == "wss" {
		con = connection.NewWebSocketConnection(newParams)
	} else if scheme == "memory" || scheme == "mem" || scheme == "surrealkv" {
		return nil, fmt.Errorf("%w", ErrEmbeddedDBNotEnabled)
	} else {
		return nil, fmt.Errorf("%w", ErrInvalidConnectionURL)
	}

	err = con.Connect()
	if err != nil {
		return nil, err
	}

	return con, nil
}

// Connect establishes a connection to the SurrealDB database.
func (c *Client) Connect() error {
	endpoint := c.buildEndpoint()
	if err := c.connectToDatabase(endpoint); err != nil {
		return err
	}

	if err := c.setupNamespaceAndDatabase(); err != nil {
		return err
	}

	if err := c.authenticate(); err != nil {
		return err
	}

	c.logger.Debugf("successfully connected to SurrealDB")

	return nil
}

// buildEndpoint constructs the SurrealDB endpoint based on the configuration.
func (c *Client) buildEndpoint() string {
	scheme := ws
	if c.config.TLSEnabled {
		scheme = https
	}

	return fmt.Sprintf("%s://%s:%d", scheme, c.config.Host, c.config.Port)
}

// connectToDatabase handles the connection to SurrealDB and returns an error if failed.
func (c *Client) connectToDatabase(endpoint string) error {
	if c.logger != nil {
		c.logger.Debugf("connecting to SurrealDB at %s", endpoint)
	}

	db, err := NewDB(endpoint)
	if err != nil {
		c.logError("failed to connect to SurrealDB", err)
		return err
	}

	if db == nil {
		c.logError("failed to connect to SurrealDB: no valid database instance", nil)
		return ErrNoDatabaseInstance
	}

	c.db = db

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

// Use is a method to select the namespace and table to use.
func (c *Client) Use(ns, database string) error {
	return c.db.Use(ns, database)
}

func (c *Client) Info() (map[string]interface{}, error) {
	var info connection.RPCResponse[map[string]interface{}]
	err := c.db.Send(&info, "info")

	return *info.Result, err
}

// SignUp is a helper method for signing up a new user.
func (c *Client) SignUp(authData *surrealdb.Auth) (string, error) {
	var token connection.RPCResponse[string]
	if err := c.db.Send(&token, "signup", authData); err != nil {
		return "", err
	}

	if err := c.db.Let(constants.AuthTokenKey, token.Result); err != nil {
		return "", err
	}

	return *token.Result, nil
}

// SignIn is a helper method for signing in a user.
func (c *Client) SignIn(authData *surrealdb.Auth) (string, error) {
	var token connection.RPCResponse[string]
	if err := c.db.Send(&token, "signin", authData); err != nil {
		return "", err
	}

	if err := c.db.Let(constants.AuthTokenKey, token.Result); err != nil {
		return "", err
	}

	return *token.Result, nil
}

// Invalidate clears the current authentication token from the database.
func (c *Client) Invalidate() error {
	if err := c.db.Send(nil, "invalidate"); err != nil {
		return err
	}

	if err := c.db.Unset(constants.AuthTokenKey); err != nil {
		return err
	}

	return nil
}

// Authenticate sends the provided authentication token to the database.
func (c *Client) Authenticate(token string) error {
	if err := c.db.Send(nil, "authenticate", token); err != nil {
		return err
	}

	if err := c.db.Let(constants.AuthTokenKey, token); err != nil {
		return err
	}

	return nil
}

// authenticate handles the authentication process if credentials are provided.
func (c *Client) authenticate() error {
	if (c.config.Username == "" && c.config.Password != "") || (c.config.Username != "" && c.config.Password == "") {
		return ErrInvalidCredentialsConfig
	}

	if c.config.Username != "" && c.config.Password != "" {
		_, err := c.SignIn(&surrealdb.Auth{
			Username: c.config.Username,
			Password: c.config.Password,
		})
		if err != nil {
			c.logError("failed to sign in to SurrealDB", err)
			return err
		}
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

type RPCResponse struct {
	ID     interface{}          `json:"id" msgpack:"id"`
	Error  *connection.RPCError `json:"error,omitempty" msgpack:"error,omitempty"`
	Result *[]QueryResult       `json:"result,omitempty" msgpack:"result,omitempty"`
}

type QueryResult struct {
	Status string `json:"status"`
	Time   string `json:"time"`
	Result any    `json:"result"`
}

// Query executes a query on the SurrealDB instance.
func (c *Client) Query(ctx context.Context, query string, vars map[string]interface{}) ([]interface{}, error) {
	if c.db == nil {
		return nil, ErrNotConnected
	}

	var res RPCResponse

	if err := c.db.Send(&res, "query", query, vars); err != nil {
		return nil, err
	}

	result := res.Result

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

type Response struct {
	ID     interface{}          `json:"id" msgpack:"id"`
	Error  *connection.RPCError `json:"error,omitempty" msgpack:"error,omitempty"`
	Result any                  `json:"result,omitempty" msgpack:"result,omitempty"`
}

// Select retrieves all records from a specific table in SurrealDB.
var (
	ErrNonStringKey = errors.New("non-string key encountered")
)

func (c *Client) Select(ctx context.Context, table string) ([]map[string]interface{}, error) {
	if c.db == nil {
		return nil, ErrNotConnected
	}
	//
	var res Response
	if err := c.db.Send(&res, "select", table); err != nil {
		return nil, err
	}

	res1 := res.Result
	result, ok := res1.([]interface{})

	if !ok {
		return nil, fmt.Errorf("%w", ErrNonStringKey)
	}

	resSlice := make([]map[string]interface{}, 0)

	for _, record := range result {
		recordMap := record.(map[interface{}]interface{})

		resMap := make(map[string]interface{})

		for k, v := range recordMap {
			keyStr, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("%w: %v", ErrNonStringKey, k)
			}

			resMap[keyStr] = v
		}

		resSlice = append(resSlice, resMap)
	}

	if c.metrics != nil {
		c.metrics.RecordHistogram(ctx, "surreal_db_operation_duration", 0, "operation", "query")
	}

	return resSlice, nil
}

// Create inserts a new record into the specified table in SurrealDB.
var ErrUnexpectedResultType = errors.New("unexpected result type")

func (c *Client) Create(ctx context.Context, table string, data interface{}) (map[string]interface{}, error) {
	if c.db == nil {
		return nil, ErrNotConnected
	}

	var CreateResult Response
	if err := c.db.Send(&CreateResult, "create", table, data); err != nil {
		return nil, err
	}

	if c.metrics != nil {
		c.metrics.RecordHistogram(ctx, "surreal_db_operation_duration", 0, "operation", "create")
	}

	result, ok := CreateResult.Result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: %v", ErrUnexpectedResultType, CreateResult.Result)
	}

	return result, nil
}

// Update modifies an existing record in the specified table.
func (c *Client) Update(ctx context.Context, table, _ string, data interface{}) (interface{}, error) {
	if c.db == nil {
		return nil, ErrNotConnected
	}

	var UpdateResult Response

	if err := c.db.Send(&UpdateResult, "update", table, data); err != nil {
		return nil, err
	}

	resultSlice := (UpdateResult.Result).([]interface{})

	resMap := make(map[string]interface{})

	for _, r := range resultSlice {
		rMap := r.(map[interface{}]interface{})
		for k, v := range rMap {
			kStr := k.(string)

			resMap[kStr] = v
		}
	}

	if c.metrics != nil {
		c.metrics.RecordHistogram(ctx, "surreal_db_operation_duration", 0, "operation", "query")
	}

	return resMap, nil
}

// Insert inserts a new record into the specified table in SurrealDB.
func (c *Client) Insert(ctx context.Context, table string, data interface{}) (*Response, error) {
	if c.db == nil {
		return nil, ErrNotConnected
	}

	var insertResult Response
	if err := c.db.Send(&insertResult, "insert", table, data); err != nil {
		return nil, err
	}

	if c.metrics != nil {
		c.metrics.RecordHistogram(ctx, "surreal_db_operation_duration", 0, "operation", "insert")
	}

	return &insertResult, nil
}

// Delete removes a record from the specified table in SurrealDB.
func (c *Client) Delete(ctx context.Context, table, id string) (any, error) {
	if c.db == nil {
		return nil, ErrNotConnected
	}

	var DeleteResult Response

	arg := models.RecordID{
		Table: table,
		ID:    id,
	}

	if err := c.db.Send(&DeleteResult, "delete", arg); err != nil {
		return nil, err
	}

	if c.metrics != nil {
		c.metrics.RecordHistogram(ctx, "surreal_db_operation_duration", 0, "operation", "query")
	}

	return DeleteResult.Result, nil
}

type Health struct {
	Status  string                 `json:"status,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthCheck performs a health check on the SurrealDB connection.
var ErrUnexpectedHealthCheckResult = errors.New("unexpected result from health check query")

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

	result, err := c.Query(ctx, query, nil)
	if err != nil {
		h.Status = statusDown
		h.Details["error"] = fmt.Sprintf("Failed to execute health check query: %v", err)

		return &h, err
	}

	if len(result) == 0 || result[0] != "SurrealDB Health Check" {
		h.Status = statusDown
		h.Details["error"] = ErrUnexpectedHealthCheckResult.Error()

		return &h, fmt.Errorf("%w", ErrUnexpectedHealthCheckResult)
	}

	h.Status = statusUP

	return &h, nil
}
