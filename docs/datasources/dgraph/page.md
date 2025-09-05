# Dgraph

## Configuration
To connect to `Dgraph`, you need to provide the following environment variables and use it:
- `HOST`: The hostname or IP address of your Dgraph server.
- `PORT`: The port number.

## Setup
GoFr supports injecting Dgraph with an interface that defines the necessary methods for interacting with the Dgraph
database. Any driver that implements the following interface can be added using the app.AddDgraph() method.

```go
// Dgraph defines the methods for interacting with a Dgraph database.
type Dgraph interface {
    // ApplySchema applies or updates the complete database schema.
    ApplySchema(ctx context.Context, schema string) error

    // AddOrUpdateField atomically creates or updates a single field definition.
    AddOrUpdateField(ctx context.Context, fieldName, fieldType, directives string) error

    // DropField permanently removes a field/predicate and all its associated data.
    DropField(ctx context.Context, fieldName string) error

	// Query executes a read-only query in the Dgraph database and returns the result.
	Query(ctx context.Context, query string) (any, error)

	// QueryWithVars executes a read-only query with variables in the Dgraph database.
	QueryWithVars(ctx context.Context, query string, vars map[string]string) (any, error)

	// Mutate executes a write operation (mutation) in the Dgraph database and returns the result.
	Mutate(ctx context.Context, mu any) (any, error)

	// Alter applies schema or other changes to the Dgraph database.
	Alter(ctx context.Context, op any) error

	// NewTxn creates a new transaction (read-write) for interacting with the Dgraph database.
	NewTxn() any

	// NewReadOnlyTxn creates a new read-only transaction for querying the Dgraph database.
	NewReadOnlyTxn() any

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
		Host: app.Config.Get("HOST"),
		Port: app.Config.Get("PORT"),
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
func DGraphInsertHandler(c *gofr.Context) (any, error) {
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
func DGraphQueryHandler(c *gofr.Context) (any, error) {
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
	var result map[string]any
	err = json.Unmarshal(resp.Json, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
```
