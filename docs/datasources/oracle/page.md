## OracleDB

GoFr supports injecting OracleDB as a relational datasource through a clean, extensible interface. Any driver that implements the following interface can be added using the `app.AddOracle()` method, and users can access OracleDB throughout their application via `gofr.Context`.

```go
type Oracle interface {
	Exec(ctx context.Context, query string, args ...any) error
	Select(ctx context.Context, dest any, query string, args ...any) error
	AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error
}
```

This approach allows users to easily inject any compatible Oracle driver, providing both usability and the flexibility to use multiple databases in a GoFr application.

### ⚠️ Important: Oracle Database Must Exist

**Before running your GoFr application, you must ensure that the Oracle database and the required schema (such as the `users` table) are already created.**

- Oracle does not allow creating a database (PDB or CDB) via a simple SQL query from a standard client connection.
- You must use Oracle tools (like DBCA, SQL*Plus as SYSDBA, or Docker container initialization) to create the database and pluggable database (PDB) before connecting your app.
- Your application can create tables within an existing schema, but the database itself must be provisioned in advance.

### Import the GoFr External Driver for OracleDB

```shell
go get gofr.dev/pkg/gofr/datasource/oracle@latest
```


### Example

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/oracle"
)

type User struct {
	Id   string `db:"ID"`
	Name string `db:"NAME"`
	Age  int    `db:"AGE"`
}

func main() {
	app := gofr.New()

	app.AddOracle(oracle.New(oracle.Config{
		Host:     "localhost",
		Port:     1521,
		Username: "system",
		Password: "password",
		Service:  "FREEPDB1", // Use your Oracle service/SID
	}))

	app.POST("/user", Post)
	app.GET("/user", Get)

	app.Run()
}

func Post(ctx *gofr.Context) (any, error) {
	err := ctx.Oracle.Exec(ctx, "INSERT INTO users (id, name, age) VALUES (:1, :2, :3)",
		"8f165e2d-feef-416c-95f6-913ce3172e15", "aryan", 10)
	if err != nil {
		return nil, err
	}

	return "successfully inserted", nil
}

func Get(ctx *gofr.Context) (any, error) {
	var users []map[string]interface{}
	err := ctx.Oracle.Select(ctx, &users, "SELECT id, name, age FROM users")
	if err != nil {
		return nil, err
	}
	return users, nil
}
```
