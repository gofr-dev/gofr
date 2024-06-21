# Injecting Database Drivers
Keeping in mind the size of the framework in the final build, it felt counter-productive to keep all the database drivers within
the framework itself. Keeping only the most used MySQL and Redis within the framework, users can now inject databases
in the server that satisfies the base interface defined by GoFr. This helps in reducing the build size and in turn build time
as unnecessary database drivers are not being compiled and added to the build.

> We are planning to provide custom drivers for most common databases, and is in the pipeline for upcoming releases!


## Clickhouse
GoFr supports injecting Clickhouse that supports the following interface. Any driver that implements the interface can be added
using `app.AddClickhouse()` method, and user's can use Clickhouse across application with `gofr.Context`.
```go
type Clickhouse interface {
    Exec(ctx context.Context, query string, args ...any) error
    Select(ctx context.Context, dest any, query string, args ...any) error
    AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error
}
```

User's can easily inject a driver that supports this interface, this provides usability without
compromising the extensibility to use multiple databases.
### Example
```go
package main

import (
    "gofr.dev/pkg/gofr"

    "gofr.dev/pkg/gofr/datasource/clickhouse"
)

type User struct {
    Id   string `ch:"id"`
    Name string `ch:"name"`
    Age  string `ch:"age"`
}

func main() {
    app := gofr.New()

    app.AddClickhouse(clickhouse.New(clickhouse.Config{
        Hosts:    "localhost:9001",
        Username: "root",
        Password: "password",
        Database: "users",
    }))
    
    app.POST("/user", Post)
    app.GET("/user", Get)
    
    app.Run()
}

func Post(ctx *gofr.Context) (interface{}, error) {
    err := ctx.Clickhouse.Exec(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)", "8f165e2d-feef-416c-95f6-913ce3172e15", "aryan", "10")
    if err != nil {
        return nil, err
    }

    return "successful inserted", nil
}

func Get(ctx *gofr.Context) (interface{}, error) {
    var user []User

    err := ctx.Clickhouse.Select(ctx, &user, "SELECT * FROM users")
    if err != nil {
        return nil, err
    }

    return user, nil
}
```

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

## Cassandra
GoFr supports pluggable Cassandra drivers. It defines an interface that specifies the required methods for interacting 
with Cassandra. Any driver implementation that adheres to this interface can be integrated into GoFr using the 
`app.AddCassandra()` method. This approach promotes flexibility and allows you to choose the Cassandra driver that best 
suits your project's needs.

```go
type Cassandra interface {
	Query(dest interface{}, stmt string, values ...interface{}) error

	Exec(stmt string, values ...interface{}) error
	
	ExecCAS(dest interface{}, stmt string, values ...interface{}) (bool, error)
}
```

GoFr simplifies Cassandra integration with a well-defined interface. Users can easily implement any driver that adheres 
to this interface, fostering a user-friendly experience.

### Example

```go
package main

import (
	"gofr.dev/pkg/gofr"
	cassandraPkg "gofr.dev/pkg/gofr/datasource/cassandra"
)

type Person struct {
	ID    int    `json:"id,omitempty"`
	Name  string `json:"name"`
	Age   int    `json:"age"`
	State string `json:"state"`
}

func main() {
	app := gofr.New()

	config := cassandraPkg.Config{
		Hosts:    "localhost",
		Keyspace: "test",
		Port:     2003,
		Username: "cassandra",
		Password: "cassandra",
	}

	cassandra := cassandraPkg.New(config)

	app.AddCassandra(cassandra)

	app.POST("/user", func(c *gofr.Context) (interface{}, error) {
		person := Person{}

		err := c.Bind(&person)
		if err != nil {
			return nil, err
		}

		err = c.Cassandra.Exec(`INSERT INTO persons(id, name, age, state) VALUES(?, ?, ?, ?)`,
			person.ID, person.Name, person.Age, person.State)
		if err != nil {
			return nil, err
		}

		return "created", nil
	})

	app.GET("/user", func(c *gofr.Context) (interface{}, error) {
		persons := make([]Person, 0)

		err := c.Cassandra.Query(&persons, `SELECT id, name, age, state FROM persons`)

		return persons, err
	})

	app.Run()
}
```
