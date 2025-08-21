## MongoDB

GoFr supports injecting MongoDB that supports the following interface. Any driver that implements the interface can be added
using `app.AddMongo()` method, and user's can use MongoDB across application with `gofr.Context`.
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

User's can easily inject a driver that supports this interface, this provides usability without
compromising the extensibility to use multiple databases.

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

	db := mongo.New(mongo.Config{URI: "mongodb://localhost:27017", Database: "test", ConnectionTimeout: 4 * time.Second})

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