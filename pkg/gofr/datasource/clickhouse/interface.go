// Package clickhouse provides functionalities for interacting with a Clickhouse database.
//
// It contains the Conn interface for executing queries, retrieving data,
// managing asynchronous inserts, and obtaining connection statistics.
package clickhouse

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// Conn defines the interface for interacting with a ClickHouse database.
//
// It abstracts the underlying database operations, allowing clients to perform
// queries, execute statements, perform asynchronous inserts, and retrieve
// connection statistics. This interface is suitable for use in application logic
// and for mocking in tests.
type Conn interface {
	// Select executes a query that returns rows and scans the result into the provided destination.
	//
	// The destination must be a pointer to a slice of structs or another compatible type
	// supported by the ClickHouse driver.
	//
	// ctx controls the lifetime of the query.
	//
	// Returns an error if the query execution or scanning fails.
	Select(ctx context.Context, dest any, query string, args ...any) error

	// Exec executes a query that does not return rows, such as INSERT or UPDATE.
	//
	// ctx controls the lifetime of the query.
	//
	// Returns an error if the execution fails.
	Exec(ctx context.Context, query string, args ...any) error

	// AsyncInsert performs an asynchronous INSERT operation.
	//
	// If wait is true, the method waits for the insert operation to complete before returning.
	// If false, the insert is queued and the method returns immediately.
	//
	// ctx controls the lifetime of the operation.
	//
	// Returns an error if the insert operation fails.
	AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error

	// Ping verifies the connection to the database is still alive.
	//
	// ctx controls the timeout for the ping operation.
	//
	// Returns an error if the connection is unhealthy or the ping fails.
	Ping(context.Context) error

	// Stats returns internal statistics for the ClickHouse driver connection,
	// such as open and idle connections.
	//
	// The returned value is driver.Stats as provided by the ClickHouse driver library.
	Stats() driver.Stats
}
