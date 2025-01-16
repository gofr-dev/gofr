package arango

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/arangodb/go-driver/v2/arangodb"
	"github.com/arangodb/go-driver/v2/connection"
)

// Client represents an ArangoDB client.
type Client struct {
	client   arangodb.Client
	logger   Logger
	metrics  Metrics
	tracer   trace.Tracer
	config   *Config
	endpoint string
}

// Config holds the configuration for ArangoDB connection.
type Config struct {
	Host     string
	User     string
	Password string
	Port     int
}

const defaultTimeout = 5 * time.Second

var (
	errStatusDown   = errors.New("status down")
	errMissingField = errors.New("missing required field in config")
)

// New creates a new ArangoDB client with the provided configuration.
func New(c Config) *Client {
	return &Client{config: &c}
}

// UseLogger sets the logger for the ArangoDB client.
func (c *Client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the ArangoDB client.
func (c *Client) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for the ArangoDB client.
func (c *Client) UseTracer(tracer interface{}) {
	if t, ok := tracer.(trace.Tracer); ok {
		c.tracer = t
	}
}

// Connect establishes a connection to the ArangoDB server
func (c *Client) Connect() {
	if err := c.validateConfig(); err != nil {
		c.logger.Errorf("config validation error: %v", err)
		return
	}

	c.endpoint = fmt.Sprintf("http://%s:%d", c.config.Host, c.config.Port)
	c.logger.Debugf("connecting to ArangoDB at %s", c.endpoint)

	endpoint := connection.NewRoundRobinEndpoints([]string{c.endpoint})
	conn := connection.NewHttp2Connection(connection.DefaultHTTP2ConfigurationWrapper(endpoint /*InsecureSkipVerify*/, false))

	auth := connection.NewBasicAuth(c.config.User, c.config.Password)

	err := conn.SetAuthentication(auth)
	if err != nil {
		c.logger.Errorf("Failed to set authentication: %v", err)
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

// CreateUser creates a new user in ArangoDB
func (c *Client) CreateUser(ctx context.Context, username, password string) error {
	tracerCtx, span := c.addTrace(ctx, "createUser", "")
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "createUser"}, startTime, "createUser", span)

	options := arangodb.UserOptions{Password: password}
	_, err := c.client.CreateUser(tracerCtx, username, &options)

	return err
}

// DropUser deletes a user from ArangoDB
func (c *Client) DropUser(ctx context.Context, username string) error {
	tracerCtx, span := c.addTrace(ctx, "dropUser", "")
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "dropUser"}, startTime, "dropUser", span)

	err := c.client.RemoveUser(tracerCtx, username)
	if err != nil {
		return err
	}

	return err
}

// GrantDB grants permissions for a database to a user.
func (c *Client) GrantDB(ctx context.Context, database, username, permission string) error {
	tracerCtx, span := c.addTrace(ctx, "grantDB", "")
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "grantDB", Collection: database, ID: username}, startTime, "grantDB", span)

	user, err := c.client.User(tracerCtx, username)
	if err != nil {
		return err
	}

	err = user.SetDatabaseAccess(tracerCtx, database, arangodb.Grant(permission))

	return err
}

// ListDBs returns a list of all databases in ArangoDB
func (c *Client) ListDBs(ctx context.Context) ([]string, error) {
	tracerCtx, span := c.addTrace(ctx, "listDBs", "")
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "listDBs"}, startTime, "listDBs", span)

	dbs, err := c.client.Databases(tracerCtx)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, db := range dbs {
		names = append(names, db.Name())
	}

	return names, nil
}

// CreateDB creates a new database in ArangoDB
func (c *Client) CreateDB(ctx context.Context, database string) error {
	tracerCtx, span := c.addTrace(ctx, "createDB", "")
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "createDB", Collection: database}, startTime, "createDB", span)

	_, err := c.client.CreateDatabase(tracerCtx, database, nil)

	return err
}

// DropDB deletes a database from ArangoDB
func (c *Client) DropDB(ctx context.Context, database string) error {
	tracerCtx, span := c.addTrace(ctx, "dropDB", "")
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "dropDB", Collection: database}, startTime, "dropDB", span)

	db, err := c.client.Database(tracerCtx, database)
	if err != nil {
		return err
	}

	err = db.Remove(tracerCtx)
	if err != nil {
		return err
	}

	return err
}

// CreateCollection creates a new collection in a database with specified type.
func (c *Client) CreateCollection(ctx context.Context, database, collection string, isEdge bool) error {
	tracerCtx, span := c.addTrace(ctx, "createCollection", "")
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "createCollection", Collection: collection}, startTime, "createCollection", span)

	db, err := c.client.Database(tracerCtx, database)
	if err != nil {
		return err
	}

	options := arangodb.CreateCollectionProperties{Type: arangodb.CollectionTypeDocument}
	if isEdge {
		options.Type = arangodb.CollectionTypeEdge
	}

	_, err = db.CreateCollection(tracerCtx, collection, &options)

	return err
}

// DropCollection deletes an existing collection from a database.
func (c *Client) DropCollection(ctx context.Context, database, collection string) error {
	tracerCtx, span := c.addTrace(ctx, "dropCollection", "")
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "dropCollection", Collection: collection}, startTime, "dropCollection", span)

	db, err := c.client.Database(tracerCtx, database)
	if err != nil {
		return err
	}

	coll, err := db.Collection(tracerCtx, collection)
	if err != nil {
		return err
	}

	err = coll.Remove(ctx)
	if err != nil {
		return err
	}

	return err
}

// TruncateCollection truncates a collection in a database.
func (c *Client) TruncateCollection(ctx context.Context, database, collection string) error {
	tracerCtx, span := c.addTrace(ctx, "truncateCollection", "")
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "truncateCollection", Collection: collection}, startTime, "truncateCollection", span)

	db, err := c.client.Database(tracerCtx, database)
	if err != nil {
		return err
	}

	coll, err := db.Collection(tracerCtx, collection)
	if err != nil {
		return err
	}

	err = coll.Truncate(ctx)
	if err != nil {
		return err
	}

	return err
}

// ListCollections lists all collections in a database.
func (c *Client) ListCollections(ctx context.Context, database string) ([]string, error) {
	tracerCtx, span := c.addTrace(ctx, "listCollections", "")
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "listCollections", Collection: database}, startTime, "listCollections", span)

	db, err := c.client.Database(tracerCtx, database)
	if err != nil {
		return nil, err
	}

	collections, err := db.Collections(tracerCtx)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, coll := range collections {
		names = append(names, coll.Name())
	}

	return names, nil
}

// GrantCollection grants permissions for a collection to a user.
func (c *Client) GrantCollection(ctx context.Context, database, collection, username, permission string) error {
	tracerCtx, span := c.addTrace(ctx, "GrantCollection", "")
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "GrantCollection", Collection: database, ID: username}, startTime, "GrantCollection", span)

	user, err := c.client.User(tracerCtx, username)
	if err != nil {
		return err
	}

	err = user.SetCollectionAccess(tracerCtx, database, collection, arangodb.Grant(permission))

	return err
}

// addTrace adds tracing to context if tracer is configured
func (c *Client) addTrace(ctx context.Context, operation, collection string) (context.Context, trace.Span) {
	if c.tracer != nil {
		contextWithTrace, span := c.tracer.Start(ctx, fmt.Sprintf("arango-%v", operation))
		span.SetAttributes(
			attribute.String("arango.collection", collection),
			attribute.String("arango.operation", operation),
		)

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
		span.SetAttributes(attribute.Int64(fmt.Sprintf("arango.%v.duration", method), duration))
	}
}
