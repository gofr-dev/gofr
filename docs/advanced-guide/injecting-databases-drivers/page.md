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
type CassandraWithContext interface {
	QueryWithCtx(ctx context.Context, dest any, stmt string, values ...any) error

	ExecWithCtx(ctx context.Context, stmt string, values ...any) error

	ExecCASWithCtx(ctx context.Context, dest any, stmt string, values ...any) (bool, error)

	NewBatchWithCtx(ctx context.Context, name string, batchType int) error

	Cassandra
	CassandraBatchWithContext
}

type CassandraBatchWithContext interface {
	BatchQueryWithCtx(ctx context.Context, name, stmt string, values ...any) error

	ExecuteBatchWithCtx(ctx context.Context, name string) error

	ExecuteBatchCASWithCtx(ctx context.Context, name string, dest ...any) (bool, error)
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
        // db tag specifies the actual column name in the database
	State string `json:"state" db:"location"` 
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

		err = c.Cassandra.ExecWithCtx(c,`INSERT INTO persons(id, name, age, location) VALUES(?, ?, ?, ?)`,
			person.ID, person.Name, person.Age, person.State)
		if err != nil {
			return nil, err
		}

		return "created", nil
	})

	app.GET("/user", func(c *gofr.Context) (interface{}, error) {
		persons := make([]Person, 0)

		err := c.Cassandra.QueryWithCtx(c, &persons, `SELECT id, name, age, location FROM persons`)

		return persons, err
	})

	app.Run()
}
```
## DGraph
GoFr supports injecting Dgraph with an interface that defines the necessary methods for interacting with the Dgraph 
database. Any driver that implements the following interface can be added using the app.AddDgraph() method.

```go
// Dgraph defines the methods for interacting with a Dgraph database.
type Dgraph interface {
	// Query executes a read-only query in the Dgraph database and returns the result.
	Query(ctx context.Context, query string) (interface{}, error)

	// QueryWithVars executes a read-only query with variables in the Dgraph database.
	QueryWithVars(ctx context.Context, query string, vars map[string]string) (interface{}, error)

	// Mutate executes a write operation (mutation) in the Dgraph database and returns the result.
	Mutate(ctx context.Context, mu interface{}) (interface{}, error)

	// Alter applies schema or other changes to the Dgraph database.
	Alter(ctx context.Context, op interface{}) error

	// NewTxn creates a new transaction (read-write) for interacting with the Dgraph database.
	NewTxn() interface{}

	// NewReadOnlyTxn creates a new read-only transaction for querying the Dgraph database.
	NewReadOnlyTxn() interface{}

	// HealthChecker checks the health of the Dgraph instance.
	HealthChecker
}
```

Users can easily inject a driver that supports this interface, allowing for flexibility without compromising usability.
This structure supports both queries and mutations in Dgraph.

### Example

```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/dgraph-io/dgo/v210/protos/api"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/dgraph"
)

func main() {
	// Create a new application
	app := gofr.New()

	db := dgraph.New(dgraph.Config{
		Host: "localhost",
		Port: "8080",
	})

	// Connect to Dgraph running on localhost:9080
	app.AddDgraph(db)

	// Add routes for Dgraph operations
	app.POST("/dgraph", DGraphInsertHandler)
	app.GET("/dgraph", DGraphQueryHandler)

	// Run the application
	app.Run()
}

// DGraphInsertHandler handles POST requests to insert data into Dgraph
func DGraphInsertHandler(c *gofr.Context) (interface{}, error) {
	// Example mutation data to insert into Dgraph
	mutationData := `
		{
			"set": [
				{
					"name": "GoFr Dev"
				},
				{
					"name": "James Doe"
				}
			]
		}
	`

	// Create an api.Mutation object
	mutation := &api.Mutation{
		SetJson:   []byte(mutationData), // Set the JSON payload
		CommitNow: true,                 // Auto-commit the transaction
	}

	// Run the mutation in Dgraph
	response, err := c.DGraph.Mutate(c, mutation)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// DGraphQueryHandler handles GET requests to fetch data from Dgraph
func DGraphQueryHandler(c *gofr.Context) (interface{}, error) {
	// A simple query to fetch all persons with a name in Dgraph
	response, err := c.DGraph.Query(c, "{ persons(func: has(name)) { uid name } }")
	if err != nil {
		return nil, err
	}

	// Cast response to *api.Response (the correct type returned by Dgraph Query)
	resp, ok := response.(*api.Response)
	if !ok {
		return nil, fmt.Errorf("unexpected response type")
	}

	// Parse the response JSON
	var result map[string]interface{}
	err = json.Unmarshal(resp.Json, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

```




## Solr
GoFr supports injecting Solr database that supports the following interface. Any driver that implements the interface can be added
using `app.AddSolr()` method, and user's can use Solr DB across application with `gofr.Context`.

```go
type Solr interface {
    Search(ctx context.Context, collection string, params map[string]any) (any, error)
    Create(ctx context.Context, collection string, document *bytes.Buffer, params map[string]any) (any, error)
    Update(ctx context.Context, collection string, document *bytes.Buffer, params map[string]any) (any, error)
    Delete(ctx context.Context, collection string, document *bytes.Buffer, params map[string]any) (any, error)
    
    Retrieve(ctx context.Context, collection string, params map[string]any) (any, error)
    ListFields(ctx context.Context, collection string, params map[string]any) (any, error)
    AddField(ctx context.Context, collection string, document *bytes.Buffer) (any, error)
    UpdateField(ctx context.Context, collection string, document *bytes.Buffer) (any, error)
    DeleteField(ctx context.Context, collection string, document *bytes.Buffer) (any, error)
}
```

User's can easily inject a driver that supports this interface, this provides usability
without compromising the extensibility to use multiple databases.

```go
package main

import (
	"bytes"
	"encoding/json"
	"errors"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/solr"
)

func main() {
	app := gofr.New()

	app.AddSolr(solr.New(solr.Config{
		Host: "localhost",
		Port: "2020",
	}))

	app.POST("/solr", post)
	app.GET("/solr", get)

	app.Run()
}

type Person struct {
	Name string
	Age  int
}

func post(c *gofr.Context) (interface{}, error) {
	p := Person{Name: "Srijan", Age: 24}
	body, _ := json.Marshal(p)

	resp, err := c.Solr.Create(c, "test", bytes.NewBuffer(body), nil)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func get(c *gofr.Context) (interface{}, error) {
	resp, err := c.Solr.Search(c, "test", nil)
	if err != nil {
		return nil, err
	}

	res, ok := resp.(solr.Response)
	if !ok {
		return nil, errors.New("invalid response type")
	}

	b, _ := json.Marshal(res.Data)
	err = json.Unmarshal(b, &Person{})
	if err != nil {
		return nil, err
	}

	return resp, nil
}
```
