package cassandra

import "github.com/gocql/gocql"

// clusterConfig defines methods for interacting with a Cassandra clusterConfig. This interface
// is designed to be mockable for unit testing purposes, allowing you to control the behavior
// of Cassandra interactions during tests.
type clusterConfig interface {
	createSession() (session, error)
}

// session defines methods for interacting with a Cassandra session. This interface
// is designed to be mockable for unit testing purposes, allowing you to control the behavior
// of Cassandra interactions during tests.
type session interface {
	query(stmt string, values ...interface{}) query
}

// query defines methods for interacting with a Cassandra query. This interface
// is designed to be mockable for unit testing purposes, allowing you to control the behavior
// of Cassandra interactions during tests.
type query interface {
	exec() error
	iter() iterator
	mapScanCAS(dest map[string]interface{}) (applied bool, err error)
	scanCAS(dest ...any) (applied bool, err error)
}

// iterator defines methods for interacting with a Cassandra iterator. This interface
// is designed to be mockable for unit testing purposes, allowing you to control the behavior
// of Cassandra interactions during tests.
type iterator interface {
	columns() []gocql.ColumnInfo
	scan(dest ...interface{}) bool
	numRows() int
}
