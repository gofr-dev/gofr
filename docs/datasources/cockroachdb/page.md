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
	"fmt"

	"gofr.dev/pkg/gofr"
)

func main() {
	// Create a new GoFr app
	app := gofr.New()

	// Example: Performing a simple query
	rows, err := app.SQL.QueryContext(context.Background(), "SELECT 1")
	if err != nil {
		fmt.Println("Error querying database:", err)
		return
	}
	defer rows.Close()

	fmt.Println("Successfully connected to CockroachDB and executed query.")

	// Start the server (optional, if you only need DB interaction)
	// app.Server.Start()
}
```
For more detailed examples and advanced usage, please refer to the [SQL usage guide](/advanced-guide/dealing-with-sql/).
