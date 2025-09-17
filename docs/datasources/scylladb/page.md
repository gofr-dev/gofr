# ScyllaDB

## Configuration
To connect to `ScyllaDB`, you need to provide the following environment variables:
- `HOST`: The hostname or IP address of your ScyllaDB server.
- `KEYSPACE`: The top level namespace.
- `PORT`: The port number.
- `USERNAME`: The username for connecting to the database.
- `PASSWORD`: The password for the specified user.

## Setup
GoFr supports pluggable ScyllaDB drivers. It defines an interface that specifies the required methods for interacting
with ScyllaDB. Any driver implementation that adheres to this interface can be integrated into GoFr using the
`app.AddScyllaDB()` method.

```go
type ScyllaDB interface {
	// Query executes a CQL (Cassandra Query Language) query on the ScyllaDB cluster
	// and stores the result in the provided destination variable `dest`.
	// Accepts pointer to struct or slice as dest parameter for single and multiple
	Query(dest any, stmt string, values ...any) error
	// QueryWithCtx executes the query with a context and binds the result into dest parameter.
	// Accepts pointer to struct or slice as dest parameter for single and multiple rows retrieval respectively.
	QueryWithCtx(ctx context.Context, dest any, stmt string, values ...any) error
	// Exec executes a CQL statement (e.g., INSERT, UPDATE, DELETE) on the ScyllaDB cluster without returning any result.
	Exec(stmt string, values ...any) error
	// ExecWithCtx executes a CQL statement with the provided context and without returning any result.
	ExecWithCtx(ctx context.Context, stmt string, values ...any) error
	// ExecCAS executes a lightweight transaction (i.e. an UPDATE or INSERT statement containing an IF clause).
	// If the transaction fails because the existing values did not match, the previous values will be stored in dest.
	// Returns true if the query is applied otherwise false.
	// Returns false and error if any error occur while executing the query.
	// Accepts only pointer to struct and built-in types as the dest parameter.
	ExecCAS(dest any, stmt string, values ...any) (bool, error)
	// NewBatch initializes a new batch operation with the specified name and batch type.
	NewBatch(name string, batchType int) error
	// NewBatchWithCtx takes context,name and batchtype and return error.
	NewBatchWithCtx(_ context.Context, name string, batchType int) error
	// BatchQuery executes a batch query in the ScyllaDB cluster with the specified name, statement, and values.
	BatchQuery(name, stmt string, values ...any) error
	// BatchQueryWithCtx executes a batch query with the provided context.
	BatchQueryWithCtx(ctx context.Context, name, stmt string, values ...any) error
	// ExecuteBatchWithCtx executes a batch with context and name returns error.
	ExecuteBatchWithCtx(ctx context.Context, name string) error
	// HealthChecker defines the HealthChecker interface.
	HealthChecker
}
```


Import the gofr's external driver for ScyllaDB:

```shell
go get gofr.dev/pkg/gofr/datasource/scylladb
```

```go
package main

import (
	"github.com/gocql/gocql"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/scylladb"
	"gofr.dev/pkg/gofr/http"
)

type User struct {
	ID    gocql.UUID `json:"id"`
	Name  string     `json:"name"`
	Email string     `json:"email"`
}

func main() {
	app := gofr.New()

	client := scylladb.New(scylladb.Config{
		Host:     app.Config.Get("HOST"),
		Keyspace: app.Config.Get("KEYSPACE"),
		Port:     app.Config.Get("PORT"),
		Username: app.Config.Get("USERNAME"),
		Password: app.Config.Get("PASSWORD"),
	})

	app.AddScyllaDB(client)

	app.GET("/users/{id}", getUser)
	app.POST("/users", addUser)

	app.Run()
}

func addUser(c *gofr.Context) (any, error) {
	var newUser User
	err := c.Bind(&newUser)
	if err != nil {
		return nil, err
	}
	_ = c.ScyllaDB.ExecWithCtx(c, `INSERT INTO users (user_id, username, email) VALUES (?, ?, ?)`, newUser.ID, newUser.Name, newUser.Email)

	return newUser, nil
}

func getUser(c *gofr.Context) (any, error) {
	var user User
	id := c.PathParam("id")

	userID, err := gocql.ParseUUID(id)
	if err != nil {
		c.Logger.Error("Invalid UUID format:", err)
		return nil, err
	}

	err = c.ScyllaDB.QueryWithCtx(c, &user, "SELECT id, name, email FROM users WHERE id = ?", userID)
	if err != nil {
		c.Logger.Error("Error querying user:", err)
		return nil, err
	}

	return user, nil
}
```
