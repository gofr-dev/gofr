package arangodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/arangodb/go-driver/v2/arangodb/shared"
	"github.com/arangodb/go-driver/v2/connection"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Client represents an ArangoDB client.
type Client struct {
	client   arangodb.Client
	logger   Logger
	metrics  Metrics
	tracer   trace.Tracer
	config   *Config
	endpoint string
	*DB
	*Document
	*Graph
}

// Config holds the configuration for ArangoDB connection.
type Config struct {
	Host     string
	User     string
	Password string
	Port     int
}

var (
	errStatusDown             = errors.New("status down")
	errMissingField           = errors.New("missing required field in config")
	errInvalidResultType      = errors.New("result must be a pointer to a slice of maps")
	errInvalidUserOptionsType = errors.New("userOptions must be a *UserOptions type")
)

// New creates a new ArangoDB client with the provided configuration.
func New(c Config) *Client {
	client := &Client{
		config: &c,
	}

	client.DB = &DB{client: client}
	client.Document = &Document{client: client}
	client.Graph = &Graph{client: client}

	return client
}

// UseLogger sets the logger for the ArangoDB client.
func (c *Client) UseLogger(logger any) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the ArangoDB client.
func (c *Client) UseMetrics(metrics any) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for the ArangoDB client.
func (c *Client) UseTracer(tracer any) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

// Connect establishes a connection to the ArangoDB server.
func (c *Client) Connect() {
	if err := c.validateConfig(); err != nil {
		c.logger.Errorf("config validation error: %v", err)
		return
	}

	c.endpoint = fmt.Sprintf("http://%s:%d", c.config.Host, c.config.Port)
	c.logger.Debugf("connecting to ArangoDB at %s", c.endpoint)

	endpoint := connection.NewRoundRobinEndpoints([]string{c.endpoint})
	conn := connection.NewHttpConnection(connection.HttpConfiguration{
		Endpoint: endpoint,
	})

	auth := connection.NewBasicAuth(c.config.User, c.config.Password)

	err := conn.SetAuthentication(auth)
	if err != nil {
		c.logger.Errorf("authentication setup failed: %v", err)
	}

	client := arangodb.NewClient(conn)
	c.client = client

	// Initialize metrics
	arangoBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_arango_stats", "Response time of ArangoDB operations in milliseconds.", arangoBuckets...)

	c.logger.Logf("connected to ArangoDB successfully at %s", c.endpoint)
}

func (c *Client) validateConfig() error {
	if c.config.Host == "" {
		return fmt.Errorf("%w: host is empty", errMissingField)
	}

	if c.config.Port == 0 {
		return fmt.Errorf("%w: port is empty", errMissingField)
	}

	if c.config.User == "" {
		return fmt.Errorf("%w: user is empty", errMissingField)
	}

	if c.config.Password == "" {
		return fmt.Errorf("%w: password is empty", errMissingField)
	}

	return nil
}

func (c *Client) User(ctx context.Context, username string) (arangodb.User, error) {
	return c.client.User(ctx, username)
}

func (c *Client) Database(ctx context.Context, name string) (arangodb.Database, error) {
	return c.client.Database(ctx, name)
}

func (c *Client) Databases(ctx context.Context) ([]arangodb.Database, error) {
	return c.client.Databases(ctx)
}

func (c *Client) Version(ctx context.Context) (arangodb.VersionInfo, error) {
	return c.client.Version(ctx)
}

// CreateUser creates a new user in ArangoDB.
func (c *Client) CreateUser(ctx context.Context, username string, options any) error {
	tracerCtx, span := c.addTrace(ctx, "createUser", map[string]string{"user": username})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Operation: "createUser", ID: username},
		startTime, "createUser", span)

	userOptions, ok := options.(*arangodb.UserOptions)
	if !ok {
		return fmt.Errorf("%w", errInvalidUserOptionsType)
	}

	_, err := c.client.CreateUser(tracerCtx, username, userOptions)
	if err != nil {
		return err
	}

	return nil
}

// DropUser deletes a user from ArangoDB.
func (c *Client) DropUser(ctx context.Context, username string) error {
	tracerCtx, span := c.addTrace(ctx, "dropUser", map[string]string{"user": username})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Operation: "dropUser",
		ID: username}, startTime, "dropUser", span)

	err := c.client.RemoveUser(tracerCtx, username)
	if err != nil {
		return err
	}

	return err
}

// GrantDB grants permissions for a database to a user.
func (c *Client) GrantDB(ctx context.Context, database, username, permission string) error {
	tracerCtx, span := c.addTrace(ctx, "grantDB", map[string]string{"DB": database})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Operation: "grantDB",
		Database: database, ID: username}, startTime, "grantDB", span)

	user, err := c.client.User(tracerCtx, username)
	if err != nil {
		return err
	}

	err = user.SetDatabaseAccess(tracerCtx, database, arangodb.Grant(permission))

	return err
}

// GrantCollection grants permissions for a collection to a user.
func (c *Client) GrantCollection(ctx context.Context, database, collection, username, permission string) error {
	tracerCtx, span := c.addTrace(ctx, "GrantCollection", map[string]string{"collection": collection})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Operation: "GrantCollection",
		Database: database, Collection: collection, ID: username}, startTime,
		"GrantCollection", span)

	user, err := c.client.User(tracerCtx, username)
	if err != nil {
		return err
	}

	err = user.SetCollectionAccess(tracerCtx, database, collection, arangodb.Grant(permission))

	return err
}

// Query executes an AQL query and binds the results.
func (c *Client) Query(ctx context.Context, dbName, query string, bindVars map[string]any, result any) error {
	tracerCtx, span := c.addTrace(ctx, "query", map[string]string{"DB": dbName})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Operation: "query",
		Database: dbName, Query: query}, startTime, "query", span)

	db, err := c.client.Database(tracerCtx, dbName)
	if err != nil {
		return err
	}

	cursor, err := db.Query(tracerCtx, query, &arangodb.QueryOptions{BindVars: bindVars})
	if err != nil {
		return err
	}

	defer cursor.Close()

	resultSlice, ok := result.(*[]map[string]any)
	if !ok {
		return errInvalidResultType
	}

	for {
		var doc map[string]any

		_, err = cursor.ReadDocument(tracerCtx, &doc)
		if shared.IsNoMoreDocuments(err) {
			break
		}

		if err != nil {
			return err
		}

		*resultSlice = append(*resultSlice, doc)
	}

	return err
}

// addTrace adds tracing to context if tracer is configured.
func (c *Client) addTrace(ctx context.Context, operation string, attributes map[string]string) (context.Context, trace.Span) {
	if c.tracer != nil {
		contextWithTrace, span := c.tracer.Start(ctx, fmt.Sprintf("arangodb-%v", operation))

		// Add default attributes
		span.SetAttributes(attribute.String("arangodb.operation", operation))

		// Add custom attributes if provided
		for key, value := range attributes {
			span.SetAttributes(attribute.String(fmt.Sprintf("arangodb.%s", key), value))
		}

		return contextWithTrace, span
	}

	return ctx, nil
}

func (c *Client) sendOperationStats(ql *QueryLog, startTime time.Time, method string, span trace.Span) {
	duration := time.Since(startTime).Microseconds()
	ql.Duration = duration

	c.logger.Debug(ql)

	c.metrics.RecordHistogram(context.Background(), "app_arango_stats", float64(duration),
		"endpoint", c.endpoint,
		"type", ql.Query,
	)

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64(fmt.Sprintf("arangodb.%v.duration", method), duration))
	}
}

// Health represents the health status of ArangoDB.
type Health struct {
	Status  string         `json:"status,omitempty"`
	Details map[string]any `json:"details,omitempty"`
}

// HealthCheck performs a health check.
func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	h := Health{
		Details: map[string]any{
			"endpoint": c.endpoint,
		},
	}

	version, err := c.client.Version(ctx)
	if err != nil {
		h.Status = "DOWN"
		return &h, errStatusDown
	}

	h.Status = "UP"
	h.Details["version"] = version.Version
	h.Details["server"] = version.Server

	return &h, nil
}
