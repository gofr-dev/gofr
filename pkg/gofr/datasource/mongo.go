package datasource

import (
	"context"
	"time"
)

type Config interface {
	Get(string) string
	GetOrDefault(string, string) string
}

type Metrics interface {
	NewCounter(name, desc string)
	NewUpDownCounter(name, desc string)
	NewHistogram(name, desc string, buckets ...float64)
	NewGauge(name, desc string)

	IncrementCounter(ctx context.Context, name string, labels ...string)
	DeltaUpDownCounter(ctx context.Context, name string, value float64, labels ...string)
	RecordHistogram(ctx context.Context, name string, value float64, labels ...string)
	SetGauge(name string, value float64)
}

type Mongo interface {
	BulkWrite(ctx context.Context, collection string, models []WriteModel) (BulkWriteResult, error)
	InsertOne(ctx context.Context, collection string, document interface{}) (InsertOneResult, error)
	InsertMany(ctx context.Context, collection string, documents []interface{}) (InsertManyResult, error)
	DeleteOne(ctx context.Context, collection string, filter interface{}) (int64, error)
	DeleteMany(ctx context.Context, collection string, filter interface{}) (int64, error)
	UpdateByID(ctx context.Context, collection string, id interface{}, update interface{}) (UpdateResult, error)
	UpdateOne(ctx context.Context, collection string, filter interface{}, update interface{}) (UpdateResult, error)
	UpdateMany(ctx context.Context, collection string, filter interface{}, update interface{}) (UpdateResult, error)
	ReplaceOne(ctx context.Context, collection string, filter interface{}, replacement interface{}) (UpdateResult, error)
	Aggregate(ctx context.Context, collection string, pipeline interface{}) (Cursor, error)
	CountDocuments(ctx context.Context, collection string, filter interface{}) (int64, error)
	EstimatedDocumentCount(ctx context.Context, collection string) (int64, error)
	Distinct(ctx context.Context, fieldName string, collection string, filter interface{}) ([]interface{}, error)
	Find(ctx context.Context, collection string, filter interface{}) (cur Cursor, err error)
	FindOne(ctx context.Context, collection string, filter interface{}) SingleResult
	FindOneAndDelete(ctx context.Context, collection string, filter interface{}) SingleResult
	FindOneAndReplace(ctx context.Context, collection string, filter interface{}, replacement interface{}) SingleResult
	FindOneAndUpdate(ctx context.Context, collection string, filter interface{}, update interface{}) SingleResult
	Drop(ctx context.Context, collection string) error
}

type BulkWriteResult struct {
	InsertedCount int64
	MatchedCount  int64
	ModifiedCount int64
	DeletedCount  int64
	UpsertedCount int64
	UpsertedIDs   map[int64]interface{}
}

type InsertOneResult struct {
	InsertedID interface{}
}

type InsertManyResult struct {
	InsertedIDs []interface{}
}

type UpdateResult struct {
	MatchedCount  int64
	ModifiedCount int64
	UpsertedCount int64
	UpsertedID    interface{}
}

type Cursor interface {
	ID() int64
	Next(ctx context.Context) bool
	TryNext(ctx context.Context) bool
	Decode(val interface{}) error
	Err() error
	Close(ctx context.Context) error
	All(ctx context.Context, results interface{}) error
	RemainingBatchLength() int
	SetBatchSize(batchSize int32)
	SetMaxTime(dur time.Duration)
	SetComment(comment interface{})
}

type SingleResult interface {
	Decode(v interface{}) error
	Err() error
}

type WriteModel interface {
	writeModel()
}
