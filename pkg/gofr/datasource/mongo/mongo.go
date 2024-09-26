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
func (c *Client) Connect() {
	c.logger.Logf("connecting to mongoDB at %v to database %v", c.config.URI, c.config.Database)

	uri := c.config.URI

	if uri == "" {
		uri = fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin",
			c.config.User, c.config.Password, c.config.Host, c.config.Port, c.config.Database)
	}

	m, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		c.logger.Errorf("error connecting to mongoDB, err:%v", err)

		return
	}

	mongoBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	c.metrics.NewHistogram("app_mongo_stats", "Response time of MONGO queries in milliseconds.", mongoBuckets...)

	c.Database = m.Database(c.config.Database)
}

// InsertOne inserts a single document into the specified collection.
func (c *Client) InsertOne(ctx context.Context, collection string, document interface{}) (interface{}, error) {
	defer c.sendOperationStats(&QueryLog{Query: "insertOne", Collection: collection, Filter: document}, time.Now())

	start := time.Now()

	tracedCtx, span := c.addTraces(ctx, "insert", collection)

	result, err := c.Database.Collection(collection).InsertOne(tracedCtx, document)
	duration := time.Since(start).Microseconds()

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64("mongo.insert.duration", duration))
	}

	return result, err
}

// InsertMany inserts multiple documents into the specified collection.
func (c *Client) InsertMany(ctx context.Context, collection string, documents []interface{}) ([]interface{}, error) {
	defer c.sendOperationStats(&QueryLog{Query: "insertMany", Collection: collection, Filter: documents}, time.Now())

	start := time.Now()

	tracedCtx, span := c.addTraces(ctx, "insertMany", collection)

	res, err := c.Database.Collection(collection).InsertMany(tracedCtx, documents)
	if err != nil {
		return nil, err
	}

	duration := time.Since(start).Microseconds()

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64("mongo.insertMany.duration", duration))
	}

	return res.InsertedIDs, nil
}

// Find retrieves documents from the specified collection based on the provided filter and binds response to result.
func (c *Client) Find(ctx context.Context, collection string, filter, results interface{}) error {
	defer c.sendOperationStats(&QueryLog{Query: "find", Collection: collection, Filter: filter}, time.Now())

	start := time.Now()

	tracedCtx, span := c.addTraces(ctx, "find", collection)

	cur, err := c.Database.Collection(collection).Find(tracedCtx, filter)
	if err != nil {
		return err
	}

	defer cur.Close(ctx)

	if err := cur.All(ctx, results); err != nil {
		return err
	}

	duration := time.Since(start).Microseconds()

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64("mongo.find.duration", duration))
	}

	return nil
}

// FindOne retrieves a single document from the specified collection based on the provided filter and binds response to result.
func (c *Client) FindOne(ctx context.Context, collection string, filter, result interface{}) error {
	defer c.sendOperationStats(&QueryLog{Query: "findOne", Collection: collection, Filter: filter}, time.Now())

	start := time.Now()

	tracedCtx, span := c.addTraces(ctx, "findOne", collection)

	b, err := c.Database.Collection(collection).FindOne(tracedCtx, filter).Raw()
	if err != nil {
		return err
	}

	duration := time.Since(start).Microseconds()

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64("mongo.findOne.duration", duration))
	}

	return bson.Unmarshal(b, result)
}

// UpdateByID updates a document in the specified collection by its ID.
func (c *Client) UpdateByID(ctx context.Context, collection string, id, update interface{}) (int64, error) {
	defer c.sendOperationStats(&QueryLog{Query: "updateByID", Collection: collection, ID: id, Update: update}, time.Now())

	start := time.Now()

	tracedCtx, span := c.addTraces(ctx, "updateByID", collection)

	res, err := c.Database.Collection(collection).UpdateByID(tracedCtx, id, update)

	duration := time.Since(start).Microseconds()

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64("mongo.updateByID.duration", duration))
	}

	return res.ModifiedCount, err
}

// UpdateOne updates a single document in the specified collection based on the provided filter.
func (c *Client) UpdateOne(ctx context.Context, collection string, filter, update interface{}) error {
	defer c.sendOperationStats(&QueryLog{Query: "updateOne", Collection: collection, Filter: filter, Update: update}, time.Now())

	start := time.Now()

	tracedCtx, span := c.addTraces(ctx, "updateOne", collection)

	_, err := c.Database.Collection(collection).UpdateOne(tracedCtx, filter, update)

	duration := time.Since(start).Microseconds()

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64("mongo.updateOne.duration", duration))
	}

	return err
}

// UpdateMany updates multiple documents in the specified collection based on the provided filter.
func (c *Client) UpdateMany(ctx context.Context, collection string, filter, update interface{}) (int64, error) {
	defer c.sendOperationStats(&QueryLog{Query: "updateMany", Collection: collection, Filter: filter, Update: update}, time.Now())

	start := time.Now()

	tracedCtx, span := c.addTraces(ctx, "updateMany", collection)

	res, err := c.Database.Collection(collection).UpdateMany(tracedCtx, filter, update)

	duration := time.Since(start).Microseconds()

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64("mongo.updateMany.duration", duration))
	}

	return res.ModifiedCount, err
}

// CountDocuments counts the number of documents in the specified collection based on the provided filter.
func (c *Client) CountDocuments(ctx context.Context, collection string, filter interface{}) (int64, error) {
	defer c.sendOperationStats(&QueryLog{Query: "countDocuments", Collection: collection, Filter: filter}, time.Now())

	start := time.Now()

	tracedCtx, span := c.addTraces(ctx, "countDocuments", collection)

	result, err := c.Database.Collection(collection).CountDocuments(tracedCtx, filter)

	duration := time.Since(start).Microseconds()

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64("mongo.countDocuments.duration", duration))
	}

	return result, err
}

// DeleteOne deletes a single document from the specified collection based on the provided filter.
func (c *Client) DeleteOne(ctx context.Context, collection string, filter interface{}) (int64, error) {
	defer c.sendOperationStats(&QueryLog{Query: "deleteOne", Collection: collection, Filter: filter}, time.Now())

	start := time.Now()

	tracedCtx, span := c.addTraces(ctx, "deleteOne", collection)

	res, err := c.Database.Collection(collection).DeleteOne(tracedCtx, filter)
	if err != nil {
		return 0, err
	}

	duration := time.Since(start).Microseconds()

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64("mongo.deleteOne.duration", duration))
	}

	return res.DeletedCount, nil
}

// DeleteMany deletes multiple documents from the specified collection based on the provided filter.
func (c *Client) DeleteMany(ctx context.Context, collection string, filter interface{}) (int64, error) {
	defer c.sendOperationStats(&QueryLog{Query: "deleteMany", Collection: collection, Filter: filter}, time.Now())

	start := time.Now()

	tracedCtx, span := c.addTraces(ctx, "deleteMany", collection)

	res, err := c.Database.Collection(collection).DeleteMany(tracedCtx, filter)
	if err != nil {
		return 0, err
	}

	duration := time.Since(start).Microseconds()

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64("mongo.deleteMany.duration", duration))
	}

	return res.DeletedCount, nil
}

// Drop drops the specified collection from the database.
func (c *Client) Drop(ctx context.Context, collection string) error {
	defer c.sendOperationStats(&QueryLog{Query: "drop", Collection: collection}, time.Now())

	start := time.Now()

	tracedCtx, span := c.addTraces(ctx, "drop", collection)

	err := c.Database.Collection(collection).Drop(tracedCtx)

	duration := time.Since(start).Microseconds()

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64("mongo.drop.duration", duration))
	}

	return err
}

// CreateCollection creates the specified collection in the database.
func (c *Client) CreateCollection(ctx context.Context, name string) error {
	defer c.sendOperationStats(&QueryLog{Query: "createCollection", Collection: name}, time.Now())

	start := time.Now()

	tracedCtx, span := c.addTraces(ctx, "createCollection", name)

	err := c.Database.CreateCollection(tracedCtx, name)

	duration := time.Since(start).Microseconds()

	if span != nil {
		defer span.End()
		span.SetAttributes(attribute.Int64("mongo.createCollection.duration", duration))
	}

	return err
}

func (c *Client) sendOperationStats(ql *QueryLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	ql.Duration = duration

	c.logger.Debug(ql)

	c.metrics.RecordHistogram(context.Background(), "app_mongo_stats", float64(duration), "hostname", c.uri,
		"database", c.database, "type", ql.Query)
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
	defer c.sendOperationStats(&QueryLog{Query: "startSession"}, time.Now())

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

func (c *Client) addTraces(ctx context.Context, method, collection string) (context.Context, trace.Span) {
	if c.tracer != nil {
		contextWithTrace, span := c.tracer.Start(ctx, fmt.Sprintf("mongodb-%v", method))

		span.SetAttributes(
			attribute.String("mongo.collection", collection),
		)

		return contextWithTrace, span
	}

	return ctx, nil
}
