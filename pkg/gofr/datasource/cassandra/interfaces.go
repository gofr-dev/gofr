package cassandra

import (
	"github.com/gocql/gocql"
)

//go:generate mockgen -source=interfaces.go -destination=mock_interfaces.go -package=cassandra

// All interfaces is designed to be mockable for unit testing purposes, allowing you to control the behavior of Cassandra
// interactions during tests.

// clusterConfig defines methods for interacting with a Cassandra clusterConfig.
type clusterConfig interface {
	createSession() (session, error)
}

// session defines methods for interacting with a Cassandra session.
type session interface {
	query(stmt string, values ...any) query
	newBatch(batchtype gocql.BatchType) batch
	executeBatch(batch batch) error
	executeBatchCAS(b batch, dest ...any) (bool, error)
}

// query defines methods for interacting with a Cassandra query.
type query interface {
	exec() error
	iter() iterator
	mapScanCAS(dest map[string]any) (applied bool, err error)
	scanCAS(dest ...any) (applied bool, err error)
}

// batch defines methods for interacting with a Cassandra batch.
type batch interface {
	Query(stmt string, args ...any)
	getBatch() *gocql.Batch
}

// iterator defines methods for interacting with a Cassandra iterator.
type iterator interface {
	columns() []gocql.ColumnInfo
	scan(dest ...any) bool
	numRows() int
}
