# SurrealDB

## Configuration
To connect to `SurrealDB`, you need to provide the following environment variables:
- `HOST`: The hostname or IP address of your SurrealDB server.
- `PORT`: The port number.
- `USERNAME`: The username for connecting to the database.
- `PASSWORD`: The password for the specified user.
- `NAMESPACE`: Top level container in SurrealDB that groups databases.
- `DATABASE`: The name of the database to connect to.
- `TLSENABLED`: TLS mode (e.g., disable, require)

## Setup 
GoFr supports injecting SurrealDB database that supports the following interface. Any driver that implements the interface can be added
using `app.AddSurrealDB()` method, and users can use Surreal DB across application through the `gofr.Context`.

```go
// SurrealDB defines an interface representing a SurrealDB client with common database operations.
type SurrealDB interface {
    // Query executes a Surreal query with the provided variables and returns the query results as a slice of interfaces{}.
    // It returns an error if the query execution fails.
    Query(ctx context.Context, query string, vars map[string]any) ([]any, error)

    // Create inserts a new record into the specified table and returns the created record as a map.
    // It returns an error if the operation fails.
    Create(ctx context.Context, table string, data any) (map[string]any, error)

    // Update modifies an existing record in the specified table by its ID with the provided data.
    // It returns the updated record as an interface and an error if the operation fails.
    Update(ctx context.Context, table string, id string, data any) (any, error)

    // Delete removes a record from the specified table by its ID.
    // It returns the result of the delete operation as an interface and an error if the operation fails.
    Delete(ctx context.Context, table string, id string) (any, error)

    // Select retrieves all records from the specified table.
    // It returns a slice of maps representing the records and an error if the operation fails.
    Select(ctx context.Context, table string) ([]map[string]any, error)

    HealthChecker
}

// SurrealDBProvider is an interface that extends SurrealDB with additional methods for logging, metrics, or connection management.
// It is typically used for initializing and managing SurrealDB-based data sources.
type SurrealDBProvider interface {
    SurrealDB

    provider
}
```
Import the gofr's external driver for SurrealDB:
```shell
  go get gofr.dev/pkg/gofr/datasource/surrealdb
```
The following example demonstrates injecting an SurrealDB instance into a GoFr application.

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/surrealdb"
	"os"
)

type Person struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email,omitempty"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

func main() {
	app := gofr.New()

	client := surrealdb.New(&surrealdb.Config{
		Host:       os.Getenv("HOST"),
		Port:       os.Getenv("PORT"),
		Username:   os.Getenv("USERNAME"),
		Password:   os.Getenv("PASSWORD"),
		Namespace:  os.Getenv("NAMESPACE"),
		Database:   os.Getenv("DATABASE"),
		TLSEnabled: os.Getenv("TLSENABLED"),
	})

	app.AddSurrealDB(client)

	// GET request to fetch person by ID
	app.GET("/person/{id}", func(ctx *gofr.Context) (any, error) {
		id := ctx.PathParam("id")

		query := "SELECT * FROM type::thing('person', $id)"
		vars := map[string]any{
			"id": id,
		}

		result, err := ctx.SurrealDB.Query(ctx, query, vars)
		if err != nil {
			return nil, err
		}

		return result, nil
	})

	// POST request to create a new person
	app.POST("/person", func(ctx *gofr.Context) (any, error) {
		var person Person

		if err := ctx.Bind(&person); err != nil {
			return ErrorResponse{Message: "Invalid request body"}, nil
		}

		result, err := ctx.SurrealDB.Create(ctx, "person", map[string]any{
			"name":  person.Name,
			"age":   person.Age,
			"email": person.Email,
		})

		if err != nil {
			return nil, err
		}

		return result, nil
	})

	app.Run()
}

```
