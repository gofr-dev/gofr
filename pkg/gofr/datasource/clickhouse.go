package datasource

import "context"

type Clickhouse interface {
	Exec(ctx context.Context, query string, args ...any) error
	Select(ctx context.Context, dest any, query string, args ...any) error
	AsyncInsert(ctx context.Context, query string, wait bool, args ...any) error
}

type ClickhouseProvider interface {
	Clickhouse

	// UseLogger sets the logger for the Cassandra client.
	UseLogger(logger interface{})

	// UseMetrics sets the metrics for the Cassandra client.
	UseMetrics(metrics interface{})

	// Connect establishes a connection to Cassandra and registers metrics using the provided configuration when the client was Created.
	Connect()
}
