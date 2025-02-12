package arangodb

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/arangodb/go-driver/v2/arangodb"
	arangoShared "github.com/arangodb/go-driver/v2/arangodb/shared"
	"github.com/arangodb/go-driver/v2/connection"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	defaultTimeout           = 5 * time.Second
	arangoEdgeCollectionType = 3
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

type EdgeDefinition []arangodb.EdgeDefinition

type UserOptions struct {
	Password string `json:"passwd,omitempty"`
	Active   *bool  `json:"active,omitempty"`
	Extra    any    `json:"extra,omitempty"`
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

	// Use HTTP connection instead of HTTP2
	endpoint := connection.NewRoundRobinEndpoints([]string{c.endpoint})
	conn := connection.NewHttpConnection(connection.HttpConfiguration{Endpoint: endpoint})

	// Set authentication
	auth := connection.NewBasicAuth(c.config.User, c.config.Password)
	if err := conn.SetAuthentication(auth); err != nil {
		c.logger.Errorf("authentication setup failed: %v", err)
		return
	}

	// Create ArangoDB client
	client := arangodb.NewClient(conn)
	c.client = client

	// Test connection by fetching server version
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	_, err := c.client.Version(ctx)
	if err != nil {
		c.logger.Errorf("failed to verify connection: %v", err)
		return
	}

	// Initialize metrics
	arangoBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_arango_stats", "Response time of ArangoDB operations in milliseconds.", arangoBuckets...)

	c.logger.Logf("Connected to ArangoDB successfully at %s", c.endpoint)
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

// Query executes an AQL query and binds the results.
//
// Parameters:
//   - ctx: Request context for tracing and cancellation.
//   - dbName: Name of the database where the query will be executed.
//   - query: AQL query string to be executed.
//   - bindVars: Map of bind variables to be used in the query.
//   - result: Pointer to a slice of maps where the query results will be stored.
//
// Returns an error if the database connection fails, the query execution fails, or the
// result parameter is not a pointer to a slice of maps.
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
		if arangoShared.IsNoMoreDocuments(err) {
			break
		}

		if err != nil {
			return err
		}

		*resultSlice = append(*resultSlice, doc)
	}

	return nil
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

func (uo *UserOptions) toArangoUserOptions() *arangodb.UserOptions {
	return &arangodb.UserOptions{
		Password: uo.Password,
		Active:   uo.Active,
		Extra:    uo.Extra,
	}
}
