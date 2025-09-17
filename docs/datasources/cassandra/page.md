# Cassandra
GoFr supports pluggable Cassandra drivers. 

## Configuration
To connect to `Cassandra`, you need to provide the following environment variables:

- `HOSTS`: The hostname or IP address of your Cassandra server.
- `KEYSPACE`: The name of the keyspace (like a database) that holds your tables and defines replication and durability settings.
- `PORT`: The port number
- `USERNAME`: The username for connecting to the database.
- `PASSWORD`: The password for the specified user.


## Setup

GoFr defines an interface that specifies the required methods for interacting
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
	"os"
	"gofr.dev/pkg/gofr"
	cassandraPkg "gofr.dev/pkg/gofr/datasource/cassandra"
)

type Person struct {
	ID   int    `json:"id,omitempty"`
	Name string `json:"name"`
	Age  int    `json:"age"`
	// db tag specifies the actual column name in the database
	State string `json:"state" db:"location"`
}

func main() {
	app := gofr.New()

	config := cassandraPkg.Config{
		Hosts:    app.Config.Get("HOSTS"),
		Keyspace: app.Config.Get("KEYSPACE"),
		Port:     app.Config.Get("PORT"),
		Username: app.Config.Get("USERNAME"),
		Password: app.Config.Get("PASSWORD"),
	}

	cassandra := cassandraPkg.New(config)

	app.AddCassandra(cassandra)

	app.POST("/user", func(c *gofr.Context) (any, error) {
		person := Person{}

		err := c.Bind(&person)
		if err != nil {
			return nil, err
		}

		err = c.Cassandra.ExecWithCtx(c, `INSERT INTO persons(id, name, age, location) VALUES(?, ?, ?, ?)`,
			person.ID, person.Name, person.Age, person.State)
		if err != nil {
			return nil, err
		}

		return "created", nil
	})

	app.GET("/user", func(c *gofr.Context) (any, error) {
		persons := make([]Person, 0)

		err := c.Cassandra.QueryWithCtx(c, &persons, `SELECT id, name, age, location FROM persons`)

		return persons, err
	})

	app.Run()
}
```
