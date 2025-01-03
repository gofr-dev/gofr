package scylladb

import (
	"github.com/gocql/gocql"
)

// clusterConfig defines methods for interacting with a ScyllaDB clusterConfig.
type clusterConfig interface {
	createSession() (session, error)
}

// iterator defines methods for interacting with a ScyllaDB iterator.
type iterator interface {
	Columns() []gocql.ColumnInfo
	Scan(dest ...interface{}) bool
	NumRows() int
}

// query defines methods for interacting with a ScyllaDB query.
type query interface {
	Exec() error
	Iter() iterator
	MapScanCAS(dest map[string]interface{}) (applied bool, err error)
	ScanCAS(dest ...interface{}) (applied bool, err error)
}

// batch defines methods for interacting with a ScyllaDB batch.
type batch interface {
	Query(stmt string, args ...interface{})
	getBatch() *gocql.Batch
}

// session defines methods for interacting with a ScyllaDB session.
type session interface {
	Query(stmt string, values ...interface{}) query
	newBatch(batchType gocql.BatchType) batch
	executeBatch(batch batch) error
	executeBatchCAS(batch batch, dest ...any) (bool, error)
}
