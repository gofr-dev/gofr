package scylladb

import (
	"github.com/gocql/gocql"
	_ "github.com/gocql/gocql"
)

type clusterConfig interface {
	createSession() (session, error)
}

type iterator interface {
	columns() []gocql.ColumnInfo
	scan(dest ...interface{}) bool
	numRows() int
}

type query interface {
	exec() error
	iter() iterator
	mapScanCAS(dest map[string]interface{}) (applied bool, err error)
	scanCAS(dest ...interface{}) (applied bool, err error)
}

type batch interface {
	Query(stmt string, args ...interface{})
	getBatch() *gocql.Batch
}

type session interface {
	query(stmt string, values ...interface{}) query
	newBatch(batchType gocql.BatchType) batch
	executeBatch(batch batch) error
	executeBatchCAS(batch batch, dest ...interface{}) (bool, error)
}
