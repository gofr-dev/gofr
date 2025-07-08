## ClickHouse

GoFr supports injecting ClickHouse that supports the following interface. Any driver that implements the interface can be added
using `app.AddClickhouse()` method, and user's can use ClickHouse across application with `gofr.Context`.
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
	Age  int    `ch:"age"`
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

func Post(ctx *gofr.Context) (any, error) {
	err := ctx.Clickhouse.Exec(ctx, "INSERT INTO users (id, name, age) VALUES (?, ?, ?)", "8f165e2d-feef-416c-95f6-913ce3172e15", "aryan", 10)
	if err != nil {
		return nil, err
	}

	return "successfully inserted", nil
}

func Get(ctx *gofr.Context) (any, error) {
	var user []User

	err := ctx.Clickhouse.Select(ctx, &user, "SELECT * FROM users")
	if err != nil {
		return nil, err
	}

	return user, nil
}
```