## ScyllaDB

GoFr supports pluggable ScyllaDB drivers. It defines an interface specifying required methods for interacting with ScyllaDB. Any driver implementation that adheres to this interface can be integrated into GoFr using the `app.AddScyllaDB()` method.

```go
// ScyllaDB defines methods required for interacting with a ScyllaDB cluster.
type ScyllaDB interface {
    // Query executes a CQL (Cassandra Query Language) query and stores the result in dest.
    // dest should be a pointer to a struct (for a single row) or a slice (for multiple rows).
    Query(dest any, stmt string, values ...any) error

    // QueryWithCtx executes a query with context and binds the result into dest.
    QueryWithCtx(ctx context.Context, dest any, stmt string, values ...any) error

    // Exec executes a CQL statement (INSERT, UPDATE, DELETE) without returning any result.
    Exec(stmt string, values ...any) error

    // ExecWithCtx executes a CQL statement with the provided context.
    ExecWithCtx(ctx context.Context, stmt string, values ...any) error

    // ExecCAS executes a lightweight transaction (LWT) with an IF clause.
    // If not applied, previous values will be stored in dest.
    // Returns true if applied, false otherwise.
    ExecCAS(dest any, stmt string, values ...any) (bool, error)

    // NewBatch initializes a new batch operation.
    NewBatch(name string, batchType int) error

    // NewBatchWithCtx takes context, batch type, and returns an error.
    NewBatchWithCtx(ctx context.Context, name string, batchType int) error

    // BatchQuery executes a batch query with the specified name.
    BatchQuery(name, stmt string, values ...any) error

    // BatchQueryWithCtx executes a batch query with the provided context.
    BatchQueryWithCtx(ctx context.Context, name, stmt string, values ...any) error

    // ExecuteBatchWithCtx executes a batch with context and name.
    ExecuteBatchWithCtx(ctx context.Context, name string) error

    // HealthChecker defines the HealthChecker interface.
    HealthChecker
}
```

#### Install the GoFr ScyllaDB Driver

```shell
go get gofr.dev/pkg/gofr/datasource/scylladb
```

#### Examples Usage

```go
package main

import (
    "context"
    "github.com/gocql/gocql"

    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/datasource/scylladb"
    "gofr.dev/pkg/gofr/http"
)

// User represents a user record.
type User struct {
    ID    gocql.UUID `json:"id"`
    Name  string     `json:"name"`
    Email string     `json:"email"`
}

func main() {
    app := gofr.New()

    client := scylladb.New(scylladb.Config{
        Host:     "localhost",
        Keyspace: "my_keyspace",
        Port:     2025,
        Username: "root",
        Password: "password",
    })

    app.AddScyllaDB(client)

    app.GET("/users/{id}", getUser)
    app.POST("/users", addUser)

    app.Run()
}

// addUser handles POST /users and inserts a new user into the database.
func addUser(c *gofr.Context) (any, error) {
    var newUser User
    if err := c.Bind(&newUser); err != nil {
        return nil, err
    }
    // Use consistent field names (`id`, `name`, `email`) in the CQL statement.
    err := c.ScyllaDB.ExecWithCtx(c, `INSERT INTO users (id, name, email) VALUES (?, ?, ?)`, newUser.ID, newUser.Name, newUser.Email)
    if err != nil {
        c.Logger.Error("Error inserting user:", err)
        return nil, err
    }
    return newUser, nil
}

// getUser handles GET /users/{id} and fetches a user by ID.
func getUser(c *gofr.Context) (any, error) {
    var user User
    id := c.PathParam("id")

    userID, err := gocql.ParseUUID(id)
    if err != nil {
        c.Logger.Error("Invalid UUID format:", err)
        return nil, err
    }

    // Use consistent field names in the CQL statement.
    err = c.ScyllaDB.QueryWithCtx(c, &user, "SELECT id, name, email FROM users WHERE id = ?", userID)
    if err != nil {
        c.Logger.Error("Error querying user:", err)
        return nil, err
    }

    return user, nil
}
```

---

### Batch Operations

For advanced use cases, ScyllaDB supports batch operations to execute multiple CQL statements atomically. Use the `NewBatch`, `BatchQuery`, and `ExecuteBatchWithCtx` methods to create and execute batches. Batch operations are useful for inserting/updating multiple rows efficiently, but should be used judiciously to avoid performance issues.

Example (pseudo-code):

```go
_ = c.ScyllaDB.NewBatch("batch1", 0) // 0 = logged batch
_ = c.ScyllaDB.BatchQuery("batch1", "INSERT INTO users (id, name, email) VALUES (?, ?, ?)", id1, name1, email1)
_ = c.ScyllaDB.BatchQuery("batch1", "INSERT INTO users (id, name, email) VALUES (?, ?, ?)", id2, name2, email2)
err := c.ScyllaDB.ExecuteBatchWithCtx(c, "batch1")
if err != nil {
    c.Logger.Error("Error executing batch:", err)
}
```

---

> **Note:** Ensure naming conventions and field usage (`id`, `name`, `email`) align with your ScyllaDB schema.
> Always check for errors when executing queries and handle them appropriately for robust applications.
