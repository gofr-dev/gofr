package arango

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
}

// Config holds the configuration for ArangoDB connection.
type Config struct {
	Host     string
	User     string
	Password string
	Port     int
}

// EdgeDefinition represents the definition of edges in a graph.
type EdgeDefinition struct {
	// Collection is the name of the edge collection to be used
	Collection string `json:"collection"`
	// From is an array of vertex collection names for the source vertices
	From []string `json:"from"`
	// To is an array of vertex collection names for the target vertices
	To []string `json:"to"`
}

var (
	errStatusDown        = errors.New("status down")
	errMissingField      = errors.New("missing required field in config")
	errInvalidResultType = errors.New("result must be a pointer to a slice of maps")
)

// New creates a new ArangoDB client with the provided configuration.
func New(c Config) *Client {
	return &Client{config: &c}
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
func (c *Client) CreateUser(ctx context.Context, name string, options *arangodb.UserOptions) (arangodb.User, error) {
	tracerCtx, span := c.addTrace(ctx, "createUser", map[string]string{"user": name})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "createUser"}, startTime, "createUser", span)

	user, err := c.client.CreateUser(tracerCtx, name, options)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// DropUser deletes a user from ArangoDB.
func (c *Client) DropUser(ctx context.Context, username string) error {
	tracerCtx, span := c.addTrace(ctx, "dropUser", map[string]string{"user": username})
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
	tracerCtx, span := c.addTrace(ctx, "grantDB", map[string]string{"db": database})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "grantDB", Collection: database, ID: username}, startTime, "grantDB", span)

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

	defer c.sendOperationStats(&QueryLog{Query: "GrantCollection", Collection: database, ID: username}, startTime, "GrantCollection", span)

	user, err := c.client.User(tracerCtx, username)
	if err != nil {
		return err
	}

	err = user.SetCollectionAccess(tracerCtx, database, collection, arangodb.Grant(permission))

	return err
}

// ListDBs returns a list of all databases in ArangoDB.
func (c *Client) ListDBs(ctx context.Context) ([]string, error) {
	tracerCtx, span := c.addTrace(ctx, "listDBs", nil)
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "listDBs"}, startTime, "listDBs", span)

	dbs, err := c.client.Databases(tracerCtx)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(dbs))

	for _, db := range dbs {
		if db.Name() != "" {
			names = append(names, db.Name())
		}
	}

	return names, nil
}

// CreateDB creates a new database in ArangoDB.
func (c *Client) CreateDB(ctx context.Context, database string) error {
	tracerCtx, span := c.addTrace(ctx, "createDB", map[string]string{"db": database})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "createDB", Collection: database}, startTime, "createDB", span)

	_, err := c.client.CreateDatabase(tracerCtx, database, nil)

	return err
}

// DropDB deletes a database from ArangoDB.
func (c *Client) DropDB(ctx context.Context, database string) error {
	tracerCtx, span := c.addTrace(ctx, "dropDB", map[string]string{"db": database})
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
	tracerCtx, span := c.addTrace(ctx, "createCollection", map[string]string{"collection": collection})
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

func (c *Client) getCollection(ctx context.Context, dbName, collectionName string) (arangodb.Collection, error) {
	db, err := c.client.Database(ctx, dbName)
	if err != nil {
		return nil, err
	}

	collection, err := db.Collection(ctx, collectionName)
	if err != nil {
		return nil, err
	}

	return collection, nil
}

// DropCollection deletes an existing collection from a database.
func (c *Client) DropCollection(ctx context.Context, database, collectionName string) error {
	tracerCtx, span := c.addTrace(ctx, "dropCollection", map[string]string{"collection": collectionName})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "dropCollection", Collection: collectionName}, startTime, "dropCollection", span)

	collection, err := c.getCollection(tracerCtx, database, collectionName)
	if err != nil {
		return err
	}

	err = collection.Remove(ctx)
	if err != nil {
		return err
	}

	return err
}

// TruncateCollection truncates a collection in a database.
func (c *Client) TruncateCollection(ctx context.Context, database, collectionName string) error {
	tracerCtx, span := c.addTrace(ctx, "truncateCollection", map[string]string{"collection": collectionName})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "truncateCollection", Collection: collectionName}, startTime, "truncateCollection", span)

	collection, err := c.getCollection(tracerCtx, database, collectionName)
	if err != nil {
		return err
	}

	err = collection.Truncate(ctx)
	if err != nil {
		return err
	}

	return err
}

// ListCollections lists all collections in a database.
func (c *Client) ListCollections(ctx context.Context, database string) ([]string, error) {
	tracerCtx, span := c.addTrace(ctx, "listCollections", map[string]string{"db": database})
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

	names := make([]string, 0, len(collections))
	for _, coll := range collections {
		names = append(names, coll.Name())
	}

	return names, nil
}

// CreateDocument creates a new document in the specified collection.
func (c *Client) CreateDocument(ctx context.Context, dbName, collectionName string, document any) (string, error) {
	tracerCtx, span := c.addTrace(ctx, "createDocument", map[string]string{"collection": collectionName})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "createDocument", Collection: collectionName}, startTime, "createDocument", span)

	collection, err := c.getCollection(tracerCtx, dbName, collectionName)
	if err != nil {
		return "", err
	}

	meta, err := collection.CreateDocument(tracerCtx, document)
	if err != nil {
		return "", err
	}

	return meta.Key, nil
}

// GetDocument retrieves a document by its ID from the specified collection.
func (c *Client) GetDocument(ctx context.Context, dbName, collectionName, documentID string, result any) error {
	tracerCtx, span := c.addTrace(ctx, "getDocument", map[string]string{"collection": collectionName})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "getDocument", Collection: collectionName, ID: documentID}, startTime, "getDocument", span)

	collection, err := c.getCollection(tracerCtx, dbName, collectionName)
	if err != nil {
		return err
	}

	_, err = collection.ReadDocument(tracerCtx, documentID, result)

	return err
}

// UpdateDocument updates an existing document in the specified collection.
func (c *Client) UpdateDocument(ctx context.Context, dbName, collectionName, documentID string, document any) error {
	tracerCtx, span := c.addTrace(ctx, "updateDocument", map[string]string{"collection": collectionName})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "updateDocument", Collection: collectionName,
		ID: documentID}, startTime, "updateDocument", span)

	collection, err := c.getCollection(tracerCtx, dbName, collectionName)
	if err != nil {
		return err
	}

	_, err = collection.UpdateDocument(tracerCtx, documentID, document)

	return err
}

// DeleteDocument deletes a document by its ID from the specified collection.
func (c *Client) DeleteDocument(ctx context.Context, dbName, collectionName, documentID string) error {
	tracerCtx, span := c.addTrace(ctx, "deleteDocument", map[string]string{"collection": collectionName})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "deleteDocument", Collection: collectionName,
		ID: documentID}, startTime, "deleteDocument", span)

	collection, err := c.getCollection(tracerCtx, dbName, collectionName)
	if err != nil {
		return err
	}

	_, err = collection.DeleteDocument(tracerCtx, documentID)

	return err
}

// CreateEdgeDocument creates a new edge document between two vertices.
func (c *Client) CreateEdgeDocument(ctx context.Context, dbName, collectionName, from, to string, document any) (string, error) {
	tracerCtx, span := c.addTrace(ctx, "createEdgeDocument", map[string]string{"collection": collectionName})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "createEdgeDocument", Collection: collectionName}, startTime, "createEdgeDocument", span)

	collection, err := c.getCollection(tracerCtx, dbName, collectionName)
	if err != nil {
		return "", err
	}

	meta, err := collection.CreateDocument(tracerCtx, map[string]any{
		"_from": from,
		"_to":   to,
		"data":  document,
	})
	if err != nil {
		return "", err
	}

	return meta.Key, nil
}

// CreateGraph creates a new graph in a database.
func (c *Client) CreateGraph(ctx context.Context, database, graph string, edgeDefinitions []EdgeDefinition) error {
	tracerCtx, span := c.addTrace(ctx, "createGraph", map[string]string{"graph": graph})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "createGraph", Collection: graph}, startTime, "createGraph", span)

	db, err := c.client.Database(tracerCtx, database)
	if err != nil {
		return err
	}

	arangoEdgeDefs := make([]arangodb.EdgeDefinition, 0, len(edgeDefinitions))
	for _, ed := range edgeDefinitions {
		arangoEdgeDefs = append(arangoEdgeDefs, arangodb.EdgeDefinition{
			Collection: ed.Collection,
			From:       ed.From,
			To:         ed.To,
		})
	}

	options := &arangodb.GraphDefinition{
		EdgeDefinitions: arangoEdgeDefs,
	}

	_, err = db.CreateGraph(tracerCtx, graph, options, nil)

	return err
}

// DropGraph deletes an existing graph from a database.
func (c *Client) DropGraph(ctx context.Context, database, graphName string) error {
	tracerCtx, span := c.addTrace(ctx, "dropGraph", map[string]string{"graph": graphName})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "dropGraph", Collection: graphName}, startTime, "dropGraph", span)

	db, err := c.client.Database(tracerCtx, database)
	if err != nil {
		return err
	}

	graph, err := db.Graph(tracerCtx, graphName, nil)
	if err != nil {
		return err
	}

	err = graph.Remove(tracerCtx, &arangodb.RemoveGraphOptions{DropCollections: true})
	if err != nil {
		return err
	}

	return err
}

// ListGraphs lists all graphs in a database.
func (c *Client) ListGraphs(ctx context.Context, database string) ([]string, error) {
	tracerCtx, span := c.addTrace(ctx, "listGraphs", map[string]string{})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: "listGraphs", Collection: database}, startTime, "listGraphs", span)

	db, err := c.client.Database(tracerCtx, database)
	if err != nil {
		return nil, err
	}

	graphsReader, err := db.Graphs(tracerCtx)
	if err != nil {
		return nil, err
	}

	var graphNames []string

	for {
		graph, err := graphsReader.Read()
		if errors.Is(err, shared.NoMoreDocumentsError{}) {
			break
		}

		if err != nil {
			return nil, err
		}

		graphNames = append(graphNames, graph.Name())
	}

	return graphNames, nil
}

// Query executes an AQL query and binds the results.
func (c *Client) Query(ctx context.Context, dbName, query string, bindVars map[string]any, result any) error {
	tracerCtx, span := c.addTrace(ctx, "query", map[string]string{"db": dbName})
	startTime := time.Now()

	defer c.sendOperationStats(&QueryLog{Query: query}, startTime, "query", span)

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
		if errors.Is(err, shared.NoMoreDocumentsError{}) {
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
		contextWithTrace, span := c.tracer.Start(ctx, fmt.Sprintf("arango-%v", operation))

		// Add default attributes
		span.SetAttributes(attribute.String("arango.operation", operation))

		// Add custom attributes if provided
		for key, value := range attributes {
			span.SetAttributes(attribute.String(fmt.Sprintf("arango.%s", key), value))
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
		span.SetAttributes(attribute.Int64(fmt.Sprintf("arango.%v.duration", method), duration))
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
