package mongo

import (
	"context"
	"time"

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
}

func New(conf Config, logger Logger, metrics Metrics) *Client {
	mongoURI := conf.Get("MONGO_URI")
	mongoDatabase := conf.Get("MONGO_DATABASE")

	logger.Debugf("connecting with mongo database '%v' at '%v'", mongoDatabase, mongoURI)

	m, err := mongo.Connect(context.Background(), options.Client().ApplyURI(conf.Get("MONGO_URI")))
	if err != nil {
		logger.Errorf("could not connect to mongoDB, error: %v", err)

		return nil
	}

	mongoBuckets := []float64{.05, .075, .1, .125, .15, .2, .3, .5, .75, 1, 2, 3, 4, 5, 7.5, 10}
	metrics.NewHistogram("app_mongo_stats", "Response time of MONGO queries in milliseconds.", mongoBuckets...)

	logger.Logf("connected with mongo database '%v' at '%v", mongoDatabase, mongoURI)

	return &Client{
		Database: m.Database(mongoDatabase),
		logger:   logger,
		metrics:  metrics,
		uri:      mongoURI,
		database: mongoDatabase,
	}
}

func (c *Client) InsertOne(ctx context.Context, collection string, document interface{}) (interface{}, error) {
	defer c.postProcess(&QueryLog{Query: "insertOne", Collection: collection, Filter: document}, time.Now())

	return c.Database.Collection(collection).InsertOne(ctx, document)
}

func (c *Client) InsertMany(ctx context.Context, collection string, documents []interface{}) ([]interface{}, error) {
	defer c.postProcess(&QueryLog{Query: "insertMany", Collection: collection, Filter: documents}, time.Now())

	res, err := c.Database.Collection(collection).InsertMany(ctx, documents)
	if err != nil {
		return nil, err
	}

	return res.InsertedIDs, nil
}

func (c *Client) Find(ctx context.Context, collection string, filter, results interface{}) error {
	defer c.postProcess(&QueryLog{Query: "find", Collection: collection, Filter: filter}, time.Now())

	cur, err := c.Database.Collection(collection).Find(ctx, filter)
	if err != nil {
		return err
	}

	defer cur.Close(ctx)

	if err := cur.All(ctx, results); err != nil {
		return err
	}

	return nil
}

func (c *Client) FindOne(ctx context.Context, collection string, filter, result interface{}) error {
	defer c.postProcess(&QueryLog{Query: "findOne", Collection: collection, Filter: filter}, time.Now())

	b, err := c.Database.Collection(collection).FindOne(ctx, filter).Raw()
	if err != nil {
		return err
	}

	return bson.Unmarshal(b, result)
}

func (c *Client) UpdateByID(ctx context.Context, collection string, id, update interface{}) (int64, error) {
	defer c.postProcess(&QueryLog{Query: "updateByID", Collection: collection, ID: id, Update: update}, time.Now())

	res, err := c.Database.Collection(collection).UpdateByID(ctx, id, update)

	return res.ModifiedCount, err
}

func (c *Client) UpdateOne(ctx context.Context, collection string, filter, update interface{}) error {
	defer c.postProcess(&QueryLog{Query: "updateOne", Collection: collection, Filter: filter, Update: update}, time.Now())

	_, err := c.Database.Collection(collection).UpdateOne(ctx, filter, update)

	return err
}

func (c *Client) UpdateMany(ctx context.Context, collection string, filter, update interface{}) (int64, error) {
	defer c.postProcess(&QueryLog{Query: "updateMany", Collection: collection, Filter: filter, Update: update}, time.Now())

	res, err := c.Database.Collection(collection).UpdateMany(ctx, filter, update)

	return res.ModifiedCount, err
}

func (c *Client) CountDocuments(ctx context.Context, collection string, filter interface{}) (int64, error) {
	defer c.postProcess(&QueryLog{Query: "countDocuments", Collection: collection, Filter: filter}, time.Now())

	return c.Database.Collection(collection).CountDocuments(ctx, filter)
}

func (c *Client) DeleteOne(ctx context.Context, collection string, filter interface{}) (int64, error) {
	defer c.postProcess(&QueryLog{Query: "deleteOne", Collection: collection, Filter: filter}, time.Now())

	res, err := c.Database.Collection(collection).DeleteOne(ctx, filter)
	if err != nil {
		return 0, err
	}

	return res.DeletedCount, nil
}

func (c *Client) DeleteMany(ctx context.Context, collection string, filter interface{}) (int64, error) {
	defer c.postProcess(&QueryLog{Query: "deleteMany", Collection: collection, Filter: filter}, time.Now())

	res, err := c.Database.Collection(collection).DeleteMany(ctx, filter)
	if err != nil {
		return 0, err
	}

	return res.DeletedCount, nil
}

func (c *Client) Drop(ctx context.Context, collection string) error {
	defer c.postProcess(&QueryLog{Query: "drop", Collection: collection}, time.Now())

	return c.Database.Collection(collection).Drop(ctx)
}

func (c *Client) postProcess(ql *QueryLog, startTime time.Time) {
	duration := time.Since(startTime).Milliseconds()

	ql.Duration = duration

	c.logger.Debugf("%v", ql)

	c.metrics.RecordHistogram(context.Background(), "app_mongo_stats", float64(duration), "hostname", c.uri,
		"database", c.database, "type", ql.Query)
}

type Health struct {
	Status  string                 `json:"status,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
}

func (c *Client) HealthCheck() interface{} {
	h := Health{
		Details: make(map[string]interface{}),
	}

	h.Details["host"] = c.uri
	h.Details["database"] = c.database

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := c.Database.Client().Ping(ctx, readpref.Primary())
	if err != nil {
		h.Status = "DOWN"

		return &h
	}

	h.Status = "UP"

	return &h
}
