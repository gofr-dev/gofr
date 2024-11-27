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

Import the gofr's external driver for ClickHouse:

```shell
go get gofr.dev/pkg/gofr/datasource/clickhouse@latest
```

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

Import the gofr's external driver for MongoDB:

```shell
go get gofr.dev/pkg/gofr/datasource/mongo@latest
```

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
	
	db := mongo.New(Config{URI: "mongodb://localhost:27017", Database: "test",ConnectionTimeout: 4*time.Second})
	
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

Import the gofr's external driver for Cassandra:

```shell
go get gofr.dev/pkg/gofr/datasource/cassandra@latest
```

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

Import the gofr's external driver for DGraph:

```shell
go get gofr.dev/pkg/gofr/datasource/dgraph@latest
```

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

Import the gofr's external driver for Solr:

```shell
go get gofr.dev/pkg/gofr/datasource/solr@latest
```
Note : This datasource package requires the user to create the collection before performing any operations.
While testing the below code create a collection using :
`curl --location 'http://localhost:2020/solr/admin/collections?action=CREATE&name=test&numShards=2&replicationFactor=1&wt=xml'`

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
	p := []Person{{Name: "Srijan", Age: 24}}
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

## OpenTSDB
GoFr supports injecting OpenTSDB to facilitate interaction with OpenTSDB's REST APIs. 
Implementations adhering to the `OpenTSDB` interface can be registered with `app.AddOpenTSDB()`, 
enabling applications to leverage OpenTSDB for time-series data management through `gofr.Context`.

```go
// OpenTSDB provides methods for GoFr applications to communicate with OpenTSDB
// through its REST APIs. 
type OpenTSDB interface {

// HealthChecker verifies if the OpenTSDB server is reachable.
// Returns an error if the server is unreachable, otherwise nil.
HealthChecker

// PutDataPoints sends data to store metrics in OpenTSDB.
//
// Parameters:
// - ctx: Context for managing request lifetime.
// - data: A slice of DataPoint objects; must contain at least one entry.
// - queryParam: Specifies the response format:
//   - client.PutRespWithSummary: Requests a summary response.
//   - client.PutRespWithDetails: Requests detailed response information.
//   - Empty string (""): No additional response details.
// - res: A pointer to PutResponse, where the server's response will be stored.
//
// Returns:
// - Error if parameters are invalid, response parsing fails, or if connectivity issues occur.
PutDataPoints(ctx context.Context, data any, queryParam string, res any) error

// QueryDataPoints retrieves data based on the specified parameters.
//
// Parameters:
// - ctx: Context for managing request lifetime.
// - param: An instance of QueryParam with query parameters for filtering data.
// - res: A pointer to QueryResponse, where the server's response will be stored.
QueryDataPoints(ctx context.Context, param any, res any) error

// QueryLatestDataPoints fetches the latest data point(s).
//
// Parameters:
// - ctx: Context for managing request lifetime.
// - param: An instance of QueryLastParam with query parameters for the latest data point.
// - res: A pointer to QueryLastResponse, where the server's response will be stored.
QueryLatestDataPoints(ctx context.Context, param any, res any) error

// GetAggregators retrieves available aggregation functions.
//
// Parameters:
// - ctx: Context for managing request lifetime.
// - res: A pointer to AggregatorsResponse, where the server's response will be stored.
GetAggregators(ctx context.Context, res any) error

// QueryAnnotation retrieves a single annotation.
//
// Parameters:
// - ctx: Context for managing request lifetime.
// - queryAnnoParam: A map of parameters for the annotation query, such as client.AnQueryStartTime, client.AnQueryTSUid.
// - res: A pointer to AnnotationResponse, where the server's response will be stored.
QueryAnnotation(ctx context.Context, queryAnnoParam map[string]any, res any) error

// PostAnnotation creates or updates an annotation.
//
// Parameters:
// - ctx: Context for managing request lifetime.
// - annotation: The annotation to be created or updated.
// - res: A pointer to AnnotationResponse, where the server's response will be stored.
PostAnnotation(ctx context.Context, annotation any, res any) error

// PutAnnotation creates or replaces an annotation.
// Fields not included in the request will be reset to default values.
//
// Parameters:
// - ctx: Context for managing request lifetime.
// - annotation: The annotation to be created or replaced.
// - res: A pointer to AnnotationResponse, where the server's response will be stored.
PutAnnotation(ctx context.Context, annotation any, res any) error

// DeleteAnnotation removes an annotation.
//
// Parameters:
// - ctx: Context for managing request lifetime.
// - annotation: The annotation to be deleted.
// - res: A pointer to AnnotationResponse, where the server's response will be stored.
DeleteAnnotation(ctx context.Context, annotation any, res any) error
}
```

Import the gofr's external driver for OpenTSDB:

```go
go get gofr.dev/pkg/gofr/datasource/opentsdb
```

The following example demonstrates injecting an OpenTSDB instance into a GoFr application 
and using it to perform a health check on the OpenTSDB server.
```go
package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/opentsdb"
)

func main() {
	app := gofr.New()

	// Initialize OpenTSDB connection
	app.AddOpenTSDB(opentsdb.New(&opentsdb.Config{
		Host:             "localhost:4242",
		MaxContentLength: 4096,
		MaxPutPointsNum:  1000,
		DetectDeltaNum:   10,
	}))

	// Register routes
	app.GET("/health", opentsdbHealthCheck)
	app.POST("/write", writeDataPoints)
	app.GET("/query", queryDataPoints)
	// Run the app
	app.Run()
}

// Health check for OpenTSDB
func opentsdbHealthCheck(c *gofr.Context) (any, error) {
	res, err := c.OpenTSDB.HealthCheck(context.Background())
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Write Data Points to OpenTSDB
func writeDataPoints(c *gofr.Context) (any, error) {
	PutDataPointNum := 4
	name := []string{"cpu", "disk", "net", "mem"}
	cpuDatas := make([]opentsdb.DataPoint, 0)

	tags := map[string]string{
		"host":      "gofr-host",
		"try-name":  "gofr-sample",
		"demo-name": "opentsdb-test",
	}

	for i := 0; i < PutDataPointNum; i++ {
		data := opentsdb.DataPoint{
			Metric:    name[i%len(name)],
			Timestamp: time.Now().Unix(),
			Value:     rand.Float64() * 100,
			Tags:      tags,
		}
		cpuDatas = append(cpuDatas, data)
	}

	resp := opentsdb.PutResponse{}

	err := c.OpenTSDB.PutDataPoints(context.Background(), cpuDatas, "details", &resp)
	if err != nil {
		return resp.Errors, err
	}

	return fmt.Sprintf("%v Data points written successfully", resp.Success), nil
}

// Query Data Points from OpenTSDB
func queryDataPoints(c *gofr.Context) (any, error) {
	st1 := time.Now().Unix() - 3600
	st2 := time.Now().Unix()

	queryParam := opentsdb.QueryParam{
		Start: st1,
		End:   st2,
	}

	name := []string{"cpu", "disk", "net", "mem"}
	subqueries := make([]opentsdb.SubQuery, 0)
	tags := map[string]string{
		"host":      "gofr-host",
		"try-name":  "gofr-sample",
		"demo-name": "opentsdb-test",
	}

	for _, metric := range name {
		subQuery := opentsdb.SubQuery{
			Aggregator: "sum",
			Metric:     metric,
			Tags:       tags,
		}
		subqueries = append(subqueries, subQuery)
	}

	queryParam.Queries = subqueries

	queryResp := &opentsdb.QueryResponse{}

	err := c.OpenTSDB.QueryDataPoints(c, &queryParam, queryResp)
	if err != nil {
		return nil, err
	}
	return queryResp.QueryRespCnts, nil
}
```