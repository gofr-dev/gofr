# CockroachDB

GoFr provides support for CockroachDB, a cloud-native SQL database that is compatible with PostgreSQL.

## Configuration

To connect to CockroachDB, you need to provide the following environment variables:

*   `DB_DIALECT`: Set to `cockroachdb`
*   `DB_HOST`: The hostname or IP address of your CockroachDB server.
*   `DB_PORT`: The port number (default is 26257).
*   `DB_USER`: The username for connecting to the database.
*   `DB_PASSWORD`: The password for the specified user.
*   `DB_NAME`: The name of the database to connect to.
*   `DB_SSL_MODE`: SSL mode (e.g., `disable`, `require`). CockroachDB Cloud requires SSL.

## Example

```go
package main

import (
	"context"

	"gofr.dev/pkg/gofr"
)

func main() {
	// Create a new GoFr app
	app := gofr.New()
	
	app.GET("/user", GetUser)
	
	app.Run()
}

func GetUser(ctx *gofr.Context)(any, error){
	// Example: Performing a simple query
	rows, err := ctx.SQL.QueryContext(context.Background(), "SELECT 1")
	if err != nil {
		return nil, err
	}
	
	defer rows.Close()
	
	return "Connection to cockroachDB Successful.", nil
}
```
For more detailed examples and advanced usage, please refer to the [SQL usage guide](/advanced-guide/dealing-with-sql/).
