package mongo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Client struct {
	*mongo.Database

	uri      string
	database string
	logger   Logger
	metrics  Metrics
	config   Config
	tracer   trace.Tracer
}

type Config struct {
	Host     string
	User     string
	Password string
	Port     int
	Database string
	// Deprecated Provide Host User Password Port Instead and driver will generate the URI
	URI string
}

var errStatusDown = errors.New("status down")

/*
Developer Note: We could have accepted logger and metrics as part of the factory function `New`, but when mongo driver is
initialised in GoFr, We want to ensure that the user need not to provides logger and metrics and then connect to the database,
i.e. by default observability features gets initialised when used with GoFr.
*/

// New initializes MongoDB driver with the provided configuration.
// The Connect method must be called to establish a connection to MongoDB.
// Usage:
// client := New(config)
// client.UseLogger(loggerInstance)
// client.UseMetrics(metricsInstance)
// client.Connect().
func New(c Config) *Client {
	return &Client{config: c}
}

// UseLogger sets the logger for the MongoDB client which asserts the Logger interface.
func (c *Client) UseLogger(logger interface{}) {
	if l, ok := logger.(Logger); ok {
		c.logger = l
	}
}

// UseMetrics sets the metrics for the MongoDB client which asserts the Metrics interface.
func (c *Client) UseMetrics(metrics interface{}) {
	if m, ok := metrics.(Metrics); ok {
		c.metrics = m
	}
}

// UseTracer sets the tracer for the MongoDB client.
func (c *Client) UseTracer(tracer any) {
	if tracer, ok := tracer.(trace.Tracer); ok {
		c.tracer = tracer
	}
}

// Connect establishes a connection to MongoDB and registers metrics using the provided configuration when the client was Created.
func (c *Client) Connect(ctx context.Context) error {
	c.logger.Logf("connecting to mongoDB at %v to database %v", c.config.URI, c.config.Database)

	uri := c.config.URI

	if uri == "" {
		uri = fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin",
			c.config.User, c.config.Password, c.config.Host, c.config.Port, c.config.Database)
	}

	m, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		c.logger.Errorf("error connecting to mongoDB, err:%v", err)

		return err
	}

	mongoBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_mongo_stats", "Response time of MONGO queries in milliseconds.", mongoBuckets...)

	c.Database = m.Database(c.config.Database)

	return nil
}

// InsertOne inserts a single document into the specified collection.
func (c *Client) InsertOne(ctx context.Context, collection string, document interface{}) (interface{}, error) {
	tracerCtx, span := c.addTrace(ctx, "insertOne", collection)

	result, err := c.Database.Collection(collection).InsertOne(tracerCtx, document)

	defer c.sendOperationStats(&QueryLog{Query: "insertOne", Collection: collection, Filter: document}, time.Now(),
		"insert", span)

	return result, err
}

// InsertMany inserts multiple documents into the specified collection.
func (c *Client) InsertMany(ctx context.Context, collection string, documents []interface{}) ([]interface{}, error) {
	tracerCtx, span := c.addTrace(ctx, "insertMany", collection)

	res, err := c.Database.Collection(collection).InsertMany(tracerCtx, documents)
	if err != nil {
		return nil, err
	}

	defer c.sendOperationStats(&QueryLog{Query: "insertMany", Collection: collection, Filter: documents}, time.Now(),
		"insertMany", span)

	return res.InsertedIDs, nil
}

// Find retrieves documents from the specified collection based on the provided filter and binds response to result.
func (c *Client) Find(ctx context.Context, collection string, filter, results interface{}) error {
	tracerCtx, span := c.addTrace(ctx, "find", collection)

	cur, err := c.Database.Collection(collection).Find(tracerCtx, filter)
	if err != nil {
		return err
	}

	defer cur.Close(ctx)

	if err := cur.All(ctx, results); err != nil {
		return err
	}

	defer c.sendOperationStats(&QueryLog{Query: "find", Collection: collection, Filter: filter}, time.Now(), "find",
		span)

	return nil
}

// FindOne retrieves a single document from the specified collection based on the provided filter and binds response to result.
func (c *Client) FindOne(ctx context.Context, collection string, filter, result interface{}) error {
	tracerCtx, span := c.addTrace(ctx, "findOne", collection)

	b, err := c.Database.Collection(collection).FindOne(tracerCtx, filter).Raw()
	if err != nil {
		return err
	}

	defer c.sendOperationStats(&QueryLog{Query: "findOne", Collection: collection, Filter: filter}, time.Now(),
		"findOne", span)

	return bson.Unmarshal(b, result)
}

// UpdateByID updates a document in the specified collection by its ID.
func (c *Client) UpdateByID(ctx context.Context, collection string, id, update interface{}) (int64, error) {
	tracerCtx, span := c.addTrace(ctx, "updateByID", collection)

	res, err := c.Database.Collection(collection).UpdateByID(tracerCtx, id, update)

	defer c.sendOperationStats(&QueryLog{Query: "updateByID", Collection: collection, ID: id, Update: update}, time.Now(),
		"updateByID", span)

	return res.ModifiedCount, err
}

// UpdateOne updates a single document in the specified collection based on the provided filter.
func (c *Client) UpdateOne(ctx context.Context, collection string, filter, update interface{}) error {
	tracerCtx, span := c.addTrace(ctx, "updateOne", collection)

	_, err := c.Database.Collection(collection).UpdateOne(tracerCtx, filter, update)

	defer c.sendOperationStats(&QueryLog{Query: "updateOne", Collection: collection, Filter: filter, Update: update},
		time.Now(), "updateOne", span)

	return err
}

// UpdateMany updates multiple documents in the specified collection based on the provided filter.
func (c *Client) UpdateMany(ctx context.Context, collection string, filter, update interface{}) (int64, error) {
	tracerCtx, span := c.addTrace(ctx, "updateMany", collection)

	res, err := c.Database.Collection(collection).UpdateMany(tracerCtx, filter, update)

	defer c.sendOperationStats(&QueryLog{Query: "updateMany", Collection: collection, Filter: filter, Update: update}, time.Now(),
		"updateMany", span)

	return res.ModifiedCount, err
}

// CountDocuments counts the number of documents in the specified collection based on the provided filter.
func (c *Client) CountDocuments(ctx context.Context, collection string, filter interface{}) (int64, error) {
	tracerCtx, span := c.addTrace(ctx, "countDocuments", collection)

	result, err := c.Database.Collection(collection).CountDocuments(tracerCtx, filter)

	defer c.sendOperationStats(&QueryLog{Query: "countDocuments", Collection: collection, Filter: filter}, time.Now(),
		"countDocuments", span)

	return result, err
}

// DeleteOne deletes a single document from the specified collection based on the provided filter.
func (c *Client) DeleteOne(ctx context.Context, collection string, filter interface{}) (int64, error) {
	tracerCtx, span := c.addTrace(ctx, "deleteOne", collection)

	res, err := c.Database.Collection(collection).DeleteOne(tracerCtx, filter)
	if err != nil {
		return 0, err
	}

	defer c.sendOperationStats(&QueryLog{Query: "deleteOne", Collection: collection, Filter: filter}, time.Now(),
		"deleteOne", span)

	return res.DeletedCount, nil
}

// DeleteMany deletes multiple documents from the specified collection based on the provided filter.
func (c *Client) DeleteMany(ctx context.Context, collection string, filter interface{}) (int64, error) {
	tracerCtx, span := c.addTrace(ctx, "deleteMany", collection)

	res, err := c.Database.Collection(collection).DeleteMany(tracerCtx, filter)
	if err != nil {
		return 0, err
	}

	defer c.sendOperationStats(&QueryLog{Query: "deleteMany", Collection: collection, Filter: filter}, time.Now(),
		"deleteMany", span)

	return res.DeletedCount, nil
}

// Drop drops the specified collection from the database.
func (c *Client) Drop(ctx context.Context, collection string) error {
	tracerCtx, span := c.addTrace(ctx, "drop", collection)

	err := c.Database.Collection(collection).Drop(tracerCtx)

	defer c.sendOperationStats(&QueryLog{Query: "drop", Collection: collection}, time.Now(), "drop", span)

	return err
}

// CreateCollection creates the specified collection in the database.
func (c *Client) CreateCollection(ctx context.Context, name string) error {
	tracerCtx, span := c.addTrace(ctx, "createCollection", name)

	err := c.Database.CreateCollection(tracerCtx, name)

	defer c.sendOperationStats(&QueryLog{Query: "createCollection", Collection: name}, time.Now(), "createCollection",
		span)

	return err
}

func (c *Client) sendOperationStats(ql *QueryLog, startTime time.Time, method string, span trace.Span) {
	duration := time.Since(startTime).Milliseconds()

	ql.Duration = duration

	c.logger.Debug(ql)

	c.metrics.RecordHistogram(context.Background(), "app_mongo_stats", float64(duration), "hostname", c.uri,
		"database", c.database, "type", ql.Query)

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64(fmt.Sprintf("mongo.%v.duration", method), duration))
	}
}

type Health struct {
	Status  string                 `json:"status,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthCheck checks the health of the MongoDB client by pinging the database.
func (c *Client) HealthCheck(ctx context.Context) (any, error) {
	h := Health{
		Details: make(map[string]interface{}),
	}

	h.Details["host"] = c.uri
	h.Details["database"] = c.database

	err := c.Database.Client().Ping(ctx, readpref.Primary())
	if err != nil {
		h.Status = "DOWN"

		return &h, errStatusDown
	}

	h.Status = "UP"

	return &h, nil
}

func (c *Client) StartSession() (interface{}, error) {
	defer c.sendOperationStats(&QueryLog{Query: "startSession"}, time.Now(), "", nil)

	s, err := c.Client().StartSession()
	ses := &session{s}

	return ses, err
}

type session struct {
	mongo.Session
}

func (s *session) StartTransaction() error {
	return s.Session.StartTransaction()
}

type Transaction interface {
	StartTransaction() error
	AbortTransaction(context.Context) error
	CommitTransaction(context.Context) error
	EndSession(context.Context)
}

func (c *Client) addTrace(ctx context.Context, method, collection string) (context.Context, trace.Span) {
	if c.tracer != nil {
		contextWithTrace, span := c.tracer.Start(ctx, fmt.Sprintf("mongodb-%v", method))

		span.SetAttributes(
			attribute.String("mongo.collection", collection),
		)

		return contextWithTrace, span
	}

	return ctx, nil
}
