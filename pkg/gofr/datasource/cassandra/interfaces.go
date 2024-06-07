package cassandra

import "github.com/gocql/gocql"

// All interfaces is designed to be mockable for unit testing purposes, allowing you to control the behavior of Cassandra
// interactions during tests.

// clusterConfig defines methods for interacting with a Cassandra clusterConfig.
type clusterConfig interface {
	createSession() (session, error)
}

// session defines methods for interacting with a Cassandra session.
type session interface {
	query(stmt string, values ...interface{}) query
}

// query defines methods for interacting with a Cassandra query.
type query interface {
	exec() error
	iter() iterator
	mapScanCAS(dest map[string]interface{}) (applied bool, err error)
	scanCAS(dest ...any) (applied bool, err error)
}

// iterator defines methods for interacting with a Cassandra iterator.
type iterator interface {
	columns() []gocql.ColumnInfo
	scan(dest ...interface{}) bool
	numRows() int
}
