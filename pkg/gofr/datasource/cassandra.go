package datasource

type Cassandra interface {
	// Query executes the query and binds the result into dest parameter.
	// Returns error if any error occurs while binding the result.
	// Can be used to single as well as multiple rows.
	// Accepts pointer to struct or slice as dest parameter for single and multiple rows retrieval respectively
	Query(dest interface{}, stmt string, values ...interface{}) error

	// Exec executes the query without returning any rows.
	// Return error if any error occurs while executing the query
	// Can be used to execute UPDATE or INSERT
	Exec(stmt string, values ...interface{}) error

	// QueryCAS executes a lightweight transaction (i.e. an UPDATE or INSERT statement containing an IF clause).
	// If the transaction fails because the existing values did not match, the previous values will be stored in dest.
	// Returns true if the query is applied otherwise returns false
	// Returns and error if any error occur while executing the query
	// Accepts only pointer to struct and built-in types as the dest parameter.
	QueryCAS(dest interface{}, stmt string, values ...interface{}) (bool, error)
}

type CassandraProvider interface {
	Cassandra

	// UseLogger sets the logger for the Cassandra client.
	UseLogger(logger interface{})

	// UseMetrics sets the metrics for the Cassandra client.
	UseMetrics(metrics interface{})

	// Connect establishes a connection to Cassandra and registers metrics using the provided configuration when the client was Created.
	Connect()
}
