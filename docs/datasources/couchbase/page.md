## Couchbase

GoFr provides first-class support for Couchbase, one of the leading NoSQL databases in the industry. This integration allows developers to seamlessly connect their applications with Couchbase and leverage its powerful features for data management.

The Couchbase interface in GoFr is designed to be intuitive and easy to use, abstracting away the complexities of the underlying driver. This allows developers to focus on their application logic without worrying about the boilerplate code for database interactions.

```go
type Couchbase interface {
    Get(ctx context.Context, key string, result any) error

    Insert(ctx context.Context, key string, document, result any) error

    Upsert(ctx context.Context, key string, document any, result any) error

    Remove(ctx context.Context, key string) error

    Query(ctx context.Context, statement string, params map[string]any, result any) error

    AnalyticsQuery(ctx context.Context, statement string, params map[string]any, result any) error
}
```

To begin using Couchbase in your GoFr application, you need to import the Couchbase datasource package:

```shell
go get gofr.dev/pkg/gofr/datasource/couchbase@latest
```

### Example

Here is an example of how to use the Couchbase datasource in a GoFr application:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/datasource/couchbase"
)

type User struct {
    ID   string `json:"id"`
    Name string `json:"name"`
    Age  int    `json:"age"`
}

func main() {
    // Create a new GoFr application
    a := gofr.New()

    // Add the Couchbase datasource to the application
    a.AddCouchbase(couchbase.New(&couchbase.Config{
        Host:     "localhost",
        User:     "Administrator",
        Password: "password",
        Bucket:   "test-bucket",
    }))

    // Add the routes
    a.GET("/users/{id}", getUser)
    a.POST("/users", createUser)

    // Run the application
    a.Run()
}

func getUser(c *gofr.Context) (any, error) {
    // Get the user ID from the URL path
    id := c.PathParam("id")

    // Get the user from Couchbase
    var user User
    if err := c.Couchbase.Get(c, id, &user); err != nil {
        return nil, err
    }

    return user, nil
}

func createUser(c *gofr.Context) (any, error) {
    // Get the user from the request body
    var user User
    if err := c.Bind(&user); err != nil {
        return nil, err
    }

    // Insert the user into Couchbase
    if err := c.Couchbase.Insert(c, user.ID, user, nil); err != nil {
        return nil, err
    }

    return "user created successfully", nil
}
```
