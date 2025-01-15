// Package clickhouse provides functionalities for interacting with a Clickhouse database.
//
// It contains the Conn interface for executing queries, retrieving data,
// managing asynchronous inserts, and obtaining connection statistics.
package clickhouse

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Conn defines the interface for interacting with a Clickhouse database.
//
// This interface includes methods for executing queries, retrieving data,
// performing asynchronous inserts, checking connection health, and obtaining
// connection statistics.
type Conn interface {
	// Select retrieves data from the Clickhouse database and populates it into `dest`.
	//
	// Parameters:
	//   - ctx: The context for managing the request lifecycle.
	//   - dest: The destination where the retrieved data will be stored.
	//   - query: The SQL query to execute.
	//   - args: Optional parameters for the query.
	//
	// Returns:
	//   - error: An error if data retrieval fails.
	Select(ctx context.Context, dest any, query string, args ...any) error

	// Exec executes a SQL query on the Clickhouse database.
	//
	// Parameters:
	//   - ctx: The context for managing the request lifecycle.
	//   - query: The SQL query to execute.
	//   - args: Optional parameters for the query.
	//
	// Returns:
	//   - error: An error if query execution fails.
	Exec(ctx context.Context, query string, args ...any) error

	// AsyncInsert performs an asynchronous insert into the Clickhouse database.
	//
	// Parameters:
	//   - ctx: The context for managing the request lifecycle.
	//   - query: The SQL query for the insert.
	//   - wait: A boolean indicating if the method should wait for the insert to complete.
	//   - args: Optional parameters for the query.
	//
	// Returns:
	//   - error: An error if the asynchronous insert fails.
	AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error

	// Ping checks the health of the Clickhouse database connection.
	//
	// Parameters:
	//   - ctx: The context for managing the request lifecycle.
	//
	// Returns:
	//   - error: An error if the connection is not healthy.
	Ping(ctx context.Context) error

	// Stats retrieves statistics about the current Clickhouse connection.
	//
	// Returns:
	//   - driver.Stats: Connection statistics, including information like
	//     open connections and available resources.
	Stats() driver.Stats
}
