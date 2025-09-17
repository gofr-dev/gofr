<img width="1183" height="1280" alt="image" src="https://github.com/user-attachments/assets/660ba60d-9a01-446a-8d6a-b15d2de05317" /><img width="1183" height="1280" alt="image" src="https://github.com/user-attachments/assets/22c3306e-9f6a-4fc1-837d-c89e76883e11" /># Couchbase

## Configuration

To connect to `Couchbase`, you need to provide the following environment variables and use it:
- `HOST`: The hostname or IP address of your Couchbase server.
- `USER`: The username for connecting to the database.
- `PASSWORD`: The password for the specified user.
- `BUCKET`: Top level container

## Setup

GoFr supports injecting `Couchbase` that implements the following interface. Any driver that implements the interface can be
added using the `app.AddCouchbase()` method, and users can use Couchbase across the application with `gofr.Context`.

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

Users can easily inject a driver that supports this interface, providing usability without compromising the extensibility to use multiple databases.
Don't forget to serup the Couchbase cluster in Couchbase Web Console first. [Follow for more details](https://docs.couchbase.com/server/current/install/getting-started-docker.html#section_jvt_zvj_42b).
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
        Host:     app.Config.Get("HOST"),
        User:     app.Config.Get("USER"),
        Password: app.Config.Get("PASSWORD"),
        Bucket:   app.Config.Get("BUCKET"),
    }))

    // Add the routes
    a.GET("/users/{id}", getUser)
    a.POST("/users", createUser)
	a.DELETE("/users/{id}", deleteUser)

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

func deleteUser(c *gofr.Context) (any, error) {
	// Get the user ID from the URL path
	id := c.PathParam("id")

	// Remove the user from Couchbase
	if err := c.Couchbase.Remove(c, id); err != nil {
		return nil, err
	}

	return "user deleted successfully", nil
}
```
