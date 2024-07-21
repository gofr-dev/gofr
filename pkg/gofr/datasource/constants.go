package datasource

// Cassandra batch types
const (
	CassandraLoggedBatch = iota
	CassandraUnloggedBatch
	CassandraCounterBatch
)
