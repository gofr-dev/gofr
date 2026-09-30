---
description: "Use MongoDB with GoFr through the framework's pluggable datasource interface. Insert, find, update, and aggregate documents with traces and metrics."
nextjs:
  metadata:
    title: "MongoDB in GoFr — Document Database Integration"
    description: "Use MongoDB with GoFr through the framework's pluggable datasource interface. Insert, find, update, and aggregate documents with traces and metrics."
---

# MongoDB

## Configuration
To connect to `MongoDB`, you need to provide the following environment variables:
- `URI`: Mongodb server URL that the client connects to.
- `DATABASE`: The name of the database to connect to.
- `CONNECTIONTIMEOUT`: The maximum time the client will wait while trying to establish a connection.

If MongoDB is unreachable at startup, GoFr logs the error and keeps the client; the health check reports `DOWN` (with the connection error in `details.error`) until the server becomes reachable, after which operations and the `app_mongo_stats` metric work without a restart. While the server is down, an operation fails after the caller's context deadline or the driver's server-selection timeout (30 seconds by default); it does not fail immediately and does not panic.

If the configuration is invalid (for example, a malformed `URI`), GoFr logs the error, the health check reports `DOWN` with the reason in `details.error`, and the GoFr methods return a "not connected to MongoDB" error instead of panicking. Methods of the embedded `*mongo.Database` field (such as `Collection` or `RunCommand`) are not guarded: that field is `nil` in this case and must not be called directly.


## Setup
GoFr supports injecting MongoDB that supports the following interface. Any driver that implements the interface can be added
using `app.AddMongo()` method, and users can use MongoDB across application with `gofr.Context`.
```go
type Mongo interface {
	Find(ctx context.Context, collection string, filter any, results any) error

	FindOne(ctx context.Context, collection string, filter any, result any) error

	InsertOne(ctx context.Context, collection string, document any) (any, error)

	InsertMany(ctx context.Context, collection string, documents []any) ([]any, error)

	DeleteOne(ctx context.Context, collection string, filter any) (int64, error)

	DeleteMany(ctx context.Context, collection string, filter any) (int64, error)

	UpdateByID(ctx context.Context, collection string, id any, update any) (int64, error)

	UpdateOne(ctx context.Context, collection string, filter any, update any) error

	UpdateMany(ctx context.Context, collection string, filter any, update any) (int64, error)

	CountDocuments(ctx context.Context, collection string, filter any) (int64, error)

	Drop(ctx context.Context, collection string) error
}
```

Users can easily inject a driver that supports this interface; this provides usability without compromising the extensibility to use multiple databases.

Import the gofr's external driver for MongoDB:

```shell
go get gofr.dev/pkg/gofr/datasource/mongo@latest
```

### Example
```go
package main

import (
	"time"
	"go.mongodb.org/mongo-driver/bson"
	"gofr.dev/pkg/gofr/datasource/mongo"

	"gofr.dev/pkg/gofr"
)

type Person struct {
	Name string `bson:"name" json:"name"`
	Age  int    `bson:"age" json:"age"`
	City string `bson:"city" json:"city"`
}

func main() {
	app := gofr.New()

	db := mongo.New(mongo.Config{URI: app.Config.Get("URI"), Database: app.Config.Get("DATABASE"), ConnectionTimeout: app.Config.Get("CONNECTIONTIMEOUT")})

	// inject the mongo into gofr to use mongoDB across the application
	// using gofr context
	app.AddMongo(db)

	app.POST("/mongo", Insert)
	app.GET("/mongo/{name}", Get)

	app.Run()
}

func Insert(ctx *gofr.Context) (any, error) {
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

func Get(ctx *gofr.Context) (any, error) {
	var result Person

	p := ctx.PathParam("name")

	err := ctx.Mongo.FindOne(ctx, "collection", bson.D{{"name", p}} /* valid filter */, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
```
