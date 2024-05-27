# Injecting Database Drivers
Keeping in mind the size of the framework in the final build, it felt counter-productive to keep all the database drivers within
the framework itself. Keeping only the most used MySQL and Redis within the framework, users can now inject databases
in the server that satisfies the base interface defined by GoFr. This helps in reducing the build size and in turn build time
as unnecessary database drivers are not being compiled and added to the build.

> We are planning to provide custom drivers for most common databases, and is in the pipeline for upcoming releases!

## MongoDB
GoFr supports injecting MongoDB that supports the following interface. Any driver that implements the interface can be added
using `app.AddMongo()` method, and user's can use MongoDB across application with `gofr.Context`.
```go
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
```

User's can easily inject a driver that supports this interface, this provides usability without
compromising the extensibility to use multiple databases.
### Example
```go
package main

import (
    "gofr.dev/pkg/gofr/datasource/mongo"
    "go.mongodb.org/mongo-driver/bson"
	
    "gofr.dev/pkg/gofr"
)

type Person struct {
	Name string `bson:"name" json:"name"`
	Age  int    `bson:"age" json:"age"`
	City string `bson:"city" json:"city"`
}

func main() {
	app := gofr.New()
	
	db := mongo.New(Config{URI: "mongodb://localhost:27017", Database: "test"})
	
	// inject the mongo into gofr to use mongoDB across the application
	// using gofr context
	app.AddMongo(db)

	app.POST("/mongo", Insert)
	app.GET("/mongo", Get)

	app.Run()
}

func Insert(ctx *gofr.Context) (interface{}, error) {
	var p Person
	err := ctx.Bind(&p)
	if err != nil {
		return nil, err
	}

	res, err := ctx.Mongo.InsertOne(ctx, "collection", p)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func Get(ctx *gofr.Context) (interface{}, error) {
	var result Person

	p := ctx.Param("name")

	err := ctx.Mongo.FindOne(ctx, "collection", bson.D{{"name", p}} /* valid filter */, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
```