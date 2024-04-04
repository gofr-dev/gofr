package datasource

import (
	"context"
)

type Mongo interface {
	Find(ctx context.Context, collection string, filter interface{}, results interface{}) error
	FindOne(ctx context.Context, collection string, filter interface{}, result interface{}) error
	InsertOne(ctx context.Context, collection string, document interface{}) (interface{}, error)
	InsertMany(ctx context.Context, collection string, documents []interface{}) ([]interface{}, error)
	DeleteOne(ctx context.Context, collection string, filter interface{}) (int64, error)
	DeleteMany(ctx context.Context, collection string, filter interface{}) (int64, error)
	UpdateByID(ctx context.Context, collection string, id interface{}, update interface{}) (int64, error)
	UpdateOne(ctx context.Context, collection string, filter interface{}, update interface{}) error
	UpdateMany(ctx context.Context, collection string, filter interface{}, update interface{}) (int64, error)
	CountDocuments(ctx context.Context, collection string, filter interface{}) (int64, error)
	Drop(ctx context.Context, collection string) error
}
