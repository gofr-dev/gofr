package datasource

type Cassandra interface {
	Query(dest interface{}, stmt string, values ...interface{}) error
	Exec(stmt string, values ...interface{}) error
	QueryCAS(dest interface{}, stmt string, values ...interface{}) (bool, error)
}

type CassandraProvider interface {
	Cassandra

	// UseLogger sets the logger for the MongoDB client.
	UseLogger(logger interface{})

	// UseMetrics sets the metrics for the MongoDB client.
	UseMetrics(metrics interface{})

	// Connect establishes a connection to MongoDB and registers metrics using the provided configuration when the client was Created.
	Connect()
}
